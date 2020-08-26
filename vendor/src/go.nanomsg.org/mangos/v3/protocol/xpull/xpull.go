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

// Package xpull implements the PULL protocol. This read only protocol
// simply receives messages from pipes.
package xpull

import (
	"sync"
	"time"

	"go.nanomsg.org/mangos/v3/protocol"
)

// Protocol identity information.
const (
	Self     = protocol.ProtoPull
	Peer     = protocol.ProtoPush
	SelfName = "pull"
	PeerName = "push"
)

type pipe struct {
	p      protocol.Pipe
	s      *socket
	closeQ chan struct{}
}

type socket struct {
	closed         bool
	closeQ         chan struct{}
	sizeQ          chan struct{}
	recvQ          chan *protocol.Message
	recvQLen       int
	resizeDiscards bool // only for testing (facilitates coverage)
	recvExpire     time.Duration
	sync.Mutex
}

var (
	nilQ <-chan time.Time
)

const defaultQLen = 128

func (s *socket) SendMsg(m *protocol.Message) error {
	return protocol.ErrProtoOp
}

func (s *socket) RecvMsg() (*protocol.Message, error) {
	// For now this uses a simple unified queue for the entire
	// socket.  Later we can look at moving this to priority queues
	// based on socket pipes.
	tq := nilQ
	for {
		s.Lock()
		if s.recvExpire > 0 {
			tq = time.After(s.recvExpire)
		}
		cq := s.closeQ
		rq := s.recvQ
		zq := s.sizeQ
		s.Unlock()
		select {
		case <-cq:
			return nil, protocol.ErrClosed
		case <-tq:
			return nil, protocol.ErrRecvTimeout
		case <-zq:
			continue
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

	case protocol.OptionReadQLen:
		if v, ok := value.(int); ok && v >= 0 {
			newQ := make(chan *protocol.Message, v)
			s.Lock()
			s.recvQLen = v
			oldQ := s.recvQ
			s.recvQ = newQ
			zq := s.sizeQ
			s.sizeQ = make(chan struct{})
			discard := s.resizeDiscards
			s.Unlock()

			close(zq)
			if !discard {
				for {
					var m *protocol.Message
					select {
					case m = <-oldQ:
					default:
					}
					if m == nil {
						break
					}
					select {
					case newQ <- m:
					default:
						m.Free()
					}
				}
			}
			return nil
		}
		return protocol.ErrBadValue

	case "_resizeDiscards":
		// This option is here to facilitate testing.
		if v, ok := value.(bool); ok {
			s.Lock()
			s.resizeDiscards = v
			s.Unlock()
		}
		return nil
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
		p:      pp,
		s:      s,
		closeQ: make(chan struct{}),
	}
	pp.SetPrivate(p)
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return protocol.ErrClosed
	}
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

	inner:
		for {
			s.Lock()
			rq := s.recvQ
			zq := s.sizeQ
			s.Unlock()

			select {
			case rq <- m:
				continue outer
			case <-zq:
				continue inner
			case <-p.closeQ:
				m.Free()
				break outer
			}
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
