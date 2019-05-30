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

// Package shell implements session shell plugin.
package shell

import (
	"bufio"
	"bytes"
	"encoding/json"
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
	"github.com/aws/amazon-ssm-agent/agent/session/plugins/sessionplugin"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// Plugin is the type for the plugin.
type ShellPlugin struct {
	stdin       *os.File
	stdout      *os.File
	ipcFilePath string
	logFilePath string
	dataChannel datachannel.IDataChannel
}

// NewPlugin returns a new instance of the Shell Plugin
func NewPlugin() (sessionplugin.ISessionPlugin, error) {
	var plugin = ShellPlugin{}
	return &plugin, nil
}

// name returns the name of Shell Plugin
func (p *ShellPlugin) name() string {
	return appconfig.PluginNameStandardStream
}

// validate validates the cloudwatch and s3 encryption configuration.
func (p *ShellPlugin) validate(context context.T,
	config agentContracts.Configuration,
	cwl cloudwatchlogsinterface.ICloudWatchLogsService,
	s3Util s3util.IAmazonS3Util) error {

	if config.CloudWatchLogGroup != "" && config.CloudWatchEncryptionEnabled {
		if encrypted := cwl.IsLogGroupEncryptedWithKMS(context.Log(), config.CloudWatchLogGroup); !encrypted {
			return errors.New(mgsConfig.CloudWatchEncryptionErrorMsg)
		}
	}

	if config.OutputS3BucketName != "" && config.S3EncryptionEnabled {
		if encrypted := s3Util.IsBucketEncrypted(context.Log(), config.OutputS3BucketName); !encrypted {
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
	dataChannel datachannel.IDataChannel) {

	log := context.Log()
	p.dataChannel = dataChannel
	defer func() {
		if err := Stop(log); err != nil {
			log.Errorf("Error occured while closing pty: %v", err)
		}
		if err := recover(); err != nil {
			log.Errorf("Error occurred while executing plugin %s: \n%v", p.name(), err)
			log.Flush()
			os.Exit(1)
		}
	}()

	if cancelFlag.ShutDown() {
		output.MarkAsShutdown()
	} else if cancelFlag.Canceled() {
		output.MarkAsCancelled()
	} else {
		p.execute(context, config, cancelFlag, output)
	}
}

var startPty = func(log log.T, runAsSsmUser bool, shellCmd string) (stdin *os.File, stdout *os.File, err error) {
	return StartPty(log, runAsSsmUser, shellCmd)
}

// execute starts pseudo terminal.
// It reads incoming message from data channel and writes to pty.stdin.
// It reads message from pty.stdout and writes to data channel
func (p *ShellPlugin) execute(context context.T,
	config agentContracts.Configuration,
	cancelFlag task.CancelFlag,
	output iohandler.IOHandler) {

	log := context.Log()
	var err error
	sessionPluginResultOutput := mgsContracts.SessionPluginResultOutput{}

	var cwl cloudwatchlogsinterface.ICloudWatchLogsService
	var s3Util s3util.IAmazonS3Util
	if config.OutputS3BucketName != "" {
		s3Util = s3util.NewAmazonS3Util(log, config.OutputS3BucketName)
	}
	if config.CloudWatchLogGroup != "" {
		cwl = cloudwatchlogspublisher.NewCloudWatchLogsService()
	}
	if err = p.validate(context, config, cwl, s3Util); err != nil {
		output.SetExitCode(appconfig.ErrorExitCode)
		output.SetStatus(agentContracts.ResultStatusFailed)
		sessionPluginResultOutput.Output = err.Error()
		output.SetOutput(sessionPluginResultOutput)
		log.Errorf("Encryption validation failed, err: %s", err)
		return
	}

	p.stdin, p.stdout, err = startPty(log, !config.RunAsElevated, config.Commands)
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

	log.Infof("Plugin %s started", p.name())

	select {
	case <-cancelled:
		log.Debug("Session cancelled. Attempting to stop pty.")
		errorCode := 0
		output.SetExitCode(errorCode)
		output.SetStatus(agentContracts.ResultStatusSuccess)
		log.Info("The session was cancelled")

	case exitCode := <-done:
		if exitCode == 1 {
			output.SetExitCode(appconfig.ErrorExitCode)
			output.SetStatus(agentContracts.ResultStatusFailed)
		} else {
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
			Stop(log)
		}
	}()

	buf := make([]byte, mgsConfig.StreamDataPayloadSize)
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

	var buffer bytes.Buffer
	for {
		n, err := reader.Read(buf)
		if err != nil {
			// Terminating session
			log.Debugf("Failed to read from pty master: %s", err)
			if err = p.dataChannel.SendAgentSessionStateMessage(log, mgsContracts.Terminating); err != nil {
				log.Errorf("Unable to send AgentSessionState message with session status %s. %v", mgsContracts.Terminating, err)
			}
			return appconfig.SuccessExitCode
		}

		//read byte array as Unicode code points (rune in go)
		bufferBytes := buffer.Bytes()
		runeReader := bufio.NewReader(bytes.NewReader(append(bufferBytes[:], buf[:n]...)))
		buffer.Reset()
		i := 0
		for i < n {
			stdoutRune, stdoutRuneLen, err := runeReader.ReadRune()
			if err != nil {
				log.Errorf("Failed to read rune from reader: %s", err)
				return appconfig.ErrorExitCode
			}
			if stdoutRune == utf8.RuneError {
				runeReader.UnreadRune()
				break
			}
			i += stdoutRuneLen
			buffer.WriteRune(stdoutRune)
		}

		if err = p.dataChannel.SendStreamDataMessage(log, mgsContracts.Output, buffer.Bytes()); err != nil {
			log.Errorf("Unable to send stream data message: %s", err)
			return appconfig.ErrorExitCode
		}

		if _, err = file.Write(buffer.Bytes()); err != nil {
			log.Errorf("Encountered an error while writing to file: %s", err)
			return appconfig.ErrorExitCode
		}

		buffer.Reset()
		if i < n {
			buffer.Write(buf[i:n])
		}

		// Wait for stdout to process more data
		time.Sleep(time.Millisecond)
	}
}

// InputStreamMessageHandler passes payload byte stream to shell stdin
func (p *ShellPlugin) InputStreamMessageHandler(log log.T, streamDataMessage mgsContracts.AgentMessage) error {
	if p.stdin == nil || p.stdout == nil {
		// This is to handle scenario when cli/console starts sending size data but pty has not been started yet
		// Since packets are rejected, cli/console will resend these packets until pty starts successfully in separate thread
		log.Tracef("Pty unavailable. Reject incoming message packet")
		return nil
	}

	switch mgsContracts.PayloadType(streamDataMessage.PayloadType) {
	case mgsContracts.Output:
		log.Tracef("Output message received: %d", streamDataMessage.SequenceNumber)
		if _, err := p.stdin.Write(streamDataMessage.Payload); err != nil {
			log.Errorf("Unable to write to stdin, err: %v.", err)
			return err
		}
	case mgsContracts.Size:
		var size mgsContracts.SizeData
		if err := json.Unmarshal(streamDataMessage.Payload, &size); err != nil {
			log.Errorf("Invalid size message: %s", err)
			return err
		}
		log.Tracef("Resize data received: cols: %d, rows: %d", size.Cols, size.Rows)
		if err := SetSize(log, size.Cols, size.Rows); err != nil {
			log.Errorf("Unable to set pty size: %s", err)
			return err
		}
	}
	return nil
}
