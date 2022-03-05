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

// Package xpair implements the PAIR protocol. This is a simple 1:1
// messaging pattern.  Only one peer can be connected at a time.
package xpair

import (
	"sync"
	"time"

	"go.nanomsg.org/mangos/v3/protocol"
)

// Protocol identity information.
const (
	Self     = protocol.ProtoPair
	Peer     = protocol.ProtoPair
	SelfName = "pair"
	PeerName = "pair"
)

type pipe struct {
	p      protocol.Pipe
	s      *socket
	closeQ chan struct{}
}

type socket struct {
	closed     bool
	closeQ     chan struct{}
	sizeQ      chan struct{}
	peer       *pipe
	recvQLen   int
	sendQLen   int
	recvExpire time.Duration
	sendExpire time.Duration
	bestEffort bool
	recvQ      chan *protocol.Message
	sendQ      chan *protocol.Message
	sync.Mutex
}

var (
	nilQ    <-chan time.Time
	closedQ chan time.Time
)

func init() {
	closedQ = make(chan time.Time)
	close(closedQ)
}

const defaultQLen = 128

func (s *socket) SendMsg(m *protocol.Message) error {
	timeQ := nilQ
	s.Lock()
	if s.closed {
		s.Unlock()
		return protocol.ErrClosed
	}
	if s.bestEffort {
		timeQ = closedQ
	} else if s.sendExpire > 0 {
		timeQ = time.After(s.sendExpire)
	}
	sizeQ := s.sizeQ
	sendQ := s.sendQ
	closeQ := s.closeQ
	s.Unlock()

	select {
	case <-closeQ:
		return protocol.ErrClosed
	case <-timeQ:
		if timeQ == closedQ {
			m.Free()
			return nil
		}
		return protocol.ErrSendTimeout

	case <-sizeQ:
		m.Free()
		return nil

	case sendQ <- m:
		return nil
	}
}

func (s *socket) RecvMsg() (*protocol.Message, error) {
	for {
		timeQ := nilQ
		s.Lock()
		if s.recvExpire > 0 {
			timeQ = time.After(s.recvExpire)
		}
		closeQ := s.closeQ
		recvQ := s.recvQ
		sizeQ := s.sizeQ
		s.Unlock()
		select {
		case <-closeQ:
			return nil, protocol.ErrClosed
		case <-timeQ:
			return nil, protocol.ErrRecvTimeout
		case m := <-recvQ:
			return m, nil
		case <-sizeQ:
		}
	}
}

func (s *socket) SetOption(name string, value interface{}) error {
	switch name {

	case protocol.OptionBestEffort:
		if v, ok := value.(bool); ok {
			s.Lock()
			s.bestEffort = v
			s.Unlock()
			return nil
		}
		return protocol.ErrBadValue

	case protocol.OptionRecvDeadline:
		if v, ok := value.(time.Duration); ok {
			s.Lock()
			s.recvExpire = v
			s.Unlock()
			return nil
		}
		return protocol.ErrBadValue

	case protocol.OptionSendDeadline:
		if v, ok := value.(time.Duration); ok {
			s.Lock()
			s.sendExpire = v
			s.Unlock()
			return nil
		}
		return protocol.ErrBadValue

	case protocol.OptionReadQLen:
		if v, ok := value.(int); ok && v >= 0 {
			recvQ := make(chan *protocol.Message, v)
			sizeQ := make(chan struct{})
			s.Lock()
			s.recvQLen = v
			s.recvQ = recvQ
			sizeQ, s.sizeQ = s.sizeQ, sizeQ
			s.Unlock()
			close(sizeQ)

			return nil
		}
		return protocol.ErrBadValue

	case protocol.OptionWriteQLen:
		if v, ok := value.(int); ok && v >= 0 {
			sendQ := make(chan *protocol.Message, v)
			sizeQ := make(chan struct{})
			s.Lock()
			s.sendQLen = v
			s.sendQ = sendQ
			sizeQ, s.sizeQ = s.sizeQ, sizeQ
			s.Unlock()
			close(sizeQ)

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
	case protocol.OptionBestEffort:
		s.Lock()
		v := s.bestEffort
		s.Unlock()
		return v, nil
	case protocol.OptionRecvDeadline:
		s.Lock()
		v := s.recvExpire
		s.Unlock()
		return v, nil
	case protocol.OptionSendDeadline:
		s.Lock()
		v := s.sendExpire
		s.Unlock()
		return v, nil
	case protocol.OptionReadQLen:
		s.Lock()
		v := s.recvQLen
		s.Unlock()
		return v, nil
	case protocol.OptionWriteQLen:
		s.Lock()
		v := s.sendQLen
		s.Unlock()
		return v, nil
	}

	return nil, protocol.ErrBadOption
}

func (s *socket) AddPipe(pp protocol.Pipe) error {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return protocol.ErrClosed
	}
	if s.peer != nil {
		return protocol.ErrProtoState
	}
	p := &pipe{
		p:      pp,
		s:      s,
		closeQ: make(chan struct{}),
	}
	s.peer = p
	go p.receiver()
	go p.sender()
	return nil
}

func (s *socket) RemovePipe(pp protocol.Pipe) {
	s.Lock()
	if p := s.peer; p != nil && pp == p.p {
		s.peer = nil
		close(p.closeQ)
	}
	s.Unlock()
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
outer:
	for {
		m := p.p.RecvMsg()
		if m == nil {
			break
		}

		s.Lock()
		recvQ := s.recvQ
		sizeQ := s.sizeQ
		s.Unlock()

		select {
		case recvQ <- m:
		case <-sizeQ:
			m.Free()
		case <-p.closeQ:
			m.Free()
			break outer
		}
	}
	p.close()
}

func (p *pipe) sender() {
	s := p.s
outer:
	for {
		s.Lock()
		sendQ := s.sendQ
		sizeQ := s.sizeQ
		s.Unlock()

		select {
		case m := <-sendQ:
			if err := p.p.SendMsg(m); err != nil {
				m.Free()
				break outer
			}

		case <-sizeQ:

		case <-p.closeQ:
			break outer
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
		sendQ:    make(chan *protocol.Message, defaultQLen),
		recvQLen: defaultQLen,
		sendQLen: defaultQLen,
	}
	return s
}

// NewSocket allocates a raw Socket using the PULL protocol.
func NewSocket() (protocol.Socket, error) {
	return protocol.MakeSocket(NewProtocol()), nil
}
