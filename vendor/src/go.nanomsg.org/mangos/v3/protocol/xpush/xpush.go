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

// Package xpush implements the raw PUSH protocol.
package xpush

import (
	"sync"
	"time"

	"go.nanomsg.org/mangos/v3/protocol"
)

// Protocol identity information.
const (
	Self     = protocol.ProtoPush
	Peer     = protocol.ProtoPull
	SelfName = "push"
	PeerName = "pull"
)

type pipe struct {
	p      protocol.Pipe
	s      *socket
	closed bool
	sendQ  chan *protocol.Message
	closeQ chan struct{}
}

type socket struct {
	closed     bool
	closeQ     chan struct{}
	sendQ      chan *protocol.Message
	sendExpire time.Duration
	sendQLen   int
	bestEffort bool
	readyQ     []*pipe
	cv         *sync.Cond
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
	tq := nilQ
	if bestEffort {
		tq = closedQ
	} else if s.sendExpire > 0 {
		tq = time.After(s.sendExpire)
	}
	if s.closed {
		s.Unlock()
		return protocol.ErrClosed
	}
	s.Unlock()

	select {
	case s.sendQ <- m:
	case <-s.closeQ:
		return protocol.ErrClosed
	case <-tq:
		if bestEffort {
			m.Free()
			return nil
		}
		return protocol.ErrSendTimeout
	}

	s.Lock()
	s.cv.Signal()
	s.Unlock()
	return nil
}

func (s *socket) sender() {
	s.Lock()
	defer s.Unlock()
	for {
		if s.closed {
			return
		}
		if len(s.readyQ) == 0 || len(s.sendQ) == 0 {
			s.cv.Wait()
			continue
		}
		m := <-s.sendQ
		p := s.readyQ[0]
		s.readyQ = s.readyQ[1:]
		go p.send(m)
	}
}

func (s *socket) RecvMsg() (*protocol.Message, error) {
	return nil, protocol.ErrProtoOp
}

func (p *pipe) receiver() {
	for {
		m := p.p.RecvMsg()
		if m == nil {
			break
		}
		// We really never expected to receive this.
		m.Free()
	}
	p.close()
}

func (p *pipe) send(m *protocol.Message) {
	s := p.s
	if err := p.p.SendMsg(m); err != nil {
		m.Free()
		p.close()
		return
	}
	s.Lock()
	if !s.closed && !p.closed {
		s.readyQ = append(s.readyQ, p)
		s.cv.Broadcast()
	}
	s.Unlock()

}

func (p *pipe) close() {
	_ = p.p.Close()
}

func (s *socket) SetOption(name string, value interface{}) error {
	switch name {

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
			s.Lock()
			s.sendQLen = v
			oldQ := s.sendQ
			s.sendQ = newQ

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
		sendQ:  make(chan *protocol.Message, 1),
	}
	pp.SetPrivate(p)
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return protocol.ErrClosed
	}
	go p.receiver()

	s.readyQ = append(s.readyQ, p)
	s.cv.Broadcast()
	return nil
}

func (s *socket) RemovePipe(pp protocol.Pipe) {
	p := pp.GetPrivate().(*pipe)
	close(p.closeQ)

	s.Lock()
	p.closed = true
	for i, rp := range s.readyQ {
		if p == rp {
			s.readyQ = append(s.readyQ[:i], s.readyQ[i+1:]...)
			break
		}
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

// NewProtocol returns a new protocol implementation.
func NewProtocol() protocol.Protocol {
	s := &socket{
		closeQ:   make(chan struct{}),
		sendQ:    make(chan *protocol.Message, defaultQLen),
		sendQLen: defaultQLen,
	}
	s.cv = sync.NewCond(s)
	go s.sender()
	return s
}

// NewSocket allocates a new Socket using the REQ protocol.
func NewSocket() (protocol.Socket, error) {
	return protocol.MakeSocket(NewProtocol()), nil
}
