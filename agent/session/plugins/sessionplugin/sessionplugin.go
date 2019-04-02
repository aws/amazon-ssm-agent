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
	"github.com/aws/amazon-ssm-agent/agent/log"
	mgsConfig "github.com/aws/amazon-ssm-agent/agent/session/config"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/aws/amazon-ssm-agent/agent/session/datachannel"
	"github.com/aws/amazon-ssm-agent/agent/session/retry"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

type NewPluginFunc func() (ISessionPlugin, error)

// ISessionPlugin interface represents functions that need to be implemented by all session manager plugins
type ISessionPlugin interface {
	GetPluginParameters(parameters interface{}) interface{}
	RequireHandshake() bool
	Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, output iohandler.IOHandler, dataChannel datachannel.IDataChannel)
	InputStreamMessageHandler(log log.T, streamDataMessage mgsContracts.AgentMessage) error
}

// SessionPlugin is the wrapper for all session manager plugins and implements all functions of Runpluginutil.T interface
type SessionPlugin struct {
	sessionPlugin ISessionPlugin
}

// NewPlugin returns a new instance of SessionPlugin which wraps a plugin that implements ISessionPlugin
func NewPlugin(newPluginFunc NewPluginFunc) (*SessionPlugin, error) {
	sessionPlugin, err := newPluginFunc()
	return &SessionPlugin{sessionPlugin}, err
}

// Execute sets up datachannel and starts execution of session manager plugin like shell
func (p *SessionPlugin) Execute(context context.T,
	config contracts.Configuration,
	cancelFlag task.CancelFlag,
	output iohandler.IOHandler) {

	log := context.Log()
	kmsKeyId := config.KmsKeyId

	dataChannel, err := getDataChannelForSessionPlugin(context, config.SessionId, config.ClientId, cancelFlag, p.sessionPlugin.InputStreamMessageHandler)
	if err != nil {
		errorString := fmt.Errorf("Setting up data channel with id %s failed: %s", config.SessionId, err)
		output.MarkAsFailed(errorString)
		log.Error(errorString)
		return
	}
	defer dataChannel.Close(log)

	if err = dataChannel.SendAgentSessionStateMessage(context.Log(), mgsContracts.Connected); err != nil {
		log.Errorf("Unable to send AgentSessionState message with session status %s. %s", mgsContracts.Connected, err)
	}

	encryptionEnabled := p.isEncryptionEnabled(kmsKeyId, config.PluginName)
	sessionTypeRequest := mgsContracts.SessionTypeRequest{
		SessionType: config.PluginName,
		Properties:  p.sessionPlugin.GetPluginParameters(config.Properties),
	}
	if p.sessionPlugin.RequireHandshake() || encryptionEnabled {
		if err = dataChannel.PerformHandshake(log, kmsKeyId, encryptionEnabled, sessionTypeRequest); err != nil {
			errorString := fmt.Errorf("Encountered error while initiating handshake. %s", err)
			output.MarkAsFailed(errorString)
			log.Error(errorString)
			return
		}
	} else {
		dataChannel.SkipHandshake(log)
	}

	p.sessionPlugin.Execute(context, config, cancelFlag, output, dataChannel)
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
