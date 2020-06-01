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
	"path/filepath"
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
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// Plugin is the type for the plugin.
type ShellPlugin struct {
	name        string
	stdin       *os.File
	stdout      *os.File
	ipcFilePath string
	logFilePath string
	dataChannel datachannel.IDataChannel
}

type IShellPlugin interface {
	Execute(context context.T, config agentContracts.Configuration, cancelFlag task.CancelFlag, output iohandler.IOHandler, dataChannel datachannel.IDataChannel, shellProps mgsContracts.ShellProperties)
	InputStreamMessageHandler(log log.T, streamDataMessage mgsContracts.AgentMessage) error
}

// NewPlugin returns a new instance of the Shell Plugin
func NewPlugin(name string) (*ShellPlugin, error) {
	var plugin = ShellPlugin{name: name}
	return &plugin, nil
}

// validate validates the cloudwatch and s3 encryption configuration.
func (p *ShellPlugin) validate(context context.T,
	config agentContracts.Configuration,
	cwl cloudwatchlogsinterface.ICloudWatchLogsService,
	s3Util s3util.IAmazonS3Util) error {

	if config.CloudWatchLogGroup != "" && config.CloudWatchEncryptionEnabled {
		if encrypted, err := cwl.IsLogGroupEncryptedWithKMS(context.Log(), config.CloudWatchLogGroup); err != nil {
			return errors.New(fmt.Sprintf("Couldn't start the session because we are unable to validate encryption on CloudWatch Logs log group. Error: %v", err))
		} else if !encrypted {
			return errors.New(mgsConfig.CloudWatchEncryptionErrorMsg)
		}
	}

	if config.OutputS3BucketName != "" && config.S3EncryptionEnabled {
		if encrypted, err := s3Util.IsBucketEncrypted(context.Log(), config.OutputS3BucketName); err != nil {
			return errors.New(fmt.Sprintf("Couldn't start the session because we are unable to validate encryption on Amazon S3 bucket. Error: %v", err))
		} else if !encrypted {
			return errors.New(mgsConfig.S3EncryptionErrorMsg)
		}
	}
	return nil
}

// Execute starts pseudo terminal.
// It reads incoming message from data channel and writes to pty.stdin.
// It reads message from pty.stdout and writes to data channel
func (p *ShellPlugin) Execute(context context.T,
	config agentContracts.Configuration,
	cancelFlag task.CancelFlag,
	output iohandler.IOHandler,
	dataChannel datachannel.IDataChannel,
	shellProps mgsContracts.ShellProperties) {

	log := context.Log()
	p.dataChannel = dataChannel
	defer func() {
		if err := Stop(log); err != nil {
			log.Errorf("Error occurred while closing pty: %v", err)
		}
		if err := recover(); err != nil {
			log.Errorf("Error occurred while executing plugin %s: \n%v", p.name, err)
			log.Flush()
			os.Exit(1)
		}
	}()

	if cancelFlag.ShutDown() {
		output.MarkAsShutdown()
	} else if cancelFlag.Canceled() {
		output.MarkAsCancelled()
	} else {
		p.execute(context, config, cancelFlag, output, shellProps)
	}
}

var startPty = func(log log.T, shellProps mgsContracts.ShellProperties, isSessionLogger bool, config agentContracts.Configuration) (stdin *os.File, stdout *os.File, err error) {
	return StartPty(log, shellProps, isSessionLogger, config)
}

// execute starts pseudo terminal.
// It reads incoming message from data channel and writes to pty.stdin.
// It reads message from pty.stdout and writes to data channel
func (p *ShellPlugin) execute(context context.T,
	config agentContracts.Configuration,
	cancelFlag task.CancelFlag,
	output iohandler.IOHandler,
	shellProps mgsContracts.ShellProperties) {

	log := context.Log()
	var err error
	sessionPluginResultOutput := mgsContracts.SessionPluginResultOutput{}

	var cwl cloudwatchlogsinterface.ICloudWatchLogsService
	var s3Util s3util.IAmazonS3Util
	if config.OutputS3BucketName != "" {
		s3Util = s3util.NewAmazonS3Util(log, config.OutputS3BucketName)
	}
	if config.CloudWatchLogGroup != "" {
		cwl = cloudwatchlogspublisher.NewCloudWatchLogsService(log)
	}
	if err = p.validate(context, config, cwl, s3Util); err != nil {
		output.SetExitCode(appconfig.ErrorExitCode)
		output.SetStatus(agentContracts.ResultStatusFailed)
		sessionPluginResultOutput.Output = err.Error()
		output.SetOutput(sessionPluginResultOutput)
		log.Errorf("Encryption validation failed, err: %s", err)
		return
	}

	p.stdin, p.stdout, err = startPty(log, shellProps, false, config)
	if err != nil {
		errorString := fmt.Errorf("Unable to start shell: %s", err)
		log.Error(errorString)
		output.MarkAsFailed(errorString)
		return
	}

	// Generate ipc file path
	p.ipcFilePath = filepath.Join(config.OrchestrationDirectory, mgsConfig.IpcFileName+mgsConfig.LogFileExtension)

	// Generate final log file path
	logFileName := config.SessionId + mgsConfig.LogFileExtension
	p.logFilePath = filepath.Join(config.OrchestrationDirectory, logFileName)

	cancelled := make(chan bool, 1)
	go func() {
		cancelState := cancelFlag.Wait()
		if cancelFlag.Canceled() {
			cancelled <- true
			log.Debug("Cancel flag set to cancelled in session")
		}
		log.Debugf("Cancel flag set to %v in session", cancelState)
	}()

	log.Debugf("Start separate go routine to read from pty stdout and write to data channel")
	done := make(chan int, 1)
	go func() {
		done <- p.writePump(log)
	}()

	log.Infof("Plugin %s started", p.name)

	select {
	case <-cancelled:
		log.Debug("Session cancelled. Attempting to stop pty.")
		if err := Stop(log); err != nil {
			log.Errorf("Error occurred while closing pty: %v", err)
		}
		errorCode := 0
		output.SetExitCode(errorCode)
		output.SetStatus(agentContracts.ResultStatusSuccess)
		log.Info("The session was cancelled")

	case exitCode := <-done:
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

	// Generate log data only if customer has enabled logging.
	// TODO: Move below logic of uploading logs to S3 and cloudwatch to IOHandler
	if config.OutputS3BucketName != "" || config.CloudWatchLogGroup != "" {
		log.Debugf("Creating log file for shell session id %s at %s", config.SessionId, p.logFilePath)
		if err = p.generateLogData(log, config); err != nil {
			errorString := fmt.Errorf("unable to generate log data: %s", err)
			log.Error(errorString)
			output.MarkAsFailed(errorString)
			return
		}

		log.Debug("Starting S3 logging")
		if config.OutputS3BucketName != "" {
			s3KeyPrefix := fileutil.BuildS3Path(config.OutputS3KeyPrefix, logFileName)
			p.uploadShellSessionLogsToS3(log, s3Util, config, s3KeyPrefix)
			sessionPluginResultOutput.S3Bucket = config.OutputS3BucketName
			sessionPluginResultOutput.S3UrlSuffix = s3KeyPrefix
		}

		log.Debug("Starting CloudWatch logging")
		if config.CloudWatchLogGroup != "" {
			cwl.StreamData(log, config.CloudWatchLogGroup, config.SessionId, p.logFilePath, true, false)
			sessionPluginResultOutput.CwlGroup = config.CloudWatchLogGroup
			sessionPluginResultOutput.CwlStream = config.SessionId
		}
	}
	output.SetOutput(sessionPluginResultOutput)

	log.Debug("Shell session execution complete")
}

// uploadShellSessionLogsToS3 uploads shell session logs to S3 bucket specified.
func (p *ShellPlugin) uploadShellSessionLogsToS3(log log.T, s3UploaderUtil s3util.IAmazonS3Util, config agentContracts.Configuration, s3KeyPrefix string) {
	log.Debugf("Preparing to upload session logs to S3 bucket %s and prefix %s", config.OutputS3BucketName, s3KeyPrefix)

	if err := s3UploaderUtil.S3Upload(log, config.OutputS3BucketName, s3KeyPrefix, p.logFilePath); err != nil {
		log.Errorf("Failed to upload shell session logs to S3: %s", err)
	}
}

// writePump reads from pty stdout and writes to data channel.
func (p *ShellPlugin) writePump(log log.T) (errorCode int) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("WritePump thread crashed with message: \n", err)
		}
	}()

	stdoutBytes := make([]byte, mgsConfig.StreamDataPayloadSize)
	reader := bufio.NewReader(p.stdout)

	// Create ipc file
	file, err := os.Create(p.ipcFilePath)
	if err != nil {
		log.Errorf("Encountered an error while creating file: %s", err)
		return appconfig.ErrorExitCode
	}
	defer file.Close()

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
		if unprocessedBuf, err = p.processStdoutData(log, stdoutBytes, stdoutBytesLen, unprocessedBuf, file); err != nil {
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
