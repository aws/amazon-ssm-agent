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

package test

import (
	"sync"
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol"
	"go.nanomsg.org/mangos/v3/transport"
)

// This file implements a mock transport, useful for testing.

// ListenerDialer does both Listen and Dial.  (That is, it is both
// a dialer and a listener at once.)
type mockCreator struct {
	pipeQ       chan MockPipe
	closeQ      chan struct{}
	errorQ      chan error
	proto       uint16
	deferClose  bool // sometimes we don't want close to really work yet
	closed      bool
	addr        string
	maxRecvSize int
	lock        sync.Mutex
}

// mockPipe implements a mocked transport.Pipe
type mockPipe struct {
	lProto     uint16
	rProto     uint16
	closeQ     chan struct{}
	recvQ      chan *mangos.Message
	sendQ      chan *mangos.Message
	recvErrQ   chan error
	sendErrQ   chan error
	deferClose bool
	closed     bool
	lock       sync.Mutex
	initOnce   sync.Once
}

func (mp *mockPipe) init() {
	mp.initOnce.Do(func() {
		mp.recvQ = make(chan *mangos.Message)
		mp.sendQ = make(chan *mangos.Message)
		mp.closeQ = make(chan struct{})
		mp.recvErrQ = make(chan error, 1)
		mp.sendErrQ = make(chan error, 1)
	})
}

func (mp *mockPipe) SendQ() <-chan *protocol.Message {
	mp.init()
	return mp.sendQ
}

func (mp *mockPipe) RecvQ() chan<- *protocol.Message {
	mp.init()
	return mp.recvQ
}

func (mp *mockPipe) InjectSendError(e error) {
	mp.init()
	select {
	case mp.sendErrQ <- e:
	default:
	}
}

func (mp *mockPipe) InjectRecvError(e error) {
	mp.init()
	select {
	case mp.recvErrQ <- e:
	default:
	}
}

func (mp *mockPipe) Send(m *mangos.Message) error {
	mp.init()
	select {
	case <-mp.closeQ:
		return mangos.ErrClosed
	case e := <-mp.sendErrQ:
		return e
	case mp.sendQ <- m:
		return nil
	}
}

func (mp *mockPipe) Recv() (*mangos.Message, error) {
	mp.init()
	select {
	case <-mp.closeQ:
		return nil, mangos.ErrClosed
	case e := <-mp.recvErrQ:
		return nil, e
	case m := <-mp.recvQ:
		return m, nil
	}
}

func (mp *mockPipe) GetOption(name string) (interface{}, error) {
	switch name {
	case mangos.OptionRemoteAddr, mangos.OptionLocalAddr:
		return "mock://mock", nil
	}
	return nil, mangos.ErrBadOption
}

func (mp *mockPipe) Close() error {
	mp.lock.Lock()
	defer mp.lock.Unlock()
	if !mp.closed {
		mp.closed = true
		if !mp.deferClose {
			select {
			case <-mp.closeQ:
			default:
				close(mp.closeQ)
			}
		}
	}
	return nil
}

func (mp *mockPipe) DeferClose(later bool) {
	mp.lock.Lock()
	defer mp.lock.Unlock()
	mp.deferClose = later
	if !later && mp.closed {
		select {
		case <-mp.closeQ:
		default:
			close(mp.closeQ)
		}
	}
}

func (mp *mockPipe) MockRecvMsg(d time.Duration) (*protocol.Message, error) {
	select {
	case <-time.After(d):
		return nil, mangos.ErrRecvTimeout
	case m := <-mp.sendQ:
		return m, nil
	case <-mp.closeQ:
		return nil, mangos.ErrClosed
	}
}

func (mp *mockPipe) MockSendMsg(m *protocol.Message, d time.Duration) error {
	select {
	case <-time.After(d):
		return mangos.ErrSendTimeout
	case mp.recvQ <- m:
		return nil
	case <-mp.closeQ:
		return mangos.ErrClosed
	}
}

// NewMockPipe creates a mocked transport pipe.
func NewMockPipe(lProto, rProto uint16) MockPipe {
	mp := &mockPipe{
		lProto: lProto,
		rProto: rProto,
	}
	mp.init()
	return mp
}

// MockPipe is a mocked transport pipe.
type MockPipe interface {
	// SendQ obtains the send queue.  Test code can read from this
	// to get messages sent by the socket.
	SendQ() <-chan *protocol.Message

	// RecvQ obtains the recv queue.  Test code can write to this
	// to send message to the socket.
	RecvQ() chan<- *protocol.Message

	// InjectSendError is used to inject an error that will be seen
	// by the next Send() operation.
	InjectSendError(error)

	// InjectRecvError is used to inject an error that will be seen
	// by the next Recv() operation.
	InjectRecvError(error)

	// DeferClose defers closing.
	DeferClose(deferring bool)

	// MockSendMsg lets us inject a message into the queue.
	MockSendMsg(*protocol.Message, time.Duration) error

	// MockRecvMsg lets us attempt to receive a message.
	MockRecvMsg(time.Duration) (*protocol.Message, error)

	transport.Pipe
}

// MockCreator is an abstraction of both dialers and listeners, which
// allows us to test various transport failure conditions.
type MockCreator interface {
	// NewPipe creates a Pipe, but does not add it.  The pipe will
	// use the assigned peer protocol.
	NewPipe(peer uint16) MockPipe

	// AddPipe adds the given pipe, returning an error if there is
	// no room to do so in the pipeQ.
	AddPipe(pipe MockPipe) error

	// DeferClose is used to defer close operations.  If Close()
	// is called, and deferring is false, then the close
	// will happen immediately.
	DeferClose(deferring bool)

	// Close is used to close the creator.
	Close() error

	// These are methods from transport, for Dialer and Listener.

	// Dial simulates dialing
	Dial() (transport.Pipe, error)

	// Listen simulates listening
	Listen() error

	// Accept simulates accepting.
	Accept() (transport.Pipe, error)

	// GetOption simulates getting an option.
	GetOption(string) (interface{}, error)

	// SetOption simulates setting an option.
	SetOption(string, interface{}) error

	// Address returns the address.
	Address() string

	// InjectError is used to inject a single error.
	InjectError(error)
}

func (mc *mockCreator) InjectError(e error) {
	select {
	case mc.errorQ <- e:
	default:
	}
}

func (mc *mockCreator) getPipe() (transport.Pipe, error) {
	select {
	case mp := <-mc.pipeQ:
		return mp, nil
	case <-mc.closeQ:
		return nil, mangos.ErrClosed
	case e := <-mc.errorQ:
		return nil, e
	}
}

func (mc *mockCreator) Dial() (transport.Pipe, error) {
	return mc.getPipe()
}

func (mc *mockCreator) Accept() (transport.Pipe, error) {
	return mc.getPipe()
}

func (mc *mockCreator) Listen() error {
	select {
	case e := <-mc.errorQ:
		return e
	case <-mc.closeQ:
		return mangos.ErrClosed
	default:
		return nil
	}
}

func (mc *mockCreator) SetOption(name string, val interface{}) error {
	switch name {
	case "mockError":
		return val.(error)
	case mangos.OptionMaxRecvSize:
		if v, ok := val.(int); ok && v >= 0 {
			// These are magical values used for test validation.
			switch v {
			case 1001:
				return mangos.ErrBadValue
			case 1002:
				return mangos.ErrBadOption
			}
			mc.maxRecvSize = v
			return nil
		}
		return mangos.ErrBadValue
	}
	return mangos.ErrBadOption
}

func (mc *mockCreator) GetOption(name string) (interface{}, error) {
	switch name {
	case "mock":
		return mc, nil
	case "mockError":
		return nil, mangos.ErrProtoState
	case mangos.OptionMaxRecvSize:
		return mc.maxRecvSize, nil
	}
	return nil, mangos.ErrBadOption
}

// NewPipe just returns a ready pipe with the local peer set up.
func (mc *mockCreator) NewPipe(peer uint16) MockPipe {
	return NewMockPipe(mc.proto, peer)
}

// AddPipe adds a pipe.
func (mc *mockCreator) AddPipe(mp MockPipe) error {
	select {
	case mc.pipeQ <- mp:
		return nil
	default:
	}
	return mangos.ErrConnRefused
}

// DeferClose is used to hold off the close to simulate a the endpoint
// still creating pipes even after close.  It doesn't actually do a close,
// but if this is disabled, and Close() was called previously, then the
// close will happen immediately.
func (mc *mockCreator) DeferClose(b bool) {
	mc.lock.Lock()
	defer mc.lock.Unlock()
	mc.deferClose = b
	if mc.closed && !mc.deferClose {
		select {
		case <-mc.closeQ:
		default:
			close(mc.closeQ)
		}
	}
}

// Close closes the endpoint, but only if SkipClose is false.
func (mc *mockCreator) Close() error {
	mc.lock.Lock()
	defer mc.lock.Unlock()
	mc.closed = true
	if !mc.deferClose {
		select {
		case <-mc.closeQ:
		default:
			close(mc.closeQ)
		}
	}
	return nil
}

func (mc *mockCreator) Address() string {
	return mc.addr
}

type mockTransport struct{}

func (mockTransport) Scheme() string {
	return "mock"
}

func (mt mockTransport) newCreator(addr string, sock mangos.Socket) (MockCreator, error) {
	if _, err := transport.StripScheme(mt, addr); err != nil {
		return nil, err
	}
	mc := &mockCreator{
		proto:  sock.Info().Self,
		pipeQ:  make(chan MockPipe, 1),
		closeQ: make(chan struct{}),
		errorQ: make(chan error, 1),
		addr:   addr,
	}
	return mc, nil
}

func (mt mockTransport) NewListener(addr string, sock mangos.Socket) (transport.Listener, error) {
	return mt.newCreator(addr, sock)
}

func (mt mockTransport) NewDialer(addr string, sock mangos.Socket) (transport.Dialer, error) {
	return mt.newCreator(addr, sock)
}

// AddMockTransport registers the mock transport.
func AddMockTransport() {
	transport.RegisterTransport(mockTransport{})
}

// GetMockListener returns a listener that creates mock pipes.
func GetMockListener(t *testing.T, s mangos.Socket) (mangos.Listener, MockCreator) {
	AddMockTransport()
	l, e := s.NewListener("mock://mock", nil)
	MustSucceed(t, e)
	v, e := l.GetOption("mock")
	MustSucceed(t, e)
	ml, ok := v.(MockCreator)
	MustBeTrue(t, ok)
	return l, ml
}

// GetMockDialer returns a dialer that creates mock pipes.
func GetMockDialer(t *testing.T, s mangos.Socket) (mangos.Dialer, MockCreator) {
	AddMockTransport()
	d, e := s.NewDialer("mock://mock", nil)
	MustSucceed(t, e)
	v, e := d.GetOption("mock")
	MustSucceed(t, e)
	ml, ok := v.(MockCreator)
	MustBeTrue(t, ok)
	return d, ml
}

// MockAddPipe simulates adding a pipe.
func MockAddPipe(t *testing.T, s mangos.Socket, c MockCreator, p MockPipe) mangos.Pipe {
	var rv mangos.Pipe
	wg := sync.WaitGroup{}
	wg.Add(1)
	hook := s.SetPipeEventHook(func(ev mangos.PipeEvent, pipe mangos.Pipe) {
		switch ev {
		case mangos.PipeEventAttached:
			rv = pipe
			wg.Done()
		}
	})
	MustSucceed(t, c.AddPipe(p))
	wg.Wait()
	s.SetPipeEventHook(hook)
	return rv
}

// MockConnect simulates connecting a pipe.
func MockConnect(t *testing.T, s mangos.Socket) (MockPipe, mangos.Pipe) {
	var pipe mangos.Pipe
	wg := sync.WaitGroup{}
	wg.Add(1)

	l, c := GetMockListener(t, s)
	MustSucceed(t, l.Listen())

	hook := s.SetPipeEventHook(func(ev mangos.PipeEvent, p mangos.Pipe) {
		switch ev {
		case mangos.PipeEventAttached:
			pipe = p
			wg.Done()
		}
	})

	mp := c.NewPipe(s.Info().Peer)
	MustSucceed(t, c.AddPipe(mp))
	wg.Wait()
	s.SetPipeEventHook(hook)
	return mp, pipe
}

// MockMustSendMsg ensures that the pipe sends a message.
func MockMustSendMsg(t *testing.T, p MockPipe, m *mangos.Message, d time.Duration) {
	MustSucceed(t, p.MockSendMsg(m, d))
}

// MockMustSend ensures that the pipe sends a message with the body given.
func MockMustSend(t *testing.T, p MockPipe, data []byte, d time.Duration) {
	msg := mangos.NewMessage(0)
	msg.Body = append(msg.Body, data...)
	MockMustSendMsg(t, p, msg, d)
}

// MockMustSendStr ensures that the pipe sends a message with a payload
// containing the given string.
func MockMustSendStr(t *testing.T, p MockPipe, str string, d time.Duration) {
	msg := mangos.NewMessage(0)
	msg.Body = []byte(str)
	MockMustSendMsg(t, p, msg, d)
}

// MockMustRecvStr ensures that the pipe receives a message with the payload
// equal to the string.
func MockMustRecvStr(t *testing.T, p MockPipe, str string, d time.Duration) {
	m := mangos.NewMessage(0)
	m.Body = append(m.Body, []byte(str)...)
	msg, err := p.MockRecvMsg(d)
	MustSucceed(t, err)
	MustBeTrue(t, string(msg.Body) == str)
}

// AddrMock returns a generic address for mock sockets.
func AddrMock() string {
	return "mock://mock"
}
