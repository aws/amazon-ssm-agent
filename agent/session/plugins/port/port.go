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
	"io"
	"net"
	"os"
	"time"

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
)

var DialCall = func(network string, address string) (net.Conn, error) {
	return net.Dial(network, address)
}

// PortParameters contains inputs required to execute port plugin.
type PortParameters struct {
	PortNumber string `json:"portNumber" yaml:"portNumber"`
}

// Plugin is the type for the port plugin.
type PortPlugin struct {
	tcpConn     net.Conn
	dataChannel datachannel.IDataChannel
}

// Returns parameters required for CLI to start session
func (p *PortPlugin) GetPluginParameters(parameters interface{}) interface{} {
	return parameters
}

// Port plugin requires handshake to establish session
func (p *PortPlugin) RequireHandshake() bool {
	return true
}

// NewPlugin returns a new instance of the Port Plugin.
func NewPlugin() (sessionplugin.ISessionPlugin, error) {
	var plugin = PortPlugin{}
	return &plugin, nil
}

// Name returns the name of Port Plugin
func (p *PortPlugin) name() string {
	return appconfig.PluginNamePort
}

// Execute establishes a connection to a specified port from the parameters
// It reads incoming messages from the data channel and writes to the port
// It reads from the  port and writes to the data channel
func (p *PortPlugin) Execute(context context.T,
	config agentContracts.Configuration,
	cancelFlag task.CancelFlag,
	output iohandler.IOHandler,
	dataChannel datachannel.IDataChannel) {

	log := context.Log()
	p.dataChannel = dataChannel
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("Error occurred while executing plugin %s: \n%v", p.name(), err)
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

// Execute establishes a connection to a specified port from the parameters
// It reads incoming messages from the data channel and writes to the port
// It reads from the port and writes to the data channel
func (p *PortPlugin) execute(context context.T,
	config agentContracts.Configuration,
	cancelFlag task.CancelFlag,
	output iohandler.IOHandler) {

	log := context.Log()
	var err error
	sessionPluginResultOutput := mgsContracts.SessionPluginResultOutput{}

	defer func() {
		if err := p.Stop(log); err != nil {
			log.Errorf("Error occurred while closing TCP connection: %v", err)
		}
	}()

	if p.tcpConn, err = p.startTCPConn(log, config.Properties); err != nil {
		log.Error(err)
		output.SetExitCode(appconfig.ErrorExitCode)
		output.SetStatus(agentContracts.ResultStatusFailed)
		sessionPluginResultOutput.Output = err.Error()
		output.SetOutput(sessionPluginResultOutput)
		return
	}

	cancelled := make(chan bool, 1)
	go func() {
		cancelState := cancelFlag.Wait()
		if cancelFlag.Canceled() {
			cancelled <- true
			log.Debug("Cancel flag set to cancelled in session")
		}
		log.Debugf("Cancel flag set to %v in session", cancelState)
	}()

	log.Debugf("Start separate go routine to read from port connection and write to data channel")
	done := make(chan int, 1)
	go func() {
		done <- p.writePump(log)
	}()

	log.Infof("Plugin %s started", p.name())

	select {
	case <-cancelled:
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
	if p.tcpConn == nil {
		// This is to handle scenario when cli/console starts sending data but port has not been opened yet
		// Since packets are rejected, cli/console will resend these packets until tcp starts successfully in separate thread
		log.Tracef("TCP connection unavailable. Reject incoming message packet")
		return nil
	}

	switch mgsContracts.PayloadType(streamDataMessage.PayloadType) {
	case mgsContracts.Output:
		log.Tracef("Output message received: %d", streamDataMessage.SequenceNumber)

		if _, err := p.tcpConn.Write(streamDataMessage.Payload); err != nil {
			log.Errorf("Unable to write to port, err: %v.", err)
			return err
		}
	}
	return nil
}

// Stop closes the TCP Connection to the instance
func (p *PortPlugin) Stop(log log.T) error {
	if p.tcpConn != nil {
		log.Debug("Closing TCP connection")
		if err := p.tcpConn.Close(); err != nil {
			return log.Errorf("Unable to close connection to port. %v", err)
		}
	}
	return nil
}

// writePump reads from the instance's port and writes to data channel
func (p *PortPlugin) writePump(log log.T) (errorCode int) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("WritePump thread crashed with message: \n", err)
		}
	}()

	packet := make([]byte, mgsConfig.StreamDataPayloadSize)

	for {
		numBytes, err := p.tcpConn.Read(packet)
		if err != nil {
			return p.handleTCPReadError(log, err)
		}

		if err = p.dataChannel.SendStreamDataMessage(log, mgsContracts.Output, packet[:numBytes]); err != nil {
			log.Errorf("Unable to send stream data message: %v", err)
			return appconfig.ErrorExitCode
		}
		// Wait for TCP to process more data
		time.Sleep(time.Millisecond)
	}
}

// handleTCPReadError handles TCP read error
func (p *PortPlugin) handleTCPReadError(log log.T, err error) int {
	if err == io.EOF {
		log.Infof("TCP Connection was closed.")
		return appconfig.SuccessExitCode
	} else {
		log.Errorf("Failed to read from port: %v", err)
		return appconfig.ErrorExitCode
	}
}

// startTCPConn starts TCP connection to the specified port
func (p *PortPlugin) startTCPConn(log log.T, parameters interface{}) (tcpConn net.Conn, err error) {
	var portParameters PortParameters
	if err = jsonutil.Remarshal(parameters, &portParameters); err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to remarshal session properties. %v", err))
	}

	if portParameters.PortNumber == "" {
		return nil, errors.New(fmt.Sprintf("Port number is empty in session properties. %v", parameters))
	}

	if tcpConn, err = DialCall("tcp", "localhost:"+portParameters.PortNumber); err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to connect to specified port: %v", err))
	}

	return tcpConn, nil
}
