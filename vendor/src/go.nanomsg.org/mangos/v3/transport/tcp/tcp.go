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

// Package tcp implements the TCP transport for mangos. To enable it simply
// import it.
package tcp

import (
	"context"
	"net"
	"sync"
	"time"

	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/transport"
)

const (
	// Transport is a transport.Transport for TCP.
	Transport = tcpTran(0)
)

func init() {
	transport.RegisterTransport(Transport)
}

type dialer struct {
	addr        string
	proto       transport.ProtocolInfo
	hs          transport.Handshaker
	maxRecvSize int
	d           net.Dialer
	lock        sync.Mutex
}

func (d *dialer) Dial() (_ transport.Pipe, err error) {
	conn, err := d.d.Dial("tcp", d.addr)
	if err != nil {
		return nil, err
	}

	d.lock.Lock()
	mrs := d.maxRecvSize
	d.lock.Unlock()
	p := transport.NewConnPipe(conn, d.proto, nil)
	p.SetMaxRecvSize(mrs)
	d.hs.Start(p)
	return d.hs.Wait()
}

func (d *dialer) SetOption(n string, v interface{}) error {
	switch n {

	case mangos.OptionMaxRecvSize:
		if b, ok := v.(int); ok {
			d.maxRecvSize = b
			return nil
		}
		return mangos.ErrBadValue
	case mangos.OptionKeepAliveTime:
		if b, ok := v.(time.Duration); ok {
			d.d.KeepAlive = b
			return nil
		}
		return mangos.ErrBadValue

	// The following options exist *only* for compatibility reasons.
	// Remove them from new code.

	case mangos.OptionKeepAlive:
		if b, ok := v.(bool); ok {
			if b {
				d.d.KeepAlive = 0 // Enable (default time)
			} else {
				d.d.KeepAlive = -1 // Disable
			}
			return nil
		}
		return mangos.ErrBadValue

	case mangos.OptionNoDelay:
		if _, ok := v.(bool); ok {
			return nil
		}
		return mangos.ErrBadValue
	}
	return mangos.ErrBadOption
}

func (d *dialer) GetOption(n string) (interface{}, error) {
	d.lock.Lock()
	defer d.lock.Unlock()
	switch n {
	case mangos.OptionMaxRecvSize:
		return d.maxRecvSize, nil
	case mangos.OptionKeepAliveTime:
		return d.d.KeepAlive, nil
	case mangos.OptionNoDelay:
		return true, nil
	case mangos.OptionKeepAlive:
		if d.d.KeepAlive >= 0 {
			return true, nil
		}
		return false, nil

	}
	return nil, mangos.ErrBadOption
}

type listener struct {
	addr        string
	bound       net.Addr
	proto       transport.ProtocolInfo
	l           net.Listener
	lc          net.ListenConfig
	maxRecvSize int
	handshaker  transport.Handshaker
	closeq      chan struct{}
	once        sync.Once
	lock        sync.Mutex
}

func (l *listener) Accept() (transport.Pipe, error) {

	if l.l == nil {
		return nil, mangos.ErrClosed
	}
	return l.handshaker.Wait()
}

func (l *listener) Listen() (err error) {
	select {
	case <-l.closeq:
		return mangos.ErrClosed
	default:
	}
	l.l, err = l.lc.Listen(context.Background(), "tcp", l.addr)
	if err != nil {
		return
	}
	l.bound = l.l.Addr()
	go func() {
		for {
			conn, err := l.l.Accept()
			if err != nil {
				select {
				case <-l.closeq:
					return
				default:
					// We probably should be checking
					// if this is a temporary error.
					// If we run out of files we will
					// spin hard here.
					time.Sleep(time.Millisecond)
					continue
				}
			}
			l.lock.Lock()
			mrs := l.maxRecvSize
			l.lock.Unlock()
			p := transport.NewConnPipe(conn, l.proto, nil)
			p.SetMaxRecvSize(mrs)
			l.handshaker.Start(p)
		}
	}()
	return
}

func (l *listener) Address() string {
	if b := l.bound; b != nil {
		return "tcp://" + b.String()
	}
	return "tcp://" + l.addr
}

func (l *listener) Close() error {
	l.once.Do(func() {
		close(l.closeq)
		if l.l != nil {
			_ = l.l.Close()
		}
		l.handshaker.Close()
	})
	return nil
}

func (l *listener) SetOption(n string, v interface{}) error {
	switch n {
	case mangos.OptionMaxRecvSize:
		if b, ok := v.(int); ok {
			l.maxRecvSize = b
			return nil
		}
		return mangos.ErrBadValue
	case mangos.OptionKeepAliveTime:
		if b, ok := v.(time.Duration); ok {
			l.lc.KeepAlive = b
			return nil
		}
		return mangos.ErrBadValue

	// The following options exist *only* for compatibility reasons.
	// Remove them from new code.

	case mangos.OptionKeepAlive:
		if b, ok := v.(bool); ok {
			if b {
				l.lc.KeepAlive = 0 // Enable (default time)
			} else {
				l.lc.KeepAlive = -1 // Disable
			}
			return nil
		}
		return mangos.ErrBadValue

	case mangos.OptionNoDelay:
		if _, ok := v.(bool); ok {
			return nil
		}
		return mangos.ErrBadValue
	}
	return mangos.ErrBadOption
}

func (l *listener) GetOption(n string) (interface{}, error) {
	switch n {
	case mangos.OptionMaxRecvSize:
		return l.maxRecvSize, nil
	case mangos.OptionKeepAliveTime:
		return l.lc.KeepAlive, nil
	case mangos.OptionNoDelay:
		return true, nil
	case mangos.OptionKeepAlive:
		if l.lc.KeepAlive >= 0 {
			return true, nil
		}
		return false, nil
	}
	return nil, mangos.ErrBadOption
}

type tcpTran int

func (t tcpTran) Scheme() string {
	return "tcp"
}

func (t tcpTran) NewDialer(addr string, sock mangos.Socket) (transport.Dialer, error) {
	var err error
	if addr, err = transport.StripScheme(t, addr); err != nil {
		return nil, err
	}

	// check to ensure the provided addr resolves correctly.
	if _, err = transport.ResolveTCPAddr(addr); err != nil {
		return nil, err
	}

	d := &dialer{
		addr:  addr,
		proto: sock.Info(),
		hs:    transport.NewConnHandshaker(),
	}

	return d, nil
}

func (t tcpTran) NewListener(addr string, sock mangos.Socket) (transport.Listener, error) {
	var err error
	l := &listener{
		proto:  sock.Info(),
		closeq: make(chan struct{}),
	}

	if addr, err = transport.StripScheme(t, addr); err != nil {
		return nil, err
	}

	l.addr = addr

	l.handshaker = transport.NewConnHandshaker()
	return l, nil
}
