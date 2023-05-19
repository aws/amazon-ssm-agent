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

// Package sessionplugin implements functionality common to all session manager plugins
package sessionplugin

import (
	"fmt"
	"math/rand"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	mgsConfig "github.com/aws/amazon-ssm-agent/agent/session/config"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/aws/amazon-ssm-agent/agent/session/datachannel"
	"github.com/aws/amazon-ssm-agent/agent/session/retry"
	"github.com/aws/amazon-ssm-agent/agent/session/shell/constants"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

type NewPluginFunc func(context.T) (ISessionPlugin, error)

// ISessionPlugin interface represents functions that need to be implemented by all session manager plugins
type ISessionPlugin interface {
	GetPluginParameters(parameters interface{}) interface{}
	RequireHandshake() bool
	Execute(config contracts.Configuration, cancelFlag task.CancelFlag, output iohandler.IOHandler, dataChannel datachannel.IDataChannel)
	InputStreamMessageHandler(log log.T, streamDataMessage mgsContracts.AgentMessage) error
}

// SessionPlugin is the wrapper for all session manager plugins and implements all functions of Runpluginutil.T interface
type SessionPlugin struct {
	context       context.T
	sessionPlugin ISessionPlugin
}

// NewPlugin returns a new instance of SessionPlugin which wraps a plugin that implements ISessionPlugin
func NewPlugin(context context.T, newPluginFunc NewPluginFunc) (*SessionPlugin, error) {
	sessionPlugin, err := newPluginFunc(context)
	return &SessionPlugin{context, sessionPlugin}, err
}

// Execute sets up datachannel and starts execution of session manager plugin like shell
func (p *SessionPlugin) Execute(
	config contracts.Configuration,
	cancelFlag task.CancelFlag,
	output iohandler.IOHandler) {

	log := p.context.Log()
	kmsKeyId := config.KmsKeyId

	dataChannel, err := getDataChannelForSessionPlugin(p.context, config.SessionId, config.ClientId, cancelFlag, p.sessionPlugin.InputStreamMessageHandler)
	if err != nil {
		errorString := fmt.Errorf("Setting up data channel with id %s failed: %s", config.SessionId, err)
		output.MarkAsFailed(errorString)
		log.Error(errorString)
		return
	}

	defer func() {
		dataChannel.PrepareToCloseChannel(log)
		dataChannel.Close(log)
	}()

	if err = dataChannel.SendAgentSessionStateMessage(p.context.Log(), mgsContracts.Connected); err != nil {
		log.Errorf("Unable to send AgentSessionState message with session status %s. %s", mgsContracts.Connected, err)
	}

	encryptionEnabled := p.isEncryptionEnabled(kmsKeyId, config.PluginName)
	sessionTypeRequest := mgsContracts.SessionTypeRequest{
		SessionType: config.PluginName,
		Properties:  p.sessionPlugin.GetPluginParameters(config.Properties),
	}
	if p.sessionPlugin.RequireHandshake() || encryptionEnabled {
		if appconfig.PluginNameNonInteractiveCommands == config.PluginName {
			var shellProps mgsContracts.ShellProperties
			if err := jsonutil.Remarshal(config.Properties, &shellProps); err != nil {
				errorString := fmt.Errorf("Fail to remarshal shell properties: %v", err)
				output.MarkAsFailed(errorString)
				log.Error(errorString)
				return
			}
			separateOutPutStream, err := constants.GetSeparateOutputStream(shellProps)
			if err != nil {
				errorString := fmt.Errorf("Fail to get separateOutPutStream property: %v", err)
				output.MarkAsFailed(errorString)
				log.Error(errorString)
				return
			}
			log.Debugf("Shell properties: %v, %b", shellProps, separateOutPutStream)

			dataChannel.SetSeparateOutputPayload(separateOutPutStream)
		}
		if err = dataChannel.PerformHandshake(log, kmsKeyId, encryptionEnabled, sessionTypeRequest); err != nil {
			errorString := fmt.Errorf("Encountered error while initiating handshake. %s", err)
			output.MarkAsFailed(errorString)
			log.Error(errorString)
			return
		}
	} else {
		dataChannel.SkipHandshake(log)
	}

	p.sessionPlugin.Execute(config, cancelFlag, output, dataChannel)
}

// isEncryptionEnabled checks kmsKeyId and pluginName to determine if encryption is enabled for this session
// TODO: make encryption configurable for port plugin
func (p *SessionPlugin) isEncryptionEnabled(kmsKeyId string, pluginName string) bool {
	return kmsKeyId != "" && pluginName != appconfig.PluginNamePort
}

// getDataChannelForSessionPlugin opens new data channel to MGS service
var getDataChannelForSessionPlugin = func(context context.T, sessionId string, clientId string, cancelFlag task.CancelFlag, inputStreamMessageHandler datachannel.InputStreamMessageHandler) (datachannel.IDataChannel, error) {
	retryer := retry.ExponentialRetryer{
		CallableFunc: func() (channel interface{}, err error) {
			return datachannel.NewDataChannel(
				context,
				sessionId,
				clientId,
				inputStreamMessageHandler,
				cancelFlag)
		},
		GeometricRatio:      mgsConfig.RetryGeometricRatio,
		InitialDelayInMilli: rand.Intn(mgsConfig.DataChannelRetryInitialDelayMillis) + mgsConfig.DataChannelRetryInitialDelayMillis,
		MaxDelayInMilli:     mgsConfig.DataChannelRetryMaxIntervalMillis,
		MaxAttempts:         mgsConfig.DataChannelNumMaxAttempts,
	}
	channel, err := retryer.Call()
	if err != nil {
		return nil, err
	}
	dataChannel := channel.(*datachannel.DataChannel)
	return dataChannel, nil
}
