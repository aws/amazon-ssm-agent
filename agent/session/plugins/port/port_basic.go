// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	mgsConfig "github.com/aws/amazon-ssm-agent/agent/session/config"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/aws/amazon-ssm-agent/agent/session/datachannel"
)

var DialCall = func(network string, address string) (net.Conn, error) {
	return net.Dial(network, address)
}

// BasicPortSession is the type for the port session.
// It supports only one connection to the destination server.
type BasicPortSession struct {
	portSession        IPortSession
	conn               net.Conn
	serverPortNumber   string
	portType           string
	reconnectToPort    bool
	reconnectToPortErr chan error
	cancelled          chan struct{}
}

// NewBasicPortSession returns a new instance of the BasicPortSession.
func NewBasicPortSession(cancelled chan struct{}, portNumber string, portType string) (IPortSession, error) {
	var plugin = BasicPortSession{
		serverPortNumber:   portNumber,
		portType:           portType,
		reconnectToPortErr: make(chan error),
		cancelled:          cancelled,
	}
	return &plugin, nil
}

// IsConnectionAvailable returns a boolean value indicating the availability of connection to destination
func (p *BasicPortSession) IsConnectionAvailable() bool {
	return p.conn != nil
}

// HandleStreamMessage passes payload byte stream to opened connection
func (p *BasicPortSession) HandleStreamMessage(log log.T, streamDataMessage mgsContracts.AgentMessage) error {
	switch mgsContracts.PayloadType(streamDataMessage.PayloadType) {
	case mgsContracts.Output:
		log.Tracef("Output message received: %d", streamDataMessage.SequenceNumber)

		if p.reconnectToPort {
			log.Debugf("Reconnect to port: %s", p.serverPortNumber)
			err := p.InitializeSession(log)

			// Pass err to reconnectToPortErr chan to unblock writePump go routine to resume reading from localhost:p.serverPortNumber
			p.reconnectToPortErr <- err
			if err != nil {
				return err
			}

			p.reconnectToPort = false
		}

		if _, err := p.conn.Write(streamDataMessage.Payload); err != nil {
			log.Errorf("Unable to write to port, err: %v.", err)
			return err
		}
	case mgsContracts.Flag:
		var flag mgsContracts.PayloadTypeFlag
		buf := bytes.NewBuffer(streamDataMessage.Payload)
		binary.Read(buf, binary.BigEndian, &flag)

		switch flag {
		case mgsContracts.DisconnectToPort:
			// DisconnectToPort flag is sent by client when tcp connection on client side is closed.
			// In this case agent should also close tcp connection with server and wait for new data from client to reconnect.
			log.Debugf("DisconnectToPort flag received: %d", streamDataMessage.SequenceNumber)
			p.Stop()
		case mgsContracts.TerminateSession:
			log.Debugf("TerminateSession flag received: %d", streamDataMessage.SequenceNumber)
			p.cancelled <- struct{}{}
		}
	}
	return nil
}

// Stop closes the TCP Connection to port
func (p *BasicPortSession) Stop() {
	if p.conn.Close() != nil {
		p.conn.Close()
	}
}

// WritePump reads from the instance's port and writes to datachannel
func (p *BasicPortSession) WritePump(log log.T, dataChannel datachannel.IDataChannel) (errorCode int) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("WritePump thread crashed with message: \n", err)
		}
	}()

	packet := make([]byte, mgsConfig.StreamDataPayloadSize)

	for {
		numBytes, err := p.conn.Read(packet)
		if err != nil {
			var exitCode int
			if exitCode = p.handleTCPReadError(log, err); exitCode == mgsConfig.ResumeReadExitCode {
				log.Debugf("Reconnection to port %v is successful, resume reading from port.", p.serverPortNumber)
				continue
			}
			return exitCode
		}

		if err = dataChannel.SendStreamDataMessage(log, mgsContracts.Output, packet[:numBytes]); err != nil {
			log.Errorf("Unable to send stream data message: %v", err)
			return appconfig.ErrorExitCode
		}
		// Wait for TCP to process more data
		time.Sleep(time.Millisecond)
	}
}

// InitializeSession dials a connection to port
func (p *BasicPortSession) InitializeSession(log log.T) (err error) {
	if p.conn, err = DialCall("tcp", "localhost:"+p.serverPortNumber); err != nil {
		return errors.New(fmt.Sprintf("Unable to connect to specified port: %v", err))
	}
	return nil
}

// handleTCPReadError handles TCP read error
func (p *BasicPortSession) handleTCPReadError(log log.T, err error) int {
	if p.portType == mgsConfig.LocalPortForwarding {
		log.Debugf("Initiating reconnection to port %s as existing connection resulted in read error: %v", p.serverPortNumber, err)
		return p.handlePortError(log, err)
	}
	return p.handleSSHDPortError(log, err)
}

// handleSSHDPortError handles error by returning proper exit code based on error encountered
func (p *BasicPortSession) handleSSHDPortError(log log.T, err error) int {
	if err == io.EOF {
		log.Infof("TCP Connection was closed.")
		return appconfig.SuccessExitCode
	} else {
		log.Errorf("Failed to read from port: %v", err)
		return appconfig.ErrorExitCode
	}
}

// handlePortError handles error by initiating reconnection to port in case of read failure
func (p *BasicPortSession) handlePortError(log log.T, err error) int {
	// Read from tcp connection to localhost:p.serverPortNumber resulted in error. Close existing connection and
	// set reconnectToPort to true. ReconnectToPort is used when new steam data message arrives on
	// web socket channel to trigger reconnection to localhost:p.serverPortNumber.
	log.Debugf("Encountered error while reading from port %v, %v", p.serverPortNumber, err)
	p.Stop()
	p.reconnectToPort = true

	log.Debugf("Waiting for reconnection to port!!")
	err = <-p.reconnectToPortErr

	if err != nil {
		log.Error(err)
		return appconfig.ErrorExitCode
	}

	// Reconnection to localhost:p.portPlugin is successful, return resume code to starting reading from connection
	return mgsConfig.ResumeReadExitCode
}
