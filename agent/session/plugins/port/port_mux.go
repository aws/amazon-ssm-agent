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

// Package port implements session manager's port plugin.
package port

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	agentContext "github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	mgsConfig "github.com/aws/amazon-ssm-agent/agent/session/config"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/aws/amazon-ssm-agent/agent/session/datachannel"
	"github.com/aws/amazon-ssm-agent/agent/session/utility"
	"github.com/xtaci/smux"
	"golang.org/x/sync/errgroup"
)

// MgsConn contains local server and corresponding server side connection to smux server
type MgsConn struct {
	listener net.Listener
	conn     net.Conn
}

// MuxServer contains smux server session and corresponding network connection
type MuxServer struct {
	conn    net.Conn
	session *smux.Session
}

// MuxPortSession is the type for the multiplexer port session.
// supports making multiple connections to the destination server.
type MuxPortSession struct {
	context          agentContext.T
	portSession      IPortSession
	cancelled        chan struct{}
	serverPortNumber string
	sessionId        string
	socketFile       string
	muxServer        *MuxServer
	mgsConn          *MgsConn
}

func (c *MgsConn) close() {
	c.listener.Close()
	c.conn.Close()
}

func (s *MuxServer) close() {
	s.session.Close()
	s.conn.Close()
}

// NewMuxPortSession returns a new instance of the MuxPortSession.
func NewMuxPortSession(context agentContext.T, cancelled chan struct{}, portNumber string, sessionId string) (IPortSession, error) {
	var plugin = MuxPortSession{
		context:          context,
		cancelled:        cancelled,
		serverPortNumber: portNumber,
		sessionId:        sessionId}
	return &plugin, nil
}

// IsConnectionAvailable returns a boolean value indicating the availability of connection to destination
func (p *MuxPortSession) IsConnectionAvailable() bool {
	return p.mgsConn != nil && p.muxServer != nil
}

// HandleStreamMessage passes payload byte stream to smux server
func (p *MuxPortSession) HandleStreamMessage(streamDataMessage mgsContracts.AgentMessage) error {
	log := p.context.Log()
	switch mgsContracts.PayloadType(streamDataMessage.PayloadType) {
	case mgsContracts.Output:
		log.Tracef("Output message received: %d", streamDataMessage.SequenceNumber)
		if _, err := p.mgsConn.conn.Write(streamDataMessage.Payload); err != nil {
			log.Errorf("Unable to write to port, err: %v.", err)
			return err
		}
	case mgsContracts.Flag:
		var flag mgsContracts.PayloadTypeFlag
		buf := bytes.NewBuffer(streamDataMessage.Payload)
		binary.Read(buf, binary.BigEndian, &flag)

		switch flag {
		case mgsContracts.TerminateSession:
			log.Debugf("TerminateSession flag received: %d", streamDataMessage.SequenceNumber)
			p.cancelled <- struct{}{}
		}
	}
	return nil
}

// Stop closes all opened connections on instance
func (p *MuxPortSession) Stop() {
	if p.mgsConn != nil {
		p.mgsConn.close()
	}
	if p.muxServer != nil {
		p.muxServer.close()
	}
	p.cleanUp()
}

// WritePump handles communication between <smux server, datachannel> and <smux server, destination server>
func (p *MuxPortSession) WritePump(dataChannel datachannel.IDataChannel) (errorCode int) {
	log := p.context.Log()
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("WritePump thread crashed with message: %v", err)
		}
	}()

	g, ctx := errgroup.WithContext(context.Background())

	// go routine to read packets from smux server and send on datachannel
	g.Go(func() error {
		return p.transferDataToMgs(ctx, dataChannel)
	})

	// go routine for smux server to accept streams (client connections) and dials connections to destination server
	g.Go(func() error {
		return p.handleServerConnections(ctx, dataChannel)
	})

	if err := g.Wait(); err != nil {
		return appconfig.ErrorExitCode
	}

	return appconfig.SuccessExitCode
}

// InitializeSession initializes MuxPortSession
func (p *MuxPortSession) InitializeSession() (err error) {
	fileutil.MakeDirs(appconfig.SessionFilesPath)
	p.socketFile = getUnixSocketPath(p.sessionId, appconfig.SessionFilesPath, "mux.sock")

	if err = p.initialize(); err != nil {
		p.cleanUp()
	}
	return
}

// initialize starts smux server and corresponding connections
func (p *MuxPortSession) initialize() (err error) {
	log := p.context.Log()
	var listener net.Listener
	// start a local listener
	if listener, err = utility.NewListener(log, p.socketFile); err != nil {
		log.Errorf("Unable to start local server: %v", err)
		return
	}

	var g errgroup.Group
	g.Go(func() error {
		var conn net.Conn
		if conn, err = listener.Accept(); err != nil {
			log.Errorf("Unable to accept connection: %v", err)
			return err
		}
		log.Debugf("Accepted a connection %s\n", conn.LocalAddr())

		var session *smux.Session
		if session, err = smux.Server(conn, nil); err != nil {
			log.Errorf("Unable to setup smux server: %v", err)
			return err
		}

		p.muxServer = &MuxServer{conn: conn, session: session}
		return nil
	})

	// start network connection
	g.Go(func() error {
		conn, err := net.Dial(listener.Addr().Network(), listener.Addr().String())
		if err != nil {
			log.Errorf("Unable to dial connection to listener on %s: %v", listener.Addr().String(), err)
			return err
		}
		p.mgsConn = &MgsConn{listener: listener, conn: conn}
		return nil
	})
	return g.Wait()
}

// cleanUp deletes unix socket file
func (p *MuxPortSession) cleanUp() {
	os.Remove(p.socketFile)
}

// transferDataToMgs reads data from smux server and sends on data channel.
func (p *MuxPortSession) transferDataToMgs(ctx context.Context, dataChannel datachannel.IDataChannel) error {
	log := p.context.Log()
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Transfer data to mgs crashed with message: %v", r)
		}
	}()
	for {
		if dataChannel.IsActive() {
			packet := make([]byte, mgsConfig.StreamDataPayloadSize)
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				numBytes, err := p.mgsConn.conn.Read(packet)
				if err != nil {
					log.Errorf("Unable to read from connection: %v", err)
					return err
				}

				if err = dataChannel.SendStreamDataMessage(log, mgsContracts.Output, packet[:numBytes]); err != nil {
					log.Errorf("Unable to send stream data message: %v", err)
					return err
				}
			}
		}
		time.Sleep(time.Millisecond)
	}
}

// handleServerConnections sets up smux stream and handles communication between smux stream and destination server.
func (p *MuxPortSession) handleServerConnections(ctx context.Context, dataChannel datachannel.IDataChannel) error {
	log := p.context.Log()
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Handle server connections crashed with message: %v", r)
		}
	}()
	// net.Dial assumes local system when host in addr is empty
	localAddr := fmt.Sprintf(":%s", p.serverPortNumber)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			stream, err := p.muxServer.session.AcceptStream()
			if err != nil {
				log.Errorf("Unable to accept stream: %v", err)
				return err
			}

			log.Debugf("Started a new mux stream %d\n", stream.ID())

			if conn, err := net.Dial("tcp", localAddr); err == nil {
				log.Tracef("Established connection to port %s", p.serverPortNumber)
				go func() {
					defer func() {
						if r := recover(); r != nil {
							log.Errorf("Handle data transfer crashed with message: %v", r)
						}
					}()
					handleDataTransfer(stream, conn)
				}()
			} else {
				log.Errorf("Unable to dial connection to server: %v", err)
				flagBuf := new(bytes.Buffer)
				binary.Write(flagBuf, binary.BigEndian, mgsContracts.ConnectToPortError)
				dataChannel.SendStreamDataMessage(log, mgsContracts.Flag, flagBuf.Bytes())
				stream.Close()
			}
		}
	}
}

// handleDataTransfer launches routines to transfer data between source and destination
func handleDataTransfer(dst io.ReadWriteCloser, src io.ReadWriteCloser) {
	var wait sync.WaitGroup
	wait.Add(2)

	go func() {
		io.Copy(dst, src)
		dst.Close()
		wait.Done()
	}()

	go func() {
		io.Copy(src, dst)
		src.Close()
		wait.Done()
	}()

	wait.Wait()
}

// getUnixSocketPath generates the unix socket file name based on sessionId and returns the path.
func getUnixSocketPath(sessionId string, dir string, suffix string) string {
	hash := fnv.New32a()
	hash.Write([]byte(sessionId))
	return filepath.Join(dir, fmt.Sprintf("%d_%s", hash.Sum32(), suffix))
}
