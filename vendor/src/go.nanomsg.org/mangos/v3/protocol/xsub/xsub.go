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

// Package xsub implements the raw SUB protocol. This protocol simply
// passes through all messages received, and does not filter them.
package xsub

import (
	"sync"
	"time"

	"go.nanomsg.org/mangos/v3/protocol"
)

// Protocol identity information.
const (
	Self     = protocol.ProtoSub
	Peer     = protocol.ProtoPub
	SelfName = "sub"
	PeerName = "pub"
)

type pipe struct {
	p protocol.Pipe
	s *socket
}

type socket struct {
	closed     bool
	closeQ     chan struct{}
	recvQLen   int
	recvExpire time.Duration
	recvQ      chan *protocol.Message
	sizeQ      chan struct{}
	sync.Mutex
}

var (
	nilQ <-chan time.Time
)

const defaultQLen = 128

func (s *socket) SendMsg(*protocol.Message) error {
	return protocol.ErrProtoOp
}

func (s *socket) RecvMsg() (*protocol.Message, error) {
	// For now this uses a simple unified queue for the entire
	// socket.  Later we can look at moving this to priority queues
	// based on socket pipes.
	timeQ := nilQ
	for {
		s.Lock()
		if s.recvExpire > 0 {
			timeQ = time.After(s.recvExpire)
		}
		closeQ := s.closeQ
		sizeQ := s.sizeQ
		recvQ := s.recvQ
		s.Unlock()
		select {
		case <-closeQ:
			return nil, protocol.ErrClosed
		case <-timeQ:
			return nil, protocol.ErrRecvTimeout
		case <-sizeQ:
			continue
		case m := <-recvQ:
			return m, nil
		}
	}
}

func (s *socket) SetOption(name string, value interface{}) error {
	switch name {

	case protocol.OptionRecvDeadline:
		if v, ok := value.(time.Duration); ok {
			s.Lock()
			s.recvExpire = v
			s.Unlock()
			return nil
		}
		return protocol.ErrBadValue

	case protocol.OptionReadQLen:
		if v, ok := value.(int); ok && v >= 0 {
			recvQ := make(chan *protocol.Message, v)
			sizeQ := make(chan struct{})
			s.Lock()
			s.recvQ = recvQ
			sizeQ, s.sizeQ = s.sizeQ, sizeQ
			s.recvQLen = v
			s.Unlock()
			close(sizeQ)

			// This leaks a few messages.  But it doesn't really
			// matter.  Resizing the queue tosses the messages.
			return nil
		}
		return protocol.ErrBadValue
	}

	return protocol.ErrBadOption
}

func (s *socket) GetOption(option string) (interface{}, error) {
	switch option {
	case protocol.OptionRaw:
		return true, nil
	case protocol.OptionRecvDeadline:
		s.Lock()
		v := s.recvExpire
		s.Unlock()
		return v, nil
	case protocol.OptionReadQLen:
		s.Lock()
		v := s.recvQLen
		s.Unlock()
		return v, nil
	}

	return nil, protocol.ErrBadOption
}

func (s *socket) AddPipe(pp protocol.Pipe) error {
	p := &pipe{
		p: pp,
		s: s,
	}
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return protocol.ErrClosed
	}
	go p.receiver()
	return nil
}

func (s *socket) RemovePipe(protocol.Pipe) {
}

func (s *socket) OpenContext() (protocol.Context, error) {
	return nil, protocol.ErrProtoOp
}

func (*socket) Info() protocol.Info {
	return protocol.Info{
		Self:     Self,
		Peer:     Peer,
		SelfName: SelfName,
		PeerName: PeerName,
	}
}

func (s *socket) Close() error {
	s.Lock()
	if s.closed {
		s.Unlock()
		return protocol.ErrClosed
	}
	s.closed = true
	s.Unlock()
	close(s.closeQ)
	return nil
}

func (p *pipe) receiver() {
	s := p.s
	for {
		m := p.p.RecvMsg()
		if m == nil {
			break
		}

		s.Lock()
		recvQ := s.recvQ
		s.Unlock()

		// No need to test for resizing or close here, because we
		// never block anyway.

		select {
		case recvQ <- m:
		default:
			m.Free()
		}
	}
	p.close()
}

func (p *pipe) close() {
	_ = p.p.Close()
}

// NewProtocol returns a new protocol implementation.
func NewProtocol() protocol.Protocol {
	s := &socket{
		closeQ:   make(chan struct{}),
		sizeQ:    make(chan struct{}),
		recvQ:    make(chan *protocol.Message, defaultQLen),
		recvQLen: defaultQLen,
	}
	return s
}

// NewSocket allocates a raw Socket using the PULL protocol.
func NewSocket() (protocol.Socket, error) {
	return protocol.MakeSocket(NewProtocol()), nil
}
