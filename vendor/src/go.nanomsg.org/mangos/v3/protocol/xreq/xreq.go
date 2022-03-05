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

// Package xreq implements the raw REQ protocol, which is the request side of
// the request/response pattern.  (REP is the response.)
package xreq

import (
	"sync"
	"time"

	"go.nanomsg.org/mangos/v3/protocol"
)

// Protocol identity information.
const (
	Self     = protocol.ProtoReq
	Peer     = protocol.ProtoRep
	SelfName = "req"
	PeerName = "rep"
)

type pipe struct {
	p      protocol.Pipe
	s      *socket
	closeQ chan struct{}
}

type socket struct {
	closed     bool
	recvQ      chan *protocol.Message
	sendQ      chan *protocol.Message
	closeQ     chan struct{}
	sizeQ      chan struct{}
	recvExpire time.Duration
	sendExpire time.Duration
	sendQLen   int
	recvQLen   int
	bestEffort bool
	sync.Mutex
}

var (
	nilQ    <-chan time.Time
	closedQ chan time.Time
)

const defaultQLen = 128

func init() {
	closedQ = make(chan time.Time)
	close(closedQ)
}

// SendMsg implements sending a message.  The message must come with
// its headers already prepared.  This will be at a minimum the request
// ID at the end of the header, plus any leading backtrace information
// coming from a paired REP socket.
func (s *socket) SendMsg(m *protocol.Message) error {
	s.Lock()
	bestEffort := s.bestEffort
	timeQ := nilQ
	if bestEffort {
		timeQ = closedQ
	} else if s.sendExpire > 0 {
		timeQ = time.After(s.sendExpire)
	}
	sendQ := s.sendQ
	sizeQ := s.sizeQ
	closeQ := s.closeQ
	s.Unlock()

	select {
	case sendQ <- m:
		return nil
	case <-sizeQ:
		m.Free()
		return nil
	case <-closeQ:
		return protocol.ErrClosed
	case <-timeQ:
		if bestEffort {
			m.Free()
			return nil
		}
		return protocol.ErrSendTimeout
	}
}

func (s *socket) RecvMsg() (*protocol.Message, error) {
	for {
		timeQ := nilQ
		s.Lock()
		if s.recvExpire > 0 {
			timeQ = time.After(s.recvExpire)
		}
		sizeQ := s.sizeQ
		recvQ := s.recvQ
		closeQ := s.closeQ
		s.Unlock()
		select {
		case <-closeQ:
			return nil, protocol.ErrClosed
		case <-timeQ:
			return nil, protocol.ErrRecvTimeout
		case m := <-recvQ:
			return m, nil
		case <-sizeQ:
			continue
		}
	}
}

func (p *pipe) receiver() {
	s := p.s
outer:
	for {
		m := p.p.RecvMsg()
		if m == nil {
			break
		}

		if len(m.Body) < 4 {
			m.Free()
			continue
		}

		m.Header = m.Body[:4]
		m.Body = m.Body[4:]

		s.Lock()
		recvQ := s.recvQ
		sizeQ := s.sizeQ
		s.Unlock()

		select {
		case recvQ <- m:
			continue
		case <-sizeQ: // resize discards
			m.Free()
			continue
		case <-p.closeQ:
			m.Free()
			break outer
		}
	}
	p.close()
}

// This is a puller, and doesn't permit for priorities.  We might want
// to refactor this to use a push based scheme later.
func (p *pipe) sender() {
	s := p.s
outer:
	for {
		s.Lock()
		sendQ := s.sendQ
		sizeQ := s.sizeQ
		s.Unlock()

		var m *protocol.Message
		select {
		case m = <-sendQ:
		case <-sizeQ:
			continue
		case <-p.closeQ:
			break outer
		}

		if e := p.p.SendMsg(m); e != nil {
			break
		}
	}
	p.close()
}

func (p *pipe) close() {
	_ = p.p.Close()
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

	case protocol.OptionSendDeadline:
		if v, ok := value.(time.Duration); ok {
			s.Lock()
			s.sendExpire = v
			s.Unlock()
			return nil
		}
		return protocol.ErrBadValue

	case protocol.OptionBestEffort:
		if v, ok := value.(bool); ok {
			s.Lock()
			s.bestEffort = v
			s.Unlock()
			return nil
		}
		return protocol.ErrBadValue

	case protocol.OptionWriteQLen:
		if v, ok := value.(int); ok && v >= 0 {

			newQ := make(chan *protocol.Message, v)
			sizeQ := make(chan struct{})
			s.Lock()
			s.sendQLen = v
			s.sendQ = newQ
			sizeQ, s.sizeQ = s.sizeQ, sizeQ
			s.Unlock()
			close(sizeQ)
			return nil
		}
		return protocol.ErrBadValue

	case protocol.OptionReadQLen:
		if v, ok := value.(int); ok && v >= 0 {
			newQ := make(chan *protocol.Message, v)
			sizeQ := make(chan struct{})
			s.Lock()
			s.recvQLen = v
			s.recvQ = newQ
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
	case protocol.OptionBestEffort:
		s.Lock()
		v := s.bestEffort
		s.Unlock()
		return v, nil
	case protocol.OptionWriteQLen:
		s.Lock()
		v := s.sendQLen
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

func (s *socket) Close() error {
	s.Lock()

	if s.closed {
		s.Unlock()
		return protocol.ErrClosed
	}
	s.closed = true
	s.sendQ = nil
	s.Unlock()
	close(s.closeQ)
	return nil
}

func (s *socket) AddPipe(pp protocol.Pipe) error {
	s.Lock()
	defer s.Unlock()
	p := &pipe{
		p:      pp,
		s:      s,
		closeQ: make(chan struct{}),
	}
	pp.SetPrivate(p)
	if s.closed {
		return protocol.ErrClosed
	}

	go p.sender()
	go p.receiver()
	return nil
}

func (s *socket) RemovePipe(pp protocol.Pipe) {
	p := pp.GetPrivate().(*pipe)
	close(p.closeQ)
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

// NewProtocol returns a new protocol implementation.
func NewProtocol() protocol.Protocol {
	s := &socket{
		closeQ:   make(chan struct{}),
		sizeQ:    make(chan struct{}),
		recvQ:    make(chan *protocol.Message, defaultQLen),
		sendQ:    make(chan *protocol.Message, defaultQLen),
		sendQLen: defaultQLen,
		recvQLen: defaultQLen,
	}
	return s
}

// NewSocket allocates a new Socket using the REQ protocol.
func NewSocket() (protocol.Socket, error) {
	return protocol.MakeSocket(NewProtocol()), nil
}
