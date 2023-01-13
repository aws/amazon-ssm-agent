// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package shell is a common library that implements session manager shell.
package shell

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/aws/amazon-ssm-agent/agent/agentlogstocloudwatch/cloudwatchlogspublisher"
	"github.com/aws/amazon-ssm-agent/agent/agentlogstocloudwatch/cloudwatchlogspublisher/cloudwatchlogsinterface"
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	agentContracts "github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/s3util"
	mgsConfig "github.com/aws/amazon-ssm-agent/agent/session/config"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/aws/amazon-ssm-agent/agent/session/datachannel"
	"github.com/aws/amazon-ssm-agent/agent/session/shell/constants"
	"github.com/aws/amazon-ssm-agent/agent/session/shell/execcmd"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
)

// Plugin is the type for the plugin.
type ShellPlugin struct {
	context        context.T
	name           string
	stdin          *os.File
	stdout         *os.File
	stdoutPipe     io.Reader
	stderrPipe     io.Reader
	execCmd        execcmd.IExecCmd
	runAsUser      string
	dataChannel    datachannel.IDataChannel
	logger         logger
	separateOutput bool
	stdoutPrefix   string
	stderrPrefix   string
}

// logger is used for storing the information related to logging of session data to S3/CW
type logger struct {
	ipcFilePath                 string
	logFilePath                 string
	logFileName                 string
	transcriptDirPath           string
	ptyTerminated               chan bool
	cloudWatchStreamingFinished chan bool
	streamLogsToCloudWatch      bool
	s3Util                      s3util.IAmazonS3Util
	cwl                         cloudwatchlogsinterface.ICloudWatchLogsService
}

type IShellPlugin interface {
	Execute(config agentContracts.Configuration, cancelFlag task.CancelFlag, output iohandler.IOHandler, dataChannel datachannel.IDataChannel, shellProps mgsContracts.ShellProperties)
	InputStreamMessageHandler(log log.T, streamDataMessage mgsContracts.AgentMessage) error
}

const separateOutputStreamPrefixRegex = "^[0-9a-zA-Z\r\n_:-]{0,30}$"

// NewPlugin returns a new instance of the Shell Plugin
func NewPlugin(context context.T, name string) (*ShellPlugin, error) {
	var plugin = ShellPlugin{
		context:   context,
		name:      name,
		runAsUser: appconfig.DefaultRunAsUserName,
		logger: logger{
			ptyTerminated:               make(chan bool),
			cloudWatchStreamingFinished: make(chan bool),
		},
	}
	return &plugin, nil
}

// validate validates the cloudwatch and s3 configurations.
func (p *ShellPlugin) validate(config agentContracts.Configuration) error {
	log := p.context.Log()
	if config.CloudWatchLogGroup != "" {
		var logGroup *cloudwatchlogs.LogGroup
		var logGroupExists bool
		if logGroupExists, logGroup = p.logger.cwl.IsLogGroupPresent(config.CloudWatchLogGroup); !logGroupExists {
			log.Warnf("The CloudWatch log group specified in session preferences either does not exist or unable to validate its existence. " +
				"This might result in no logging of session data to CloudWatch.")
			p.logger.streamLogsToCloudWatch = false
		}

		if config.CloudWatchEncryptionEnabled {
			if encrypted, err := p.logger.cwl.IsLogGroupEncryptedWithKMS(logGroup); err != nil {
				return fmt.Errorf("Couldn't start the session because we are unable to validate encryption on CloudWatch Logs log group. Error: %v", err)
			} else if !encrypted {
				return errors.New(mgsConfig.CloudWatchEncryptionErrorMsg)
			}
		}

		if p.logger.streamLogsToCloudWatch {
			if logStreamingSupported, err := p.isLogStreamingSupported(p.context.Log()); !logStreamingSupported {
				log.Warn(err.Error())
				p.logger.streamLogsToCloudWatch = false
			}
		}
	}

	if config.OutputS3BucketName != "" && config.S3EncryptionEnabled {
		if encrypted, err := p.logger.s3Util.IsBucketEncrypted(p.context.Log(), config.OutputS3BucketName); err != nil {
			return fmt.Errorf("Couldn't start the session because we are unable to validate encryption on Amazon S3 bucket. Error: %v", err)
		} else if !encrypted {
			return errors.New(mgsConfig.S3EncryptionErrorMsg)
		}
	}
	return nil
}

// validPrefix checks whether the given prefix string is valid.
func (p *ShellPlugin) validPrefix(prefix string) bool {
	prefixRegex := regexp.MustCompile(separateOutputStreamPrefixRegex)
	return prefixRegex.MatchString(prefix)
}

// setSeparateOutputStreamProperties validates separateOutputStream properties and set them to shell plugin context.
func (p *ShellPlugin) setSeparateOutputStreamProperties(shellProps mgsContracts.ShellProperties) error {
	separateOutput, err := constants.GetSeparateOutputStream(shellProps)
	if err != nil {
		return fmt.Errorf("fail to get separateOutPutStream property: %v", err)
	}
	p.separateOutput = separateOutput

	if p.separateOutput {
		stdoutPrefix := constants.GetStdOutSeparatorPrefix(shellProps)
		if !p.validPrefix(stdoutPrefix) {
			return fmt.Errorf("invalid stdoutSeparatorPrefix %v", stdoutPrefix)
		}
		p.stdoutPrefix = stdoutPrefix

		stderrPrefix := constants.GetStdErrSeparatorPrefix(shellProps)
		if !p.validPrefix(stderrPrefix) {
			return fmt.Errorf("invalid stderrSeparatorPrefix %v", stderrPrefix)
		}
		p.stderrPrefix = stderrPrefix
	}

	return nil
}

// Execute starts pseudo terminal.
// It reads incoming message from data channel and writes to pty.stdin.
// It reads message from pty.stdout and writes to data channel
func (p *ShellPlugin) Execute(
	config agentContracts.Configuration,
	cancelFlag task.CancelFlag,
	output iohandler.IOHandler,
	dataChannel datachannel.IDataChannel,
	shellProps mgsContracts.ShellProperties) {

	log := p.context.Log()
	p.dataChannel = dataChannel
	defer func() {
		if err := p.stop(log); err != nil {
			log.Errorf("Error occurred while closing pty: %v", err)
		}
		if err := recover(); err != nil {
			log.Errorf("Error occurred while executing plugin %s: \n%v", p.name, err)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
			log.Flush()
			os.Exit(1)
		}
	}()

	if cancelFlag.ShutDown() {
		output.MarkAsShutdown()
	} else if cancelFlag.Canceled() {
		output.MarkAsCancelled()
	} else {
		p.execute(config, cancelFlag, output, shellProps)
	}
}

var getCommandExecutor = func(
	log log.T,
	shellProps mgsContracts.ShellProperties,
	isSessionLogger bool,
	config agentContracts.Configuration,
	plugin *ShellPlugin) (err error) {

	return StartCommandExecutor(log, shellProps, isSessionLogger, config, plugin)
}

// execute starts command execution.
// It reads incoming message from data channel and executes it by either writing to pty.stdin or relying on exec.Cmd.
// It reads message from pty.stdout and writes to data channel.
func (p *ShellPlugin) execute(config agentContracts.Configuration,
	cancelFlag task.CancelFlag,
	output iohandler.IOHandler,
	shellProps mgsContracts.ShellProperties) {

	// Initialization
	log := p.context.Log()
	sessionPluginResultOutput := mgsContracts.SessionPluginResultOutput{}
	p.initializeLogger(log, config)

	// Validate session configuration before starting
	if err := p.validate(config); err != nil {
		output.SetExitCode(appconfig.ErrorExitCode)
		output.SetStatus(agentContracts.ResultStatusFailed)
		sessionPluginResultOutput.Output = err.Error()
		output.SetOutput(sessionPluginResultOutput)
		log.Errorf("Validation failed, err: %s", err)

		return
	}

	if appconfig.PluginNameNonInteractiveCommands == p.name {
		if err := p.setSeparateOutputStreamProperties(shellProps); err != nil {
			output.SetExitCode(appconfig.ErrorExitCode)
			output.SetStatus(agentContracts.ResultStatusFailed)
			sessionPluginResultOutput.Output = err.Error()
			output.SetOutput(sessionPluginResultOutput)
			log.Errorf("SeparateOutputStream properties validation failed, err: %s", err)
			return
		}
	}
	// Catch signals and send a signal to the "sigs" chan if it triggers
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGQUIT)

	// Setup cancellation flag for accepting TerminateSession requests and idle session timeout scenarios
	cancelled := make(chan bool, 1)
	go func() {
		sig := <-sigs
		log.Infof("caught signal to terminate: %v", sig)
		cancelled <- true
	}()

	// Get the command executor, which is either pseudo terminal or exec.Cmd depending on the plugin type
	if err := getCommandExecutor(log, shellProps, false, config, p); err != nil {
		errorString := fmt.Errorf("Unable to start command: %s\n", err)
		log.Error(errorString)
		time.Sleep(2 * time.Second)
		if appconfig.PluginNameNonInteractiveCommands == p.name {
			// Error started before exec.cmd starts needs to be explicitly propagated to data channel.
			p.sendErrorToDataChannel(log, errorString.Error())
		}
		output.MarkAsFailed(errorString)
		return
	}

	// Create ipcFile used for logging session data temporarily on disk
	ipcFile, err := os.Create(p.logger.ipcFilePath)
	if err != nil {
		errorString := fmt.Errorf("encountered an error while creating file %s: %s", p.logger.ipcFilePath, err)
		log.Error(errorString)
		output.MarkAsFailed(errorString)
		return
	}
	defer func() {
		if closeErr := ipcFile.Close(); closeErr != nil {
			log.Warnf("error occurred while closing ipcFile, %v", closeErr)
		}
	}()

	go func() {
		cancelState := cancelFlag.Wait()
		if cancelFlag.Canceled() {
			cancelled <- true
			log.Debug("Cancel flag set to cancelled in session")
		}
		log.Debugf("Cancel flag set to %v in session", cancelState)
	}()

	if appconfig.PluginNameNonInteractiveCommands == p.name {
		p.executeCommandsWithExec(config, cancelled, cancelFlag, output, ipcFile)
	} else {
		p.executeCommandsWithPty(config, cancelled, cancelFlag, output, ipcFile)
	}

	// Finish logger activity like uploading logs to S3/CW
	p.finishLogging(config, output, sessionPluginResultOutput, ipcFile)

	log.Debug("Shell session execution complete")
}

// Executes command in pseudo terminal with pty
func (p *ShellPlugin) executeCommandsWithPty(config agentContracts.Configuration,
	cancelled chan bool,
	cancelFlag task.CancelFlag,
	output iohandler.IOHandler,
	ipcFile *os.File) {

	log := p.context.Log()

	writePumpDone := p.setupRoutineToWriteCommandOutput(log, ipcFile, 1)

	log.Infof("Plugin %s started", p.name)

	// Execute shell profile
	if appconfig.PluginNameStandardStream == p.name {
		if err := p.runShellProfile(log, config); err != nil {
			errorString := fmt.Errorf("Encountered an error while executing shell profile: %s", err)
			log.Error(errorString)
			output.MarkAsFailed(errorString)
			return
		}
	}

	// Start logging activity like streaming to CW
	p.startStreamingLogs(ipcFile, config)

	// Wait for session to be completed/cancelled/interrupted
	select {
	case <-cancelled:
		log.Debug("Session cancelled. Attempting to stop pty.")

		defer func() {
			if p.execCmd != nil {
				if err := p.execCmd.Wait(); err != nil {
					log.Errorf("unable to wait pty: %s", err)
				}
			}
		}()
		if p.execCmd != nil {
			if err := p.execCmd.Kill(); err != nil {
				log.Errorf("unable to terminate pty: %s", err)
			}
		}

		if err := p.stop(log); err != nil {
			log.Errorf("Error occurred while closing pty: %v", err)
		}
		errorCode := 0
		output.SetExitCode(errorCode)
		output.SetStatus(agentContracts.ResultStatusSuccess)
		log.Info("The session was cancelled")

	case exitCode := <-writePumpDone:
		defer func() {
			if p.execCmd != nil {
				if err := p.execCmd.Wait(); err != nil {
					log.Errorf("pty process: %v exited unsuccessfully, error message: %v", p.execCmd.Pid(), err)
				} else {
					log.Debugf("pty process: %v exited successfully", p.execCmd.Pid())
				}
			}
		}()
		if exitCode == 1 {
			output.SetExitCode(appconfig.ErrorExitCode)
			output.SetStatus(agentContracts.ResultStatusFailed)
		} else {
			// Call datachannel PrepareToCloseChannel so all messages in the buffer are sent
			p.dataChannel.PrepareToCloseChannel(log)

			// Send session status as Terminating to service on receiving success exit code from pty
			if err := p.dataChannel.SendAgentSessionStateMessage(log, mgsContracts.Terminating); err != nil {
				log.Errorf("Unable to send AgentSessionState message with session status %s. %v", mgsContracts.Terminating, err)
			}
			output.SetExitCode(appconfig.SuccessExitCode)
			output.SetStatus(agentContracts.ResultStatusSuccess)
		}
		if cancelFlag.Canceled() {
			log.Errorf("The cancellation failed to stop the session.")
		}
	}
}

// Execute single command in non-interactive mode with exec.Cmd
func (p *ShellPlugin) executeCommandsWithExec(config agentContracts.Configuration,
	cancelled chan bool,
	cancelFlag task.CancelFlag,
	output iohandler.IOHandler,
	ipcFile *os.File) {

	log := p.context.Log()

	log.Infof("Plugin %s started", p.name)

	// CW streaming logs is disabled for NonInteractiveCommands plugin, which is by far the only session plugin that uses exec.Cmd.
	// However, leaving the startStreamingLogs call path here in case future session plugins use exec.Cmd differently and need streaming logs.
	p.startStreamingLogs(ipcFile, config)

	if p.separateOutput {
		if err := p.processCommandsWithOutputStreamSeparate(cancelled, cancelFlag, output, ipcFile); err != nil {
			p.sendErrorToDataChannel(log, err.Error())
		}
	} else {
		if err := p.processCommandsWithExec(cancelled, cancelFlag, output, ipcFile); err != nil {
			p.sendErrorToDataChannel(log, err.Error())
		}
		p.cleanupOutputFile(log, config)
	}
}

// Handle go routines between session termination and command execution with exec.Cmd
func (p *ShellPlugin) processCommandsWithOutputStreamSeparate(cancelled chan bool,
	cancelFlag task.CancelFlag,
	output iohandler.IOHandler,
	ipcFile *os.File) (err error) {

	log := p.context.Log()

	writeStdOutDone := p.setupRoutineToWriteCmdPipelineOutput(log, ipcFile, false)
	writeStdErrDone := p.setupRoutineToWriteCmdPipelineOutput(log, ipcFile, true)

	if err := p.execCmd.Start(); err != nil {
		errorString := fmt.Errorf("Error occurred starting the command: %s\n", err)
		log.Error(errorString)
		output.MarkAsFailed(errorString)
		return err
	}

	// Wait for session to be completed/cancelled/interrupted
	cmdWaitDone := make(chan error, 1)
	cmdExitCode := make(chan int, 1)
	writeStdOutResult, writeStdErrResult := appconfig.ErrorExitCode, appconfig.ErrorExitCode

	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Errorf("Write Stdout thread crashed with message: %v\n", err)
				log.Errorf("Stacktrace:\n%s", debug.Stack())
			}
		}()
		log.Debugf("Start separate go routine to wait for command to complete. Pid: %v", p.execCmd.Pid())
		writeStdOutResult, writeStdErrResult = <-writeStdOutDone, <-writeStdErrDone
		log.Debugf("writeStdOutResult: %v, writeStdErrResult: %v", writeStdOutResult, writeStdErrResult)
		close(writeStdOutDone)
		close(writeStdErrDone)

		err := p.execCmd.Wait()
		if err != nil {
			if exiterr, ok := err.(*exec.ExitError); ok {
				log.Infof("Command Exit Status: %d", exiterr.ExitCode())
				cmdExitCode <- exiterr.ExitCode()

			} else {
				log.Errorf("Failed to get exit code, set it to %v", appconfig.ErrorExitCode)
				cmdExitCode <- appconfig.ErrorExitCode
			}
		} else {
			log.Infof("Command success with exit status 0")
			cmdExitCode <- appconfig.SuccessExitCode
		}
		cmdWaitDone <- err
	}()

	select {
	case <-cancelled:
		log.Debug("Session cancelled. Attempting to stop the command execution.")
		if err := p.execCmd.Kill(); err != nil {
			log.Errorf("unable to terminate command execution process %s: %v", p.execCmd.Pid(), err)
		}
		output.SetExitCode(appconfig.SuccessExitCode)
		output.SetStatus(agentContracts.ResultStatusSuccess)
		log.Info("The session was cancelled")

	case cmdWaitErr := <-cmdWaitDone:
		if cmdWaitErr != nil {
			log.Errorf("received error when waiting for command to complete: %v", cmdWaitErr)
		}
		if cancelFlag.Canceled() {
			log.Errorf("the cancellation failed to stop the session.")
		}

		if writeStdOutResult == appconfig.SuccessExitCode && writeStdErrResult == appconfig.SuccessExitCode {
			log.Debugf("Writing session plugin output is done. Exit code: 0.")
			output.SetExitCode(appconfig.SuccessExitCode)
			output.SetStatus(agentContracts.ResultStatusSuccess)
		} else {
			log.Debugf("Writing session plugin output is done. Exit code: 1.")
			output.SetExitCode(appconfig.ErrorExitCode)
			output.SetStatus(agentContracts.ResultStatusFailed)
		}
		commandExitCode := <-cmdExitCode
		close(cmdExitCode)
		log.Infof("The session commandExitCode %d", commandExitCode)
		p.sendExitCode(log, ipcFile, commandExitCode)
	}

	// Call datachannel PrepareToCloseChannel so all messages in the buffer are sent
	p.dataChannel.PrepareToCloseChannel(log)

	// Send session status as Terminating to service on completing command execution
	if err := p.dataChannel.SendAgentSessionStateMessage(log, mgsContracts.Terminating); err != nil {
		log.Errorf("Unable to send AgentSessionState message with session status %s. %v", mgsContracts.Terminating, err)
	}

	return nil
}

// Handle go routines between session termination and command execution with exec.Cmd
func (p *ShellPlugin) processCommandsWithExec(cancelled chan bool,
	cancelFlag task.CancelFlag,
	output iohandler.IOHandler,
	ipcFile *os.File) (err error) {

	log := p.context.Log()

	if err := p.execCmd.Start(); err != nil {
		errorString := fmt.Errorf("Error occurred starting the command: %s\n", err)
		log.Error(errorString)
		output.MarkAsFailed(errorString)
		return err
	}

	// Wait for session to be completed/cancelled/interrupted
	cmdWaitDone := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("Process commands with exec panic: %v", r)
				log.Errorf("Stacktrace:\n%s", debug.Stack())
			}
		}()
		log.Debugf("Start separate go routine to wait for command to complete. Pid: %v", p.execCmd.Pid())
		err := p.execCmd.Wait()
		cmdWaitDone <- err
	}()

	select {
	case <-cancelled:
		log.Debug("Session cancelled. Attempting to stop the command execution.")
		if err := p.execCmd.Kill(); err != nil {
			log.Errorf("unable to terminate command execution process %s: %v", p.execCmd.Pid(), err)
		}
		errorCode := 0
		output.SetExitCode(errorCode)
		output.SetStatus(agentContracts.ResultStatusSuccess)
		log.Info("The session was cancelled")

	case cmdWaitErr := <-cmdWaitDone:
		if cmdWaitErr != nil {
			log.Errorf("received error when waiting for command to complete: %v", cmdWaitErr)
		}
		if cancelFlag.Canceled() {
			log.Errorf("the cancellation failed to stop the session.")
		}
	}

	writePumpDone := p.setupRoutineToWriteCommandOutput(log, ipcFile, 0)

	select {
	case <-cancelled:
		log.Debug("Session cancelled. Attempting to stop the command execution.")
		if err := p.execCmd.Kill(); err != nil {
			log.Errorf("unable to terminate command execution process %s: %v", p.execCmd.Pid(), err)
		}
		errorCode := 0
		output.SetExitCode(errorCode)
		output.SetStatus(agentContracts.ResultStatusSuccess)
		log.Info("The session was cancelled")

	case exitCode := <-writePumpDone:
		log.Debugf("Writing command output is done. Exit code: %v.", exitCode)
		if exitCode == 1 {
			output.SetExitCode(appconfig.ErrorExitCode)
			output.SetStatus(agentContracts.ResultStatusFailed)
		} else {
			output.SetExitCode(appconfig.SuccessExitCode)
			output.SetStatus(agentContracts.ResultStatusSuccess)
		}

		// Call datachannel PrepareToCloseChannel so all messages in the buffer are sent
		p.dataChannel.PrepareToCloseChannel(log)

		// Send session status as Terminating to service on completing command execution
		if err := p.dataChannel.SendAgentSessionStateMessage(log, mgsContracts.Terminating); err != nil {
			log.Errorf("Unable to send AgentSessionState message with session status %s. %v", mgsContracts.Terminating, err)
		}
	}

	return nil
}

// initializeLogger initializes plugin logger to be used for s3/cw logging
func (p *ShellPlugin) initializeLogger(log log.T, config agentContracts.Configuration) {
	if config.OutputS3BucketName != "" {
		var err error
		p.logger.s3Util, err = s3util.NewAmazonS3Util(p.context, config.OutputS3BucketName)
		if err != nil {
			log.Warnf("S3 client initialization failed, err: %v", err)
		}
	}
	if config.CloudWatchLogGroup != "" && p.logger.cwl == nil {
		p.logger.cwl = cloudwatchlogspublisher.NewCloudWatchLogsService(p.context)
	}

	// Set CW streaming as true if log group provided and streaming enabled
	if config.CloudWatchLogGroup != "" && config.CloudWatchStreamingEnabled {
		p.logger.streamLogsToCloudWatch = true
	}

	// Generate ipc file path
	p.logger.ipcFilePath = filepath.Join(config.OrchestrationDirectory, mgsConfig.IpcFileName+mgsConfig.LogFileExtension)

	// Generate final log file path
	p.logger.logFileName = config.SessionId + mgsConfig.LogFileExtension
	p.logger.logFilePath = filepath.Join(config.OrchestrationDirectory, p.logger.logFileName)
}

// uploadShellSessionLogsToS3 uploads shell session logs to S3 bucket specified.
func (p *ShellPlugin) uploadShellSessionLogsToS3(log log.T, s3UploaderUtil s3util.IAmazonS3Util, config agentContracts.Configuration, s3KeyPrefix string) {
	if s3UploaderUtil == nil {
		log.Warnf("Uploading logs to S3 cannot be completed due to failure in initializing s3util.")
		return
	}

	log.Debugf("Preparing to upload session logs to S3 bucket %s and prefix %s", config.OutputS3BucketName, s3KeyPrefix)

	if err := s3UploaderUtil.S3Upload(log, config.OutputS3BucketName, s3KeyPrefix, p.logger.logFilePath); err != nil {
		log.Errorf("Failed to upload shell session logs to S3: %s", err)
	}
}

// Set up go routine to write command output to data channel
func (p *ShellPlugin) setupRoutineToWriteCommandOutput(log log.T, ipcFile *os.File, initialWaitSecond int) chan int {
	log.Debugf("Start separate go routine to read from command output and write to data channel")

	done := make(chan int, 1)
	go func() {
		done <- p.writePump(log, ipcFile, initialWaitSecond)
	}()

	return done
}

// Set up go routine to write command output to data channel
func (p *ShellPlugin) setupRoutineToWriteCmdPipelineOutput(log log.T, ipcFile *os.File, isStderr bool) chan int {
	done := make(chan int, 1)

	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Errorf("Write pipeline output crashed with message: %v\n", err)
				log.Errorf("Stacktrace:\n%s", debug.Stack())
			}
		}()

		var pipe io.Reader
		var unprocessedBuf bytes.Buffer
		var outputPrefix string
		prefix := make([]byte, 128)
		payloadType := mgsContracts.Output

		if isStderr {
			pipe = p.stderrPipe
			outputPrefix = p.stderrPrefix
			payloadType = mgsContracts.StdErr
		} else {
			pipe = p.stdoutPipe
			outputPrefix = p.stdoutPrefix
		}

		prefixLen := len(outputPrefix)
		if prefixLen > 0 {
			r := strings.NewReader(outputPrefix)
			len, err := r.Read(prefix)
			if err != nil {
				log.Debugf("Failed to read prefix: %s", err)
			}
			prefixLen = len
		}

		prefix = prefix[:prefixLen]
		outputBytes := make([]byte, mgsConfig.StreamDataPayloadSize-prefixLen)
		needPrefix := true
		for {
			if p.dataChannel.IsActive() {
				outputBytesLen, err := pipe.Read(outputBytes)
				if err == io.EOF {
					log.Debugf("Pipeline closed, finish pipeline reading. Is StdErr pipe: %t", isStderr)
					done <- appconfig.SuccessExitCode
					break
				} else if err != nil {
					log.Errorf("Failed to read from command output pipeline: %s", err)
					done <- appconfig.ErrorExitCode
					break
				}
				// Add prefix for first none empty frame, later decide it based on content of last character
				if needPrefix && outputBytesLen > 0 {
					unprocessedBuf.Write(prefix)
				}
				if outputBytesLen > 0 && outputBytes[outputBytesLen-1] != '\n' {
					needPrefix = false
				} else {
					needPrefix = true
				}
				// unprocessedBuf contains incomplete utf8 encoded unicode bytes returned after processing of stdoutBytes
				if unprocessedBuf, err = p.processStdoutData(log, outputBytes, outputBytesLen, unprocessedBuf, ipcFile, payloadType); err != nil {
					log.Errorf("Error processing command pipeline output data, %v", err)
					done <- appconfig.ErrorExitCode
					break
				}
			} else {
				log.Errorf("Data Channel not in active status")
				done <- appconfig.ErrorExitCode
				break
			}
		}
	}()

	return done
}

// Write command exit code to data channel
func (p *ShellPlugin) sendExitCode(log log.T, ipcFile *os.File, exitCode int) error {
	var unprocessedBuf bytes.Buffer
	outputBytes := make([]byte, 128)

	output := "EXIT_CODE: " + strconv.Itoa(exitCode)
	r := strings.NewReader(output)
	outputBytesLen, err := r.Read(outputBytes)
	if err != nil {
		log.Debugf("Failed to read prefix for exit code: %s", err)
		return err
	}
	log.Infof("Sending exit code: %d", exitCode)

	if p.dataChannel.IsActive() {
		if unprocessedBuf, err = p.processStdoutData(log, outputBytes, outputBytesLen, unprocessedBuf, ipcFile, mgsContracts.ExitCode); err != nil {
			log.Errorf("Error processing command pipeline output data, %v", err)
		}
	} else {
		return fmt.Errorf("failed to send exit code as data channel closed")
	}

	return nil
}

// writePump reads from pty stdout and writes to data channel.
func (p *ShellPlugin) writePump(log log.T, ipcFile *os.File, initialWaitSecond int) (errorCode int) {
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("WritePump thread crashed with message: %v\n", err)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()

	stdoutBytes := make([]byte, mgsConfig.StreamDataPayloadSize)
	reader := bufio.NewReader(p.stdout)

	// Wait for all input commands to run.
	time.Sleep(time.Duration(initialWaitSecond) * time.Second)

	var unprocessedBuf bytes.Buffer
	for {
		if p.dataChannel.IsActive() {
			stdoutBytesLen, err := reader.Read(stdoutBytes)
			if err != nil {
				log.Debugf("Failed to read from command output: %s", err)
				return appconfig.SuccessExitCode
			}

			// unprocessedBuf contains incomplete utf8 encoded unicode bytes returned after processing of stdoutBytes
			if unprocessedBuf, err = p.processStdoutData(log, stdoutBytes, stdoutBytesLen, unprocessedBuf, ipcFile, mgsContracts.Output); err != nil {
				log.Errorf("Error processing stdout data, %v", err)
				return appconfig.ErrorExitCode
			}
		}

		// Wait for stdout to process more data
		time.Sleep(time.Millisecond)
	}
}

// processStdoutData reads utf8 encoded unicode characters from stdoutBytes and sends it over websocket channel.
func (p *ShellPlugin) processStdoutData(
	log log.T,
	stdoutBytes []byte,
	stdoutBytesLen int,
	unprocessedBuf bytes.Buffer,
	file *os.File,
	payloadType mgsContracts.PayloadType) (bytes.Buffer, error) {

	// append stdoutBytes to unprocessedBytes and then read rune from appended bytes to send it over websocket channel
	unprocessedBytes := unprocessedBuf.Bytes()
	unprocessedBytes = append(unprocessedBytes[:], stdoutBytes[:stdoutBytesLen]...)
	runeReader := bufio.NewReader(bytes.NewReader(unprocessedBytes))

	var processedBuf bytes.Buffer
	unprocessedBytesLen := len(unprocessedBytes)
	i := 0
	for i < unprocessedBytesLen {
		// read stdout bytes as utf8 encoded unicode character
		stdoutRune, stdoutRuneLen, err := runeReader.ReadRune()
		if err != nil {
			return processedBuf, fmt.Errorf("failed to read rune from reader: %s", err)
		}

		// Invalid utf8 encoded character results into RuneError.
		if stdoutRune == utf8.RuneError {

			// If invalid character is encountered within last 3 bytes of buffer (utf8 takes 1-4 bytes for a unicode character),
			// then break the loop and leave these bytes in unprocessed buffer for them to get processed later with more bytes returned by stdout.
			if unprocessedBytesLen-i < utf8.UTFMax {
				runeReader.UnreadRune()
				break
			}

			// If invalid character is encountered beyond last 3 bytes of buffer, then the character at ith position is invalid utf8 character.
			// Add invalid byte at ith position to processedBuf in such case and return to client to handle display of invalid character.
			processedBuf.Write(unprocessedBytes[i : i+1])
		} else {
			processedBuf.WriteRune(stdoutRune)
		}
		i += stdoutRuneLen
	}

	if err := p.dataChannel.SendStreamDataMessage(log, payloadType, processedBuf.Bytes()); err != nil {
		return processedBuf, fmt.Errorf("unable to send stream data message: %s", err)
	}

	if _, err := file.Write(processedBuf.Bytes()); err != nil {
		return processedBuf, fmt.Errorf("encountered an error while writing to file: %s", err)
	}

	// return incomplete utf8 encoded unicode bytes to be processed with next batch of stdoutBytes
	unprocessedBuf.Reset()
	if i < unprocessedBytesLen {
		unprocessedBuf.Write(unprocessedBytes[i:unprocessedBytesLen])
	}
	return unprocessedBuf, nil
}

// startStreamingLogs starts streaming of logs to CloudWatch
func (p *ShellPlugin) startStreamingLogs(
	ipcFile *os.File,
	config agentContracts.Configuration) (err error) {
	log := p.context.Log()

	// do nothing if streaming is disabled
	if !p.logger.streamLogsToCloudWatch {
		return
	}

	p.logger.cwl.SetCloudWatchMessage(
		"1.0",
		p.dataChannel.GetRegion(),
		p.dataChannel.GetInstanceId(),
		p.runAsUser,
		config.SessionId,
		config.SessionOwner)

	var streamingFilePath string
	if streamingFilePath, err = p.getStreamingFilePath(log); err != nil {
		return fmt.Errorf("error getting local transcript file path: %v", err)
	}

	// starts streaming
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("CloudWatch stream data panic: %v", r)
				log.Errorf("Stacktrace:\n%s", debug.Stack())
			}
		}()
		p.logger.cloudWatchStreamingFinished <- p.logger.cwl.StreamData(
			config.CloudWatchLogGroup,
			config.SessionId,
			streamingFilePath,
			false,
			false,
			p.logger.ptyTerminated,
			p.isCleanupOfControlCharactersRequired(),
			true)
	}()

	// check if log streaming is interrupted
	go func() {
		checkForLoggingInterruption(log, ipcFile, p)
	}()

	log.Debug("Streaming of logs to CloudWatch has started")
	return
}

// finishLogging generates and uploads logs to S3/CW, terminates streaming to CW if enabled
func (p *ShellPlugin) finishLogging(
	config agentContracts.Configuration,
	output iohandler.IOHandler,
	sessionPluginResultOutput mgsContracts.SessionPluginResultOutput,
	ipcFile *os.File) {
	log := p.context.Log()

	// Generate log data only if customer has either enabled S3 logging or CW logging with streaming disabled
	if config.OutputS3BucketName != "" || (config.CloudWatchLogGroup != "" && !config.CloudWatchStreamingEnabled) {
		log.Debugf("Creating log file for shell session id %s at %s", config.SessionId, p.logger.logFilePath)
		if err := p.generateLogData(log, config); err != nil {
			errorString := fmt.Errorf("unable to generate log data: %s", err)
			log.Error(errorString)
			output.MarkAsFailed(errorString)
			return
		}

		if config.OutputS3BucketName != "" {
			log.Debug("Starting S3 logging")
			s3KeyPrefix := fileutil.BuildS3Path(config.OutputS3KeyPrefix, p.logger.logFileName)
			p.uploadShellSessionLogsToS3(log, p.logger.s3Util, config, s3KeyPrefix)
			sessionPluginResultOutput.S3Bucket = config.OutputS3BucketName
			sessionPluginResultOutput.S3UrlSuffix = s3KeyPrefix
		}

		if config.CloudWatchLogGroup != "" && !config.CloudWatchStreamingEnabled {
			log.Debug("Starting CloudWatch logging")
			p.logger.cwl.StreamData(
				config.CloudWatchLogGroup,
				config.SessionId,
				p.logger.logFilePath,
				true,
				false,
				p.logger.ptyTerminated,
				false,
				false)
		}
	}

	// End streaming of logs since pty is terminated
	if p.logger.streamLogsToCloudWatch {
		p.logger.ptyTerminated <- true
		log.Debug("Waiting for streaming to finish")

		<-p.logger.cloudWatchStreamingFinished
		log.Debug("Streaming done, proceed to close session worker")

		p.cleanupLogFile(log, ipcFile)
	}

	// Populate CW log group information
	if config.CloudWatchLogGroup != "" {
		sessionPluginResultOutput.CwlGroup = config.CloudWatchLogGroup
		sessionPluginResultOutput.CwlStream = config.SessionId
	}
	output.SetOutput(sessionPluginResultOutput)
}

func (p *ShellPlugin) cleanupOutputFile(log log.T, config agentContracts.Configuration) {
	if err := os.Remove(filepath.Join(config.OrchestrationDirectory, mgsConfig.ExecOutputFileName)); err != nil {
		log.Debugf("Unable to clean up output file, %v", err)
	}
}

func (p *ShellPlugin) sendErrorToDataChannel(log log.T, errorString string) {
	time.Sleep(1 * time.Second)
	if dataChannelError := p.dataChannel.SendStreamDataMessage(log, mgsContracts.StdErr, []byte(errorString)); dataChannelError != nil {
		log.Errorf("Unable to send error message to data channel: %v", dataChannelError)
	}
}
