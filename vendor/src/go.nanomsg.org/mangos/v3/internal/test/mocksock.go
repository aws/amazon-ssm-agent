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
	"time"

	"go.nanomsg.org/mangos/v3/protocol"
)

// This file implements a mock socket, useful for testing.

// mockSock is a dumb pass through mock of a socket.
type mockSock struct {
	recvQ      chan *protocol.Message
	sendQ      chan *protocol.Message
	closeQ     chan struct{}
	proto      uint16
	peer       uint16
	name       string
	peerName   string
	sendExpire time.Duration
	recvExpire time.Duration
	once       sync.Once
	lock       sync.Mutex
	raw        interface{}
	rawError   error
}

func (s *mockSock) Close() error {
	select {
	case <-s.closeQ:
		return protocol.ErrClosed
	default:
		s.once.Do(func() {
			close(s.closeQ)
		})
		return nil
	}
}

func (s *mockSock) SendMsg(m *protocol.Message) error {
	var timerQ <-chan time.Time
	s.lock.Lock()
	if exp := s.sendExpire; exp > 0 {
		timerQ = time.After(exp)
	}
	s.lock.Unlock()
	select {
	case s.sendQ <- m:
		return nil
	case <-s.closeQ:
		return protocol.ErrClosed
	case <-timerQ:
		return protocol.ErrSendTimeout
	}
}

func (s *mockSock) RecvMsg() (*protocol.Message, error) {
	var timerQ <-chan time.Time
	s.lock.Lock()
	if exp := s.recvExpire; exp > 0 {
		timerQ = time.After(exp)
	}
	s.lock.Unlock()
	select {
	case m := <-s.recvQ:
		return m, nil
	case <-s.closeQ:
		return nil, protocol.ErrClosed
	case <-timerQ:
		return nil, protocol.ErrRecvTimeout
	}
}

func (s *mockSock) GetOption(name string) (interface{}, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	switch name {
	case protocol.OptionRecvDeadline:
		return s.recvExpire, nil
	case protocol.OptionSendDeadline:
		return s.sendExpire, nil
	case protocol.OptionRaw:
		return s.raw, s.rawError
	}
	return nil, protocol.ErrBadOption
}

func (s *mockSock) SetOption(name string, val interface{}) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	switch name {
	case protocol.OptionRecvDeadline:
		if d, ok := val.(time.Duration); ok {
			s.recvExpire = d
			return nil
		}
		return protocol.ErrBadValue
	case protocol.OptionSendDeadline:
		if d, ok := val.(time.Duration); ok {
			s.sendExpire = d
			return nil
		}
		return protocol.ErrBadValue
	}
	return protocol.ErrBadOption
}

func (s *mockSock) Info() protocol.Info {
	return protocol.Info{
		Self:     s.proto,
		Peer:     s.peer,
		SelfName: s.name,
		PeerName: s.peerName,
	}
}

type mockSockPipe struct {
	p      protocol.Pipe
	closeQ chan struct{}
	once   sync.Once
}

func (p *mockSockPipe) close() {
	p.once.Do(func() {
		_ = p.p.Close()
		close(p.closeQ)
	})
}

func (s *mockSock) sender(p *mockSockPipe) {
	for {
		select {
		case m := <-s.sendQ:
			if p.p.SendMsg(m) != nil {
				p.close()
				return
			}
		case <-p.closeQ:
			return
		}
	}
}

func (s *mockSock) receiver(p *mockSockPipe) {
	for {
		m := p.p.RecvMsg()
		if m == nil {
			p.close()
			return
		}
		select {
		case s.recvQ <- m:
		case <-p.closeQ:
			return
		}
	}
}

func (s *mockSock) AddPipe(pp protocol.Pipe) error {
	p := &mockSockPipe{
		p:      pp,
		closeQ: make(chan struct{}),
	}
	pp.SetPrivate(p)
	go s.sender(p)
	go s.receiver(p)
	return nil
}

func (*mockSock) RemovePipe(pp protocol.Pipe) {
	p := pp.GetPrivate().(*mockSockPipe)
	p.close()
}

func (*mockSock) OpenContext() (protocol.Context, error) {
	return nil, protocol.ErrProtoOp
}

// GetMockSocket returns a mock socket.
func GetMockSocket() protocol.Socket {
	return protocol.MakeSocket(&mockSock{
		recvQ:    make(chan *protocol.Message, 1),
		sendQ:    make(chan *protocol.Message, 1),
		closeQ:   make(chan struct{}),
		proto:    1,
		peer:     1,
		name:     "mockSock",
		peerName: "mockSock",
	})
}

// GetMockSocketRaw is an extended form to get a mocked socket, with particular
// properties set (including the response to GetOption() for Raw.)
func GetMockSocketRaw(proto, peer uint16, name, peerName string, raw interface{}, err error) protocol.Socket {
	return protocol.MakeSocket(&mockSock{
		recvQ:    make(chan *protocol.Message, 1),
		sendQ:    make(chan *protocol.Message, 1),
		closeQ:   make(chan struct{}),
		proto:    proto,
		peer:     peer,
		name:     name,
		peerName: peerName,
		raw:      raw,
		rawError: err,
	})
}

// NewMockSocket returns a mock socket, and nil.
func NewMockSocket() (protocol.Socket, error) {
	return GetMockSocket(), nil
}

// GetMockSocketEx returns a socket for a specific protocol.
func GetMockSocketEx(proto uint16, name string) protocol.Socket {
	return protocol.MakeSocket(&mockSock{
		recvQ:    make(chan *protocol.Message, 1),
		sendQ:    make(chan *protocol.Message, 1),
		closeQ:   make(chan struct{}),
		proto:    proto,
		peer:     proto,
		name:     name,
		peerName: name,
	})
}
