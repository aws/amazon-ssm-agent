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

// Package tlstcp implements the TLS over TCP transport for mangos.
// To enable it simply import it.
package tlstcp

import (
	"context"
	"crypto/tls"
	"net"
	"sync"
	"time"

	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/transport"
)

// Transport is a transport.Transport for TLS over TCP.
const Transport = tlsTran(0)

func init() {
	transport.RegisterTransport(Transport)
}

type dialer struct {
	addr        string
	proto       transport.ProtocolInfo
	hs          transport.Handshaker
	d           *net.Dialer
	config      *tls.Config
	maxRecvSize int
	lock        sync.Mutex
}

func (d *dialer) Dial() (transport.Pipe, error) {

	d.lock.Lock()
	config := d.config
	maxRecvSize := d.maxRecvSize
	d.lock.Unlock()

	conn, err := tls.DialWithDialer(d.d, "tcp", d.addr, config)
	if err != nil {
		return nil, err
	}
	opts := make(map[string]interface{})
	opts[mangos.OptionTLSConnState] = conn.ConnectionState()
	p := transport.NewConnPipe(conn, d.proto, opts)
	p.SetMaxRecvSize(maxRecvSize)
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
	case mangos.OptionTLSConfig:
		if b, ok := v.(*tls.Config); ok {
			d.config = b
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

	// We don't support disabling Nagle anymore.
	case mangos.OptionNoDelay:
		if _, ok := v.(bool); ok {
			return nil
		}
		return mangos.ErrBadValue
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
	}
	return mangos.ErrBadOption
}

func (d *dialer) GetOption(n string) (interface{}, error) {
	d.lock.Lock()
	defer d.lock.Unlock()
	switch n {
	case mangos.OptionMaxRecvSize:
		return d.maxRecvSize, nil
	case mangos.OptionNoDelay:
		return true, nil // Compatibility only, always true
	case mangos.OptionTLSConfig:
		return d.config, nil
	case mangos.OptionKeepAlive:
		if d.d.KeepAlive >= 0 {
			return true, nil
		}
		return false, nil
	case mangos.OptionKeepAliveTime:
		return d.d.KeepAlive, nil
	}
	return nil, mangos.ErrBadOption
}

type listener struct {
	addr        string
	bound       net.Addr
	lc          net.ListenConfig
	l           net.Listener
	maxRecvSize int
	proto       transport.ProtocolInfo
	config      *tls.Config
	hs          transport.Handshaker
	closeQ      chan struct{}
	once        sync.Once
	lock        sync.Mutex
}

func (l *listener) Listen() error {
	var err error
	select {
	case <-l.closeQ:
		return mangos.ErrClosed
	default:
	}
	l.lock.Lock()
	config := l.config
	if config == nil {
		return mangos.ErrTLSNoConfig
	}
	if config.Certificates == nil || len(config.Certificates) == 0 {
		l.lock.Unlock()
		return mangos.ErrTLSNoCert
	}

	inner, err := l.lc.Listen(context.Background(), "tcp", l.addr)
	if err != nil {
		l.lock.Unlock()
		return err
	}
	l.l = tls.NewListener(inner, config)
	l.bound = l.l.Addr()
	l.lock.Unlock()

	go func() {
		for {
			conn, err := l.l.Accept()
			if err != nil {
				select {
				case <-l.closeQ:
					return
				default:
					time.Sleep(time.Millisecond)
					continue
				}
			}

			tc := conn.(*tls.Conn)
			opts := make(map[string]interface{})
			l.lock.Lock()
			maxRecvSize := l.maxRecvSize
			l.lock.Unlock()
			opts[mangos.OptionTLSConnState] = tc.ConnectionState()
			p := transport.NewConnPipe(conn, l.proto, opts)
			p.SetMaxRecvSize(maxRecvSize)

			l.hs.Start(p)
		}
	}()

	return nil
}

func (l *listener) Address() string {
	if b := l.bound; b != nil {
		return "tls+tcp://" + b.String()
	}
	return "tls+tcp://" + l.addr
}

func (l *listener) Accept() (transport.Pipe, error) {
	if l.l == nil {
		return nil, mangos.ErrClosed
	}
	return l.hs.Wait()
}

func (l *listener) Close() error {
	l.once.Do(func() {
		if l.l != nil {
			_ = l.l.Close()
		}
		l.hs.Close()
		close(l.closeQ)
	})
	return nil
}

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
	case mangos.OptionTLSConfig:
		if b, ok := v.(*tls.Config); ok {
			l.config = b
			return nil
		}
		return mangos.ErrBadValue
	case mangos.OptionKeepAliveTime:
		if b, ok := v.(time.Duration); ok {
			l.lc.KeepAlive = b
			return nil
		}
		return mangos.ErrBadValue

		// Legacy stuff follows
	case mangos.OptionNoDelay:
		if _, ok := v.(bool); ok {
			return nil
		}
		return mangos.ErrBadValue
	case mangos.OptionKeepAlive:
		if b, ok := v.(bool); ok {
			if b {
				l.lc.KeepAlive = 0
			} else {
				l.lc.KeepAlive = -1
			}
			return nil
		}
		return mangos.ErrBadValue

	}
	return mangos.ErrBadOption
}

func (l *listener) GetOption(n string) (interface{}, error) {
	l.lock.Lock()
	defer l.lock.Unlock()
	switch n {
	case mangos.OptionMaxRecvSize:
		return l.maxRecvSize, nil
	case mangos.OptionTLSConfig:
		return l.config, nil
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

type tlsTran int

func (t tlsTran) Scheme() string {
	return "tls+tcp"
}

func (t tlsTran) NewDialer(addr string, sock mangos.Socket) (transport.Dialer, error) {
	var err error

	if addr, err = transport.StripScheme(t, addr); err != nil {
		return nil, err
	}

	// check to ensure the provided addr resolves correctly.
	if _, err = transport.ResolveTCPAddr(addr); err != nil {
		return nil, err
	}

	d := &dialer{
		proto: sock.Info(),
		addr:  addr,
		hs:    transport.NewConnHandshaker(),
		d:     &net.Dialer{},
	}
	return d, nil
}

// NewListener implements the Transport NewListener method.
func (t tlsTran) NewListener(addr string, sock mangos.Socket) (transport.Listener, error) {
	l := &listener{
		proto:  sock.Info(),
		closeQ: make(chan struct{}),
	}

	var err error
	if addr, err = transport.StripScheme(t, addr); err != nil {
		return nil, err
	}
	l.addr = addr
	l.hs = transport.NewConnHandshaker()

	return l, nil
}
