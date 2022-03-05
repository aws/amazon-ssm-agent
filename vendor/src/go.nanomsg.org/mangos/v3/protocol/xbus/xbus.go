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

// Package xbus implements the BUS protocol. This sends messages
// out to all peers, and receives their responses.  It specifically
// filters messages sent to itself, so that a single BUS can be used
// to loop back to peers.
package xbus

import (
	"encoding/binary"
	"sync"
	"time"

	"go.nanomsg.org/mangos/v3/protocol"
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
	pipes      map[uint32]*pipe
	recvQLen   int
	sendQLen   int
	recvExpire time.Duration
	recvQ      chan *protocol.Message
	sync.Mutex
}

// Protocol identity information.
const (
	Self     = protocol.ProtoBus
	Peer     = protocol.ProtoBus
	SelfName = "bus"
	PeerName = "bus"
)

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
	var id uint32

	if len(m.Header) == 4 {
		m = m.MakeUnique()
		// This is coming back to us - its a forwarded message
		// from an earlier pipe.  Note that we could also have
		// used the m.Pipe but this is how mangos v1 and nanomsg
		// did it historically.
		id = binary.BigEndian.Uint32(m.Header)
		m.Header = m.Header[:0]
	}

	for _, p := range s.pipes {

		// Don't deliver the message back up to the same pipe it
		// arrived from.
		if p.p.ID() == id {
			continue
		}
		m.Clone()
		select {
		case p.sendQ <- m:
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
	for {
		s.Lock()
		rq := s.recvQ
		cq := s.closeQ
		zq := s.sizeQ
		tq := nilQ
		if s.recvExpire > 0 {
			tq = time.After(s.recvExpire)
		}
		s.Unlock()

		select {
		case <-cq:
			return nil, protocol.ErrClosed
		case <-zq:
			continue
		case <-tq:
			return nil, protocol.ErrRecvTimeout
		case m := <-rq:
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
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return protocol.ErrClosed
	}

	p := &pipe{
		p:      pp,
		s:      s,
		closeQ: make(chan struct{}),
		sendQ:  make(chan *protocol.Message, s.sendQLen),
	}
	s.pipes[pp.ID()] = p
	pp.SetPrivate(p)

	go p.sender()
	go p.receiver()
	return nil
}

func (s *socket) RemovePipe(pp protocol.Pipe) {
	if p, ok := pp.GetPrivate().(*pipe); ok {
		s.Lock()
		delete(s.pipes, pp.ID())
		s.Unlock()
		close(p.closeQ)
	}
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

func (p *pipe) sender() {
outer:
	for {
		var m *protocol.Message
		select {
		case <-p.closeQ:
			break outer
		case m = <-p.sendQ:
		}

		if err := p.p.SendMsg(m); err != nil {
			m.Free()
			break
		}
	}
	p.Close()
}

func (p *pipe) receiver() {
	s := p.s
outer:
	for {
		m := p.p.RecvMsg()
		if m == nil {
			break
		}

		// We store the received pipe ID in the header.
		// This permits a device to be set up as a bouncer.
		// In that case, this pipe won't get a copy of the
		// message.

		m.Header = make([]byte, 4)
		binary.BigEndian.PutUint32(m.Header, p.p.ID())

		s.Lock()
		rq := s.recvQ
		zq := s.sizeQ
		s.Unlock()

		select {
		case rq <- m:
		case <-p.closeQ:
			m.Free()
			break outer
		case <-zq:
			m.Free() // discard this one
			break outer
		}
	}
	p.Close()
}

func (p *pipe) Close() {
	_ = p.p.Close()
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
	}
	return s
}

// NewSocket allocates a raw Socket using the BUS protocol.
func NewSocket() (protocol.Socket, error) {
	return protocol.MakeSocket(NewProtocol()), nil
}
