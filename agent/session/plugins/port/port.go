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
	"net"
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
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ecs"
)

const muxSupportedClientVersion = "1.1.70"
const muxKeepAliveDisabledAfterThisClientVersion = "1.2.331.0"

// PortParameters contains inputs required to execute port plugin.
type PortParameters struct {
	Host       string `json:"host" yaml:"host"`
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

var lookupHost = net.LookupHost

var getMetadataIdentity = identity.GetMetadataIdentity

var newEC2Identity = func(log log.T, appConfig *appconfig.SsmagentConfig) identity.IAgentIdentityInner {
	return ec2.NewEC2Identity(log)
}

var newECSIdentity = func(log log.T, _ *appconfig.SsmagentConfig) identity.IAgentIdentityInner {
	return ecs.NewECSIdentity(log)
}

// GetSession initializes session based on the type of the port session
// mux for port forwarding session and if client supports multiplexing; basic otherwise
var GetSession = func(context context.T, portParameters PortParameters, cancelled chan struct{}, clientVersion string, sessionId string) (session IPortSession, err error) {
	host := "localhost"
	if portParameters.Host != "" {
		host = portParameters.Host
		context.Log().Debug("Using remote host: %s", host)
	}
	destinationAddress := net.JoinHostPort(host, portParameters.PortNumber)

	if portParameters.Type == mgsConfig.LocalPortForwarding &&
		versionutil.Compare(clientVersion, muxSupportedClientVersion, true) >= 0 {

		if session, err = NewMuxPortSession(context, clientVersion, cancelled, destinationAddress, sessionId); err == nil {
			return session, nil
		}
	} else {
		if session, err = NewBasicPortSession(context, cancelled, destinationAddress, portParameters.Type); err == nil {
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

	if err := p.validateParameters(portParameters, config); err != nil {
		return err
	}

	p.session, err = GetSession(p.context, portParameters, p.cancelled, p.dataChannel.GetClientVersion(), config.SessionId)

	return
}

// validateParameters validates port plugin parameters
func (p *PortPlugin) validateParameters(portParameters PortParameters, config agentContracts.Configuration) (err error) {
	if portParameters.PortNumber == "" {
		return errors.New(fmt.Sprintf("Port number is empty in session properties. %v", config.Properties))
	}

	if portParameters.Host == "" {
		return
	}

	appConfig := p.context.AppConfig()
	dnsAddress, err := dnsRoutingAddress(p.context.Log(), &appConfig)
	if err != nil {
		p.context.Log().Warn("Error retrieving vpc dns address: %v", err)
	}

	resolvedAddresses, err := lookupHost(portParameters.Host)
	if portParameters.Host != "" && err == nil {
		for _, host := range resolvedAddresses {
			// Port forwarding to IMDS, VPC DNS, and local IP address is not allowed
			hostIPAddress := net.ParseIP(host)
			for _, address := range append(appConfig.Mgs.DeniedPortForwardingRemoteIPs, dnsAddress...) {
				if hostIPAddress.Equal(net.ParseIP(address)) {
					return errors.New(fmt.Sprintf("Forwarding to IP address %s is forbidden.", portParameters.Host))
				}
			}
		}
	}

	return
}

func dnsRoutingAddress(log log.T, appConfig *appconfig.SsmagentConfig) ([]string, error) {
	var ipaddress map[string][]string
	var err error

	ec2I := newEC2Identity(log, appConfig)
	ecsI := newECSIdentity(log, nil)
	if ecsI.IsIdentityEnvironment() {
		if metadataI, ok := getMetadataIdentity(ecsI); ok {
			ipaddress, err = metadataI.VpcPrimaryCIDRBlock()
		}
	} else if ec2I.IsIdentityEnvironment() {
		if metadataI, ok := getMetadataIdentity(ec2I); ok {
			ipaddress, err = metadataI.VpcPrimaryCIDRBlock()
		}
	}

	calculation := calculateAddress(ipaddress)

	return calculation, err
}

func calculateAddress(ipaddresses map[string][]string) []string {
	var calculation []string
	for _, ipversion := range []string{"ipv4", "ipv6"} {
		for _, ipaddress := range ipaddresses[ipversion] {
			ip, ipnet, err := net.ParseCIDR(ipaddress)
			if err != nil {
				continue
			}
			address := make([]string, 5)
			dnsAddress := make(net.IP, len(ip))
			copy(dnsAddress, ip)
			// add  the first four and last ip address in the VPC CIDR block to deny list
			for i := 0; i < 4; i++ {
				address[i] = dnsAddress.String()
				dnsAddress[len(ip)-1] += 1
			}
			address[len(address)-1] = broadcastAddress(ipnet.IP, ipnet.Mask)
			calculation = append(calculation, address...)
		}
	}
	return calculation
}

func broadcastAddress(ip net.IP, mask net.IPMask) string {
	// ip | ~mask
	address := make(net.IP, len(ip))
	for i, bit := range mask {
		address[i] = ip[i] | ^bit
	}
	return address.String()
}
