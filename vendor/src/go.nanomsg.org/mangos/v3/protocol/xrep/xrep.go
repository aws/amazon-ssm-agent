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

// Package xrep implements the raw REP protocol, which is the response side of
// the request/response pattern.  (REQ is the request.)
package xrep

import (
	"encoding/binary"
	"sync"
	"time"

	"go.nanomsg.org/mangos/v3/protocol"
)

// Protocol identity information.
const (
	Self     = protocol.ProtoRep
	Peer     = protocol.ProtoReq
	SelfName = "rep"
	PeerName = "req"
)

type pipe struct {
	p      protocol.Pipe
	s      *socket
	closeQ chan struct{}
	sendQ  chan *protocol.Message
}

type socket struct {
	closed     bool
	closeQ     chan struct{}
	sizeQ      chan struct{}
	recvQ      chan *protocol.Message
	pipes      map[uint32]*pipe
	recvExpire time.Duration
	sendExpire time.Duration
	sendQLen   int
	recvQLen   int
	bestEffort bool
	ttl        int
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

// SendMessage implements sending a message.  The message must already
// have headers... the first 4 bytes are the identity of the pipe
// we should send to.
func (s *socket) SendMsg(m *protocol.Message) error {

	s.Lock()

	if s.closed {
		s.Unlock()
		return protocol.ErrClosed
	}

	if len(m.Header) < 4 {
		s.Unlock()
		m.Free()
		return nil
	}

	id := binary.BigEndian.Uint32(m.Header)
	hdr := m.Header
	m.Header = m.Header[4:]

	bestEffort := s.bestEffort
	tq := nilQ
	p, ok := s.pipes[id]
	if !ok {
		s.Unlock()
		m.Free()
		return nil
	}
	if bestEffort {
		tq = closedQ
	} else if s.sendExpire > 0 {
		tq = time.After(s.sendExpire)
	}
	s.Unlock()

	select {
	case p.sendQ <- m:
		return nil
	case <-p.closeQ:
		// restore the header
		m.Header = hdr
		return protocol.ErrClosed
	case <-tq:
		if bestEffort {
			m.Free()
			return nil
		}
		// restore the header
		m.Header = hdr
		return protocol.ErrSendTimeout
	}
}

func (s *socket) RecvMsg() (*protocol.Message, error) {
	for {
		s.Lock()
		timeQ := nilQ
		recvQ := s.recvQ
		sizeQ := s.sizeQ
		closeQ := s.closeQ
		if s.recvExpire > 0 {
			timeQ = time.After(s.recvExpire)
		}
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

func (p *pipe) receiver() {
	s := p.s
outer:
	for {
		m := p.p.RecvMsg()
		if m == nil {
			break
		}

		// Outer most value of header is pipe ID
		m.Header = append(make([]byte, 4), m.Header...)
		binary.BigEndian.PutUint32(m.Header, p.p.ID())

		s.Lock()
		ttl := s.ttl
		recvQ := s.recvQ
		sizeQ := s.sizeQ
		s.Unlock()

		hops := 1
		finish := false
		for !finish {
			if hops > ttl {
				m.Free()
				continue outer
			}
			hops++
			if len(m.Body) < 4 {
				m.Free() // Garbled!
				continue outer
			}
			if m.Body[0]&0x80 != 0 {
				// High order bit set indicates ID and end of
				// message headers.
				finish = true
			}
			m.Header = append(m.Header, m.Body[:4]...)
			m.Body = m.Body[4:]
		}

		select {
		case recvQ <- m:
			continue
		case <-sizeQ:
			continue
		case <-p.closeQ:
			m.Free()
			break outer
		}
	}
	go p.close()
}

// This is a puller, and doesn't permit for priorities.  We might want
// to refactor this to use a push based scheme later.
func (p *pipe) sender() {
outer:
	for {
		var m *protocol.Message
		select {
		case m = <-p.sendQ:
		case <-p.closeQ:
			break outer
		}

		if e := p.p.SendMsg(m); e != nil {
			break
		}
	}
	go p.close()
}

func (p *pipe) close() {
	_ = p.p.Close()
}

func (s *socket) SetOption(name string, value interface{}) error {
	switch name {

	case protocol.OptionTTL:
		if v, ok := value.(int); ok && v > 0 && v < 256 {
			s.Lock()
			s.ttl = v
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
			s.Lock()
			// This does not impact pipes already connected.
			s.sendQLen = v
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
	}

	return protocol.ErrBadOption
}

func (s *socket) GetOption(option string) (interface{}, error) {
	switch option {
	case protocol.OptionRaw:
		return true, nil
	case protocol.OptionTTL:
		s.Lock()
		v := s.ttl
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
	s.Unlock()
	close(s.closeQ)
	return nil
}

func (s *socket) AddPipe(pp protocol.Pipe) error {
	p := &pipe{
		p:      pp,
		s:      s,
		closeQ: make(chan struct{}),
		sendQ:  make(chan *protocol.Message, s.sendQLen),
	}
	pp.SetPrivate(p)
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return protocol.ErrClosed
	}
	s.pipes[pp.ID()] = p
	go p.sender()
	go p.receiver()
	return nil
}

func (s *socket) RemovePipe(pp protocol.Pipe) {
	p := pp.GetPrivate().(*pipe)
	close(p.closeQ)

	s.Lock()
	delete(s.pipes, p.p.ID())
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

// NewProtocol returns a new protocol implementation.
func NewProtocol() protocol.Protocol {
	s := &socket{
		pipes:    make(map[uint32]*pipe),
		closeQ:   make(chan struct{}),
		sizeQ:    make(chan struct{}),
		recvQ:    make(chan *protocol.Message, defaultQLen),
		sendQLen: defaultQLen,
		recvQLen: defaultQLen,
		ttl:      8,
	}
	return s
}

// NewSocket allocates a new Socket using the REP protocol.
func NewSocket() (protocol.Socket, error) {
	return protocol.MakeSocket(NewProtocol()), nil
}
