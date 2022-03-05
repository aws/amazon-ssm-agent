// +build windows

// Copyright 2019 The Mangos Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use file except in compliance with the License.
// You may obtain a copy of the license at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package ipc implements the IPC transport on top of Windows Named Pipes.
// To enable it simply import it.
package ipc

import (
	"net"
	"sync"
	"sync/atomic"

	"github.com/Microsoft/go-winio"

	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/transport"
)

const Transport = ipcTran(0)

func init() {
	transport.RegisterTransport(Transport)
}

// The options here are pretty specific to Windows Named Pipes.

const (
	// OptionSecurityDescriptor represents a Windows security
	// descriptor in SDDL format (string).  This can only be set on
	// a Listener, and must be set before the Listen routine
	// is called.
	OptionSecurityDescriptor = "WIN-IPC-SECURITY-DESCRIPTOR"

	// OptionInputBufferSize represents the Windows Named Pipe
	// input buffer size in bytes (type int32).  Default is 4096.
	// This is only for Listeners, and must be set before the
	// Listener is started.
	OptionInputBufferSize = "WIN-IPC-INPUT-BUFFER-SIZE"

	// OptionOutputBufferSize represents the Windows Named Pipe
	// output buffer size in bytes (type int32).  Default is 4096.
	// This is only for Listeners, and must be set before the
	// Listener is started.
	OptionOutputBufferSize = "WIN-IPC-OUTPUT-BUFFER-SIZE"
)

type dialer struct {
	path        string
	proto       transport.ProtocolInfo
	hs          transport.Handshaker
	recvMaxSize int32
	lock        sync.Mutex
}

// Dial implements the PipeDialer Dial method.
func (d *dialer) Dial() (transport.Pipe, error) {

	conn, err := winio.DialPipe("\\\\.\\pipe\\"+d.path, nil)
	if err != nil {
		return nil, err
	}
	p := transport.NewConnPipeIPC(conn, d.proto)
	p.SetMaxRecvSize(int(atomic.LoadInt32(&d.recvMaxSize)))
	d.hs.Start(p)
	return d.hs.Wait()
}

// SetOption implements a stub PipeDialer SetOption method.
func (d *dialer) SetOption(n string, v interface{}) error {
	switch n {
	case mangos.OptionMaxRecvSize:
		if val, ok := v.(int); ok {
			atomic.StoreInt32(&d.recvMaxSize, int32(val))
			return nil
		}
		return mangos.ErrBadValue
	}

	return mangos.ErrBadOption
}

// GetOption implements a stub PipeDialer GetOption method.
func (d *dialer) GetOption(n string) (interface{}, error) {
	switch n {
	case mangos.OptionMaxRecvSize:
		return int(atomic.LoadInt32(&d.recvMaxSize)), nil
	}
	return nil, mangos.ErrBadOption
}

type listener struct {
	path             string
	proto            transport.ProtocolInfo
	listener         net.Listener
	hs               transport.Handshaker
	closed           bool
	recvMaxSize      int32
	outputBufferSize int32
	inputBufferSize  int32
	securityDesc     string
	once             sync.Once
	lock             sync.Mutex
}

// Listen implements the PipeListener Listen method.
func (l *listener) Listen() error {

	l.lock.Lock()
	config := &winio.PipeConfig{
		InputBufferSize:    atomic.LoadInt32(&l.inputBufferSize),
		OutputBufferSize:   atomic.LoadInt32(&l.outputBufferSize),
		SecurityDescriptor: l.securityDesc,
		MessageMode:        false,
	}
	if l.closed {
		l.lock.Unlock()
		return mangos.ErrClosed
	}
	l.lock.Unlock()

	listener, err := winio.ListenPipe("\\\\.\\pipe\\"+l.path, config)
	if err != nil {
		return err
	}
	l.listener = listener
	go func() {
		for {
			l.lock.Lock()
			if l.closed {
				l.lock.Unlock()
				return
			}
			l.lock.Unlock()
			conn, err := listener.Accept()
			if err != nil {
				// Generally this will be ErrClosed
				continue
			}
			p := transport.NewConnPipeIPC(conn, l.proto)
			p.SetMaxRecvSize(int(atomic.LoadInt32(&l.recvMaxSize)))
			l.hs.Start(p)
		}
	}()
	return nil
}

func (l *listener) Address() string {
	return "ipc://" + l.path
}

// Accept implements the the PipeListener Accept method.
func (l *listener) Accept() (mangos.TranPipe, error) {

	if l.listener == nil {
		return nil, mangos.ErrClosed
	}
	return l.hs.Wait()
}

// Close implements the PipeListener Close method.
func (l *listener) Close() error {
	l.once.Do(func() {
		l.lock.Lock()
		l.closed = true
		l.lock.Unlock()
		if l.listener != nil {
			_ = l.listener.Close()
		}
		l.hs.Close()
	})
	return nil
}

// SetOption implements a stub PipeListener SetOption method.
func (l *listener) SetOption(name string, val interface{}) error {
	switch name {
	case OptionInputBufferSize:
		if b, ok := val.(int32); ok {
			atomic.StoreInt32(&l.inputBufferSize, b)
			return nil
		}
		return mangos.ErrBadValue
	case OptionOutputBufferSize:
		if b, ok := val.(int32); ok {
			atomic.StoreInt32(&l.outputBufferSize, b)
			return nil
		}
		return mangos.ErrBadValue

	case OptionSecurityDescriptor:
		if b, ok := val.(string); ok {
			l.securityDesc = b
			return nil
		}
		return mangos.ErrBadValue

	case mangos.OptionMaxRecvSize:
		if b, ok := val.(int); ok {
			atomic.StoreInt32(&l.recvMaxSize, int32(b))
			return nil
		}
		return mangos.ErrBadValue
	default:
		return mangos.ErrBadOption
	}
}

// GetOption implements a stub PipeListener GetOption method.
func (l *listener) GetOption(name string) (interface{}, error) {
	switch name {
	case mangos.OptionMaxRecvSize:
		return int(atomic.LoadInt32(&l.recvMaxSize)), nil
	case OptionInputBufferSize:
		return atomic.LoadInt32(&l.inputBufferSize), nil
	case OptionOutputBufferSize:
		return atomic.LoadInt32(&l.outputBufferSize), nil
	case OptionSecurityDescriptor:
		return l.securityDesc, nil
	}
	return nil, mangos.ErrBadOption
}

type ipcTran int

// Scheme implements the Transport Scheme method.
func (ipcTran) Scheme() string {
	return "ipc"
}

// NewDialer implements the Transport NewDialer method.
func (t ipcTran) NewDialer(address string, sock mangos.Socket) (mangos.TranDialer, error) {
	var err error

	if address, err = transport.StripScheme(t, address); err != nil {
		return nil, err
	}

	d := &dialer{
		proto: sock.Info(),
		path:  address,
		hs:    transport.NewConnHandshaker(),
	}

	return d, nil
}

// NewListener implements the Transport NewListener method.
func (t ipcTran) NewListener(address string, sock mangos.Socket) (transport.Listener, error) {
	var err error

	if address, err = transport.StripScheme(t, address); err != nil {
		return nil, err
	}

	l := &listener{
		proto: sock.Info(),
		path:  address,
		hs:    transport.NewConnHandshaker(),
	}

	l.inputBufferSize = 4096
	l.outputBufferSize = 4096
	l.securityDesc = ""
	l.recvMaxSize = 0

	return l, nil
}
