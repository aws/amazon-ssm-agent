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
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
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
	"github.com/aws/amazon-ssm-agent/agent/session/shell/execcmd"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
)

// Plugin is the type for the plugin.
type ShellPlugin struct {
	context     context.T
	name        string
	stdin       *os.File
	stdout      *os.File
	execCmd     execcmd.IExecCmd
	runAsUser   string
	dataChannel datachannel.IDataChannel
	logger      logger
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

var startPty = func(
	log log.T,
	shellProps mgsContracts.ShellProperties,
	isSessionLogger bool,
	config agentContracts.Configuration,
	plugin *ShellPlugin) (err error) {

	return StartPty(log, shellProps, isSessionLogger, config, plugin)
}

// execute starts pseudo terminal.
// It reads incoming message from data channel and writes to pty.stdin.
// It reads message from pty.stdout and writes to data channel
func (p *ShellPlugin) execute(
	config agentContracts.Configuration,
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

	// Start pseudo terminal
	if err := startPty(log, shellProps, false, config, p); err != nil {
		errorString := fmt.Errorf("Unable to start shell: %s", err)
		log.Error(errorString)
		time.Sleep(2 * time.Second)
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
	defer ipcFile.Close()

	go func() {
		cancelState := cancelFlag.Wait()
		if cancelFlag.Canceled() {
			cancelled <- true
			log.Debug("Cancel flag set to cancelled in session")
		}
		log.Debugf("Cancel flag set to %v in session", cancelState)
	}()

	// Start to read from shell and write to datachannel
	log.Debugf("Start separate go routine to read from pty stdout and write to data channel")
	done := make(chan int, 1)
	go func() {
		done <- p.writePump(log, ipcFile)
	}()
	log.Infof("Plugin %s started", p.name)

	// Execute shell profile
	if p.name == appconfig.PluginNameStandardStream {
		if err = p.runShellProfile(log, config); err != nil {
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

	case exitCode := <-done:
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
			// Send session status as Terminating to service on receiving success exit code from pty
			if err = p.dataChannel.SendAgentSessionStateMessage(log, mgsContracts.Terminating); err != nil {
				log.Errorf("Unable to send AgentSessionState message with session status %s. %v", mgsContracts.Terminating, err)
			}
			output.SetExitCode(appconfig.SuccessExitCode)
			output.SetStatus(agentContracts.ResultStatusSuccess)
		}
		if cancelFlag.Canceled() {
			log.Errorf("The cancellation failed to stop the session.")
		}
	}

	// Finish logger activity like uploading logs to S3/CW
	p.finishLogging(config, output, sessionPluginResultOutput, ipcFile)

	log.Debug("Shell session execution complete")
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

// writePump reads from pty stdout and writes to data channel.
func (p *ShellPlugin) writePump(log log.T, ipcFile *os.File) (errorCode int) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("WritePump thread crashed with message: \n", err)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()

	stdoutBytes := make([]byte, mgsConfig.StreamDataPayloadSize)
	reader := bufio.NewReader(p.stdout)

	// Wait for all input commands to run.
	time.Sleep(time.Second)

	var unprocessedBuf bytes.Buffer
	for {
		stdoutBytesLen, err := reader.Read(stdoutBytes)
		if err != nil {
			log.Debugf("Failed to read from pty master: %s", err)
			return appconfig.SuccessExitCode
		}

		// unprocessedBuf contains incomplete utf8 encoded unicode bytes returned after processing of stdoutBytes
		if unprocessedBuf, err = p.processStdoutData(log, stdoutBytes, stdoutBytesLen, unprocessedBuf, ipcFile); err != nil {
			log.Errorf("Error processing stdout data, %v", err)
			return appconfig.ErrorExitCode
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
	file *os.File) (bytes.Buffer, error) {

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

	if err := p.dataChannel.SendStreamDataMessage(log, mgsContracts.Output, processedBuf.Bytes()); err != nil {
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
