// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package port implements session manager's port plugin
package port

import (
	"errors"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	agentContracts "github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	mgsConfig "github.com/aws/amazon-ssm-agent/agent/session/config"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/aws/amazon-ssm-agent/agent/session/datachannel"
	"github.com/aws/amazon-ssm-agent/agent/session/plugins/sessionplugin"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/versionutil"
)

const muxSupportedClientVersion = "1.1.70"

// PortParameters contains inputs required to execute port plugin.
type PortParameters struct {
	PortNumber string `json:"portNumber" yaml:"portNumber"`
	Type       string `json:"type"`
}

// Plugin is the type for the port plugin.
type PortPlugin struct {
	context     context.T
	dataChannel datachannel.IDataChannel
	cancelled   chan struct{}
	session     IPortSession
}

// IPortSession interface represents functions that need to be implemented by all port sessions
type IPortSession interface {
	InitializeSession() (err error)
	HandleStreamMessage(streamDataMessage mgsContracts.AgentMessage) (err error)
	WritePump(channel datachannel.IDataChannel) (errorCode int)
	IsConnectionAvailable() (isAvailable bool)
	Stop()
}

// GetSession initializes session based on the type of the port session
// mux for port forwarding session and if client supports multiplexing; basic otherwise
var GetSession = func(context context.T, portParameters PortParameters, cancelled chan struct{}, clientVersion string, sessionId string) (session IPortSession, err error) {
	if portParameters.Type == mgsConfig.LocalPortForwarding &&
		versionutil.Compare(clientVersion, muxSupportedClientVersion, true) >= 0 {

		if session, err = NewMuxPortSession(context, cancelled, portParameters.PortNumber, sessionId); err == nil {
			return session, nil
		}
	} else {
		if session, err = NewBasicPortSession(context, cancelled, portParameters.PortNumber, portParameters.Type); err == nil {
			return session, nil
		}
	}
	return nil, err
}

// Returns parameters required for CLI to start session
func (p *PortPlugin) GetPluginParameters(parameters interface{}) interface{} {
	return parameters
}

// Port plugin requires handshake to establish session
func (p *PortPlugin) RequireHandshake() bool {
	return true
}

// NewPortPlugin returns a new instance of the Port Plugin.
func NewPlugin(context context.T) (sessionplugin.ISessionPlugin, error) {
	var plugin = PortPlugin{
		context:   context,
		cancelled: make(chan struct{}),
	}
	return &plugin, nil
}

// Name returns the name of Port Plugin
func (p *PortPlugin) name() string {
	return appconfig.PluginNamePort
}

// Execute establishes a connection to a specified port from the parameters
// It reads incoming messages from the data channel and writes to the port
// It reads from the port and writes to the data channel
func (p *PortPlugin) Execute(
	config agentContracts.Configuration,
	cancelFlag task.CancelFlag,
	output iohandler.IOHandler,
	dataChannel datachannel.IDataChannel) {

	log := p.context.Log()
	p.dataChannel = dataChannel
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("Error occurred while executing plugin %s: \n%v", p.name(), err)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
			os.Exit(1)
		}
	}()

	if cancelFlag.ShutDown() {
		output.MarkAsShutdown()
	} else if cancelFlag.Canceled() {
		output.MarkAsCancelled()
	} else {
		p.execute(config, cancelFlag, output)
	}
}

// Execute establishes a connection to a specified port from the parameters
// It reads incoming messages from the data channel and writes to the port
// It reads from the port and writes to the data channel
func (p *PortPlugin) execute(
	config agentContracts.Configuration,
	cancelFlag task.CancelFlag,
	output iohandler.IOHandler) {

	log := p.context.Log()
	var err error
	sessionPluginResultOutput := mgsContracts.SessionPluginResultOutput{}

	defer func() {
		p.stop()
	}()

	if err = p.initializeParameters(config); err != nil {
		log.Error(err)
		output.SetExitCode(appconfig.ErrorExitCode)
		output.SetStatus(agentContracts.ResultStatusFailed)
		sessionPluginResultOutput.Output = err.Error()
		output.SetOutput(sessionPluginResultOutput)
		return
	}

	if err = p.session.InitializeSession(); err != nil {
		log.Error(err)
		output.SetExitCode(appconfig.ErrorExitCode)
		output.SetStatus(agentContracts.ResultStatusFailed)
		sessionPluginResultOutput.Output = err.Error()
		output.SetOutput(sessionPluginResultOutput)
		return
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("Session cancel flag panic: \n%v", r)
				log.Errorf("Stacktrace:\n%s", debug.Stack())
			}
		}()
		cancelState := cancelFlag.Wait()
		if cancelFlag.Canceled() {
			p.cancelled <- struct{}{}
			log.Debug("Cancel flag set to cancelled in session")
		}
		log.Debugf("Cancel flag set to %v in session", cancelState)
	}()

	log.Debugf("Start separate go routine to read from port connection and write to data channel")
	done := make(chan int, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("Session write pump panic: \n%v", r)
				log.Errorf("Stacktrace:\n%s", debug.Stack())
			}
		}()
		done <- p.session.WritePump(p.dataChannel)
	}()
	log.Infof("Plugin %s started", p.name())

	select {
	case <-p.cancelled:
		log.Debug("Session cancelled. Attempting to close TCP Connection.")
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

	log.Debug("Port session execution complete")
}

// InputStreamMessageHandler passes payload byte stream to port
func (p *PortPlugin) InputStreamMessageHandler(log log.T, streamDataMessage mgsContracts.AgentMessage) error {
	if p.session == nil || !p.session.IsConnectionAvailable() {
		// This is to handle scenario when cli/console starts sending data but session has not been initialized yet
		// Since packets are rejected, cli/console will resend these packets until tcp starts successfully in separate thread
		log.Tracef("TCP connection unavailable. Reject incoming message packet")
		return mgsContracts.ErrHandlerNotReady
	}
	return p.session.HandleStreamMessage(streamDataMessage)
}

// Stop closes all opened connections to port
func (p *PortPlugin) stop() {
	p.context.Log().Debug("Closing all connections")
	if p.session != nil {
		p.session.Stop()
	}
}

// initializeParameters initializes PortPlugin with input parameters
func (p *PortPlugin) initializeParameters(config agentContracts.Configuration) (err error) {
	var portParameters PortParameters
	if err = jsonutil.Remarshal(config.Properties, &portParameters); err != nil {
		return errors.New(fmt.Sprintf("Unable to remarshal session properties. %v", err))
	}

	if portParameters.PortNumber == "" {
		return errors.New(fmt.Sprintf("Port number is empty in session properties. %v", config.Properties))
	}
	p.session, err = GetSession(p.context, portParameters, p.cancelled, p.dataChannel.GetClientVersion(), config.SessionId)

	return
}
