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

// Package xstar implements the experimental star protocol. This sends messages
// out to all peers, and receives their responses.  It also implicitly resends
// any message it receives to all of its peers, but it will not rebroadcast
// a message to the peer it was received from.
package xstar

import (
	"sync"
	"time"

	"go.nanomsg.org/mangos/v3/protocol"
)

// Protocol identity information.
const (
	Self     = protocol.ProtoStar
	Peer     = protocol.ProtoStar
	SelfName = "star"
	PeerName = "star"
)

type pipe struct {
	p      protocol.Pipe
	s      *socket
	closeq chan struct{}
	sendq  chan *protocol.Message
}

type socket struct {
	closed     bool
	closeq     chan struct{}
	pipes      map[uint32]*pipe
	recvQLen   int
	sendQLen   int
	recvExpire time.Duration
	recvq      chan *protocol.Message
	ttl        int
	sync.Mutex
}

var (
	nilQ <-chan time.Time
)

const defaultQLen = 128

func (s *socket) SendMsg(m *protocol.Message) error {
	s.Lock()
	if s.closed {
		s.Unlock()
		return protocol.ErrClosed
	}

	// Raw mode messages are required to come wth the header.
	if len(m.Header) != 4 {
		s.Unlock()
		m.Free()
		return nil
	}

	for _, p := range s.pipes {

		m.Clone()
		select {
		case p.sendq <- m:
		default:
			// back-pressure, but we do not exert
			m.Free()
		}
	}
	s.Unlock()
	m.Free()
	return nil
}

func (s *socket) RecvMsg() (*protocol.Message, error) {
	// For now this uses a simple unified queue for the entire
	// socket.  Later we can look at moving this to priority queues
	// based on socket pipes.
	tq := nilQ
	s.Lock()
	if s.recvExpire > 0 {
		tq = time.After(s.recvExpire)
	}
	s.Unlock()
	select {
	case <-s.closeq:
		return nil, protocol.ErrClosed
	case <-tq:
		return nil, protocol.ErrRecvTimeout
	case m := <-s.recvq:
		return m, nil
	}
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

	case protocol.OptionWriteQLen:
		if v, ok := value.(int); ok && v >= 0 {
			s.Lock()
			s.sendQLen = v
			s.Unlock()
			return nil
		}
		return protocol.ErrBadValue

	case protocol.OptionReadQLen:
		if v, ok := value.(int); ok && v >= 0 {
			newchan := make(chan *protocol.Message, v)
			s.Lock()
			s.recvQLen = v
			s.recvq = newchan
			s.Unlock()

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

func (s *socket) AddPipe(pp protocol.Pipe) error {
	p := &pipe{
		p:      pp,
		s:      s,
		closeq: make(chan struct{}),
		sendq:  make(chan *protocol.Message, s.sendQLen),
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
	close(p.closeq)
	s.Lock()
	delete(p.s.pipes, p.p.ID())
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

	close(s.closeq)
	return nil
}

func (p *pipe) sender() {
outer:
	for {
		var m *protocol.Message
		select {
		case <-p.closeq:
			break outer
		case m = <-p.sendq:
		}

		if err := p.p.SendMsg(m); err != nil {
			m.Free()
			break
		}
	}
	p.close()
}

func (p *pipe) receiver() {
	s := p.s
outer:
	for {
		m := p.p.RecvMsg()
		if m == nil {
			break
		}

		if len(m.Body) < 4 ||
			m.Body[0] != 0 || m.Body[1] != 0 || m.Body[2] != 0 ||
			int(m.Body[3]) >= s.ttl {
			m.Free()
			continue
		}
		m.Header = m.Body[:4]
		m.Body = m.Body[4:]
		m.Header[3]++

		userm := m.Dup()
		s.Lock()
		for _, p2 := range s.pipes {
			if p2 == p {
				continue
			}

			m2 := m.Dup()
			select {
			case p2.sendq <- m2:
			default:
				m2.Free()
			}
		}
		s.Unlock()
		m.Free()

		select {
		case s.recvq <- userm:
		case <-p.closeq:
			userm.Free()
			break outer
		case <-s.closeq:
			userm.Free()
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
		pipes:    make(map[uint32]*pipe),
		closeq:   make(chan struct{}),
		recvq:    make(chan *protocol.Message, defaultQLen),
		sendQLen: defaultQLen,
		recvQLen: defaultQLen,
		ttl:      8,
	}
	return s
}

// NewSocket allocates a raw Socket using the BUS protocol.
func NewSocket() (protocol.Socket, error) {
	return protocol.MakeSocket(NewProtocol()), nil
}
