// +build !windows,!plan9,!js

// Copyright 2020 The Mangos Authors
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

// Package ipc implements the IPC transport on top of UNIX domain sockets.
// To enable it simply import it.
package ipc

import (
	"net"
	"os"
	"sync"
	"syscall"
	"time"

	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/transport"
)

const (
	// Transport is a transport.Transport for IPC.
	Transport = ipcTran(0)
)

func init() {
	transport.RegisterTransport(Transport)
}

type dialer struct {
	addr        *net.UnixAddr
	proto       transport.ProtocolInfo
	hs          transport.Handshaker
	maxRecvSize int
	lock        sync.Mutex
}

// Dial implements the Dialer Dial method
func (d *dialer) Dial() (transport.Pipe, error) {

	conn, err := net.DialUnix("unix", nil, d.addr)
	if err != nil {
		return nil, err
	}
	p := transport.NewConnPipeIPC(conn, d.proto)
	d.lock.Lock()
	p.SetOption(mangos.OptionMaxRecvSize, d.maxRecvSize)
	getPeer(conn, p)
	d.lock.Unlock()
	d.hs.Start(p)
	return d.hs.Wait()
}

// SetOption implements Dialer SetOption method.
func (d *dialer) SetOption(n string, v interface{}) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	switch n {
	case mangos.OptionMaxRecvSize:
		if b, ok := v.(int); ok {
			d.maxRecvSize = b
			return nil
		}
		return mangos.ErrBadValue
	}
	return mangos.ErrBadOption
}

// GetOption implements Dialer GetOption method.
func (d *dialer) GetOption(n string) (interface{}, error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	switch n {
	case mangos.OptionMaxRecvSize:
		return d.maxRecvSize, nil
	}
	return nil, mangos.ErrBadOption
}

type listener struct {
	addr        *net.UnixAddr
	proto       transport.ProtocolInfo
	listener    *net.UnixListener
	hs          transport.Handshaker
	closeq      chan struct{}
	closed      bool
	maxRecvSize int
	owner       int
	group       int
	chown       bool
	mode        uint32
	chmod       bool
	once        sync.Once
	lock        sync.Mutex
}

// Listen implements the PipeListener Listen method.
func (l *listener) Listen() error {
	l.lock.Lock()
	defer l.lock.Unlock()

	select {
	case <-l.closeq:
		return mangos.ErrClosed
	default:
	}
	listener, err := net.ListenUnix("unix", l.addr)

	if l.chown {
		os.Chown(l.addr.String(), l.owner, l.group)

	}
	if l.chmod {
		os.Chmod(l.addr.String(), os.FileMode(l.mode))
	}

	if err != nil && (isSyscallError(err, syscall.EADDRINUSE) || isSyscallError(err, syscall.EEXIST)) {
		l.removeStaleIPC()
		listener, err = net.ListenUnix("unix", l.addr)
		if isSyscallError(err, syscall.EADDRINUSE) || isSyscallError(err, syscall.EEXIST) {
			err = mangos.ErrAddrInUse
		}
	}
	if err != nil {
		return err
	}
	l.listener = listener
	go func() {
		for {
			conn, err := l.listener.AcceptUnix()
			if err != nil {
				select {
				case <-l.closeq:
					return
				default:
					continue
				}
			}
			p := transport.NewConnPipeIPC(conn, l.proto)
			l.lock.Lock()
			p.SetOption(mangos.OptionMaxRecvSize, l.maxRecvSize)
			getPeer(conn, p)
			l.lock.Unlock()
			l.hs.Start(p)
		}
	}()
	return nil
}

func (l *listener) Address() string {
	return "ipc://" + l.addr.String()
}

// Accept implements the the PipeListener Accept method.
func (l *listener) Accept() (transport.Pipe, error) {
	l.lock.Lock()
	if l.listener == nil {
		l.lock.Unlock()
		return nil, mangos.ErrClosed
	}
	l.lock.Unlock()
	return l.hs.Wait()
}

// Close implements the PipeListener Close method.
func (l *listener) Close() error {
	l.once.Do(func() {
		l.lock.Lock()
		l.closed = true
		if l.listener != nil {
			l.listener.Close()
		}
		l.hs.Close()
		close(l.closeq)
		l.lock.Unlock()
	})
	return nil
}

// SetOption implements a stub PipeListener SetOption method.
func (l *listener) SetOption(n string, v interface{}) error {
	l.lock.Lock()
	defer l.lock.Unlock()

	switch n {
	case mangos.OptionMaxRecvSize:
		if b, ok := v.(int); ok {
			l.maxRecvSize = b
			return nil
		}
		return mangos.ErrBadValue
	case OptionIpcSocketPermissions:
		if b, ok := v.(uint32); ok && b&uint32(os.ModePerm) == v {
			l.mode = b
			l.chmod = true
			return nil
		}
		if b, ok := v.(os.FileMode); ok && b&os.ModePerm == v {
			l.mode = uint32(b)
			l.chmod = true
			return nil
		}
		return mangos.ErrBadValue
	case OptionIpcSocketOwner:
		if b, ok := v.(int); ok {
			l.owner = b
			l.chown = true
			return nil
		}
		return mangos.ErrBadValue
	case OptionIpcSocketGroup:
		if b, ok := v.(int); ok {
			l.group = b
			l.chown = true
			return nil
		}
		return mangos.ErrBadValue
	}

	return mangos.ErrBadOption
}

// GetOption implements a stub PipeListener GetOption method.
func (l *listener) GetOption(n string) (interface{}, error) {
	l.lock.Lock()
	defer l.lock.Unlock()

	switch n {
	case mangos.OptionMaxRecvSize:
		return l.maxRecvSize, nil
	}
	return nil, mangos.ErrBadOption
}

func (l *listener) removeStaleIPC() {
	addr := l.addr.String()
	// if it's not a socket, then leave it alone!
	if st, err := os.Stat(addr); err != nil || st.Mode()&os.ModeType != os.ModeSocket {
		return
	}
	conn, err := net.DialTimeout("unix", l.addr.String(), 100*time.Millisecond)
	if err != nil && isSyscallError(err, syscall.ECONNREFUSED) {
		os.Remove(l.addr.String())
		return
	}
	if err == nil {
		conn.Close()
	}
}

type ipcTran int

// Scheme implements the Transport Scheme method.
func (ipcTran) Scheme() string {
	return "ipc"
}

// NewDialer implements the Transport NewDialer method.
func (t ipcTran) NewDialer(addr string, sock mangos.Socket) (transport.Dialer, error) {
	var err error
	d := &dialer{
		proto: sock.Info(),
		hs:    transport.NewConnHandshaker(),
	}

	if addr, err = transport.StripScheme(t, addr); err != nil {
		return nil, err
	}

	// ignoring the errors, because this cannot fail on POSIX systems;
	// the only error conditions are if the network is not "unix"
	d.addr, _ = net.ResolveUnixAddr("unix", addr)
	return d, nil
}

// NewListener implements the Transport NewListener method.
func (t ipcTran) NewListener(addr string, sock mangos.Socket) (transport.Listener, error) {
	var err error
	l := &listener{
		proto:  sock.Info(),
		closeq: make(chan struct{}),
		hs:     transport.NewConnHandshaker(),
		owner:  os.Geteuid(),
		group:  os.Getegid(),
	}

	if addr, err = transport.StripScheme(t, addr); err != nil {
		return nil, err
	}

	// ignoring the errors, as it cannot fail.
	l.addr, _ = net.ResolveUnixAddr("unix", addr)
	return l, nil
}

func isSyscallError(err error, code syscall.Errno) bool {
	opErr, ok := err.(*net.OpError)
	if !ok {
		return false
	}
	syscallErr, ok := opErr.Err.(*os.SyscallError)
	if !ok {
		return false
	}
	errno, ok := syscallErr.Err.(syscall.Errno)
	if !ok {
		return false
	}
	if errno == code {
		return true
	}
	return false
}
