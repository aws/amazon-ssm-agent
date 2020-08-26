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

// Package respondent implements the RESPONDENT protocol, which is the
// response side of the survey pattern.
package respondent

import (
	"sync"
	"time"

	"go.nanomsg.org/mangos/v3/protocol"
)

// Protocol identity information.
const (
	Self     = protocol.ProtoRespondent
	Peer     = protocol.ProtoSurveyor
	SelfName = "respondent"
	PeerName = "surveyor"
)

type msg struct {
	m *protocol.Message
	p *pipe
}

type pipe struct {
	s      *socket
	p      protocol.Pipe
	sendQ  chan *protocol.Message
	closeQ chan struct{}
}

type socket struct {
	closed   bool
	ttl      int
	sendQLen int
	recvQLen int
	sizeQ    chan struct{}
	recvQ    chan msg
	contexts map[*context]struct{}
	defCtx   *context
	closeQ   chan struct{}

	sync.Mutex
}

type context struct {
	s          *socket
	closed     bool
	recvExpire time.Duration
	sendExpire time.Duration
	bestEffort bool
	recvPipe   *pipe
	backtrace  []byte
	closeQ     chan struct{}
}

const defaultQLen = 128

// closedQ represents a non-blocking time channel.
var closedQ <-chan time.Time

// nilQ represents a nil time channel (blocks forever)
var nilQ <-chan time.Time

func init() {
	tq := make(chan time.Time)
	closedQ = tq
	close(tq)
}

func (c *context) RecvMsg() (*protocol.Message, error) {
	s := c.s

	for {
		s.Lock()
		if c.closed {
			s.Unlock()
			return nil, protocol.ErrClosed
		}
		cq := c.closeQ
		tq := nilQ
		rq := s.recvQ
		zq := s.sizeQ
		expTime := c.recvExpire
		c.backtrace = nil
		c.recvPipe = nil
		s.Unlock()

		if expTime > 0 {
			tq = time.After(expTime)
		}

		select {
		case msg := <-rq:
			s.Lock()
			c.recvPipe = msg.p
			c.backtrace = append([]byte{}, msg.m.Header...)
			s.Unlock()
			msg.m.Header = nil
			return msg.m, nil
		case <-zq:
			continue
		case <-tq:
			return nil, protocol.ErrRecvTimeout
		case <-cq:
			return nil, protocol.ErrClosed
		}
	}
}

func (c *context) SendMsg(m *protocol.Message) error {

	s := c.s
	s.Lock()
	if s.closed || c.closed {
		s.Unlock()
		return protocol.ErrClosed
	}
	if c.backtrace == nil {
		s.Unlock()
		return protocol.ErrProtoState
	}
	p := c.recvPipe
	bt := c.backtrace
	c.backtrace = nil
	c.recvPipe = nil
	bestEffort := c.bestEffort
	tq := nilQ
	cq := c.closeQ
	s.Unlock()

	if bestEffort {
		tq = closedQ
	} else if c.sendExpire > 0 {
		tq = time.After(c.sendExpire)
	}

	m.Header = bt

	select {
	case <-cq:
		m.Header = nil
		return protocol.ErrClosed
	case <-p.closeQ:
		// Pipe closed, so no way to get it to the recipient.
		// Just discard the message.
		m.Free()
		return nil
	case <-tq:
		if bestEffort {
			m.Free()
			return nil
		}
		m.Header = nil
		return protocol.ErrSendTimeout

	case p.sendQ <- m:
		return nil
	}
}

func (c *context) close() {
	if !c.closed {
		delete(c.s.contexts, c)
		c.closed = true
		close(c.closeQ)
	}
}

func (c *context) Close() error {
	s := c.s
	s.Lock()
	defer s.Unlock()
	if c.closed {
		return protocol.ErrClosed
	}
	c.close()
	return nil
}

func (c *context) GetOption(name string) (interface{}, error) {
	switch name {
	case protocol.OptionBestEffort:
		c.s.Lock()
		v := c.bestEffort
		c.s.Unlock()
		return v, nil

	case protocol.OptionRecvDeadline:
		c.s.Lock()
		v := c.recvExpire
		c.s.Unlock()
		return v, nil

	case protocol.OptionSendDeadline:
		c.s.Lock()
		v := c.sendExpire
		c.s.Unlock()
		return v, nil

	default:
		return nil, protocol.ErrBadOption
	}
}

func (c *context) SetOption(name string, v interface{}) error {
	switch name {
	case protocol.OptionSendDeadline:
		if val, ok := v.(time.Duration); ok && val.Nanoseconds() > 0 {
			c.s.Lock()
			c.sendExpire = val
			c.s.Unlock()
			return nil
		}
		return protocol.ErrBadValue

	case protocol.OptionRecvDeadline:
		if val, ok := v.(time.Duration); ok && val.Nanoseconds() > 0 {
			c.s.Lock()
			c.recvExpire = val
			c.s.Unlock()
			return nil
		}
		return protocol.ErrBadValue

	case protocol.OptionBestEffort:
		if val, ok := v.(bool); ok {
			c.s.Lock()
			c.bestEffort = val
			c.s.Unlock()
			return nil
		}
		return protocol.ErrBadValue

	default:
		return protocol.ErrBadOption
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

		// Move backtrace from body to header.
		hops := 0
		for {
			if hops >= s.ttl {
				m.Free() // ErrTooManyHops
				continue outer
			}
			hops++
			if len(m.Body) < 4 {
				m.Free() // ErrGarbled
				continue outer
			}
			m.Header = append(m.Header, m.Body[:4]...)
			m.Body = m.Body[4:]
			// Check for high order bit set (0x80000000, big endian)
			if m.Header[len(m.Header)-4]&0x80 != 0 {
				// Protocol error from partner, discard and
				// close the pipe.
				break
			}
		}
		msg := msg{
			m: m,
			p: p,
		}

	inner:
		for {
			s.Lock()
			rq := s.recvQ
			cq := s.closeQ
			zq := s.sizeQ
			s.Unlock()

			select {
			case <-zq:
				continue inner
			case rq <- msg:
				break inner
			case <-cq:
				m.Free()
				break outer
			}
		}
	}
	go p.close()
}

func (p *pipe) sender() {
	for {
		select {
		case m := <-p.sendQ:
			if p.p.SendMsg(m) != nil {
				p.close()
				return
			}
		case <-p.closeQ:
			return
		}
	}
}

func (p *pipe) close() {
	_ = p.p.Close()
}

func (s *socket) Close() error {

	s.Lock()
	defer s.Unlock()

	if s.closed {
		return protocol.ErrClosed
	}
	s.closed = true
	close(s.closeQ)
	for c := range s.contexts {
		c.close()
	}
	return nil
}

func (*socket) Info() protocol.Info {
	return protocol.Info{
		Self:     Self,
		Peer:     Peer,
		SelfName: SelfName,
		PeerName: PeerName,
	}
}

func (s *socket) AddPipe(pp protocol.Pipe) error {

	p := &pipe{
		p:      pp,
		s:      s,
		sendQ:  make(chan *protocol.Message, s.sendQLen),
		closeQ: make(chan struct{}),
	}
	pp.SetPrivate(p)
	s.Lock()
	if s.closed {
		s.Unlock()
		return protocol.ErrClosed
	}
	go p.sender()
	go p.receiver()
	s.Unlock()
	return nil
}

func (s *socket) RemovePipe(pp protocol.Pipe) {

	p := pp.GetPrivate().(*pipe)
	close(p.closeQ)
}

func (s *socket) SetOption(name string, v interface{}) error {
	switch name {
	case protocol.OptionWriteQLen:
		if qLen, ok := v.(int); ok && qLen >= 0 {
			s.Lock()
			s.sendQLen = qLen
			s.Unlock()
			return nil
		}
		return protocol.ErrBadValue

	case protocol.OptionReadQLen:
		if qLen, ok := v.(int); ok && qLen >= 0 {

			newQ := make(chan msg, qLen)
			s.Lock()
			sizeQ := s.sizeQ
			s.sizeQ = make(chan struct{})
			s.recvQ = newQ
			s.recvQLen = qLen
			s.Unlock()

			// Close the sizeQ to let anyone watching know that
			// they should re-examine the recvQ.
			close(sizeQ)
			return nil
		}
		return protocol.ErrBadValue

	case protocol.OptionTTL:
		if ttl, ok := v.(int); ok && ttl > 0 && ttl < 256 {
			s.Lock()
			s.ttl = ttl
			s.Unlock()
			return nil
		}
		return protocol.ErrBadValue
	}
	return s.defCtx.SetOption(name, v)
}

func (s *socket) GetOption(name string) (interface{}, error) {
	switch name {
	case protocol.OptionRaw:
		return false, nil
	case protocol.OptionTTL:
		s.Lock()
		v := s.ttl
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

	return s.defCtx.GetOption(name)
}

func (s *socket) OpenContext() (protocol.Context, error) {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return nil, protocol.ErrClosed
	}
	c := &context{
		s:          s,
		closeQ:     make(chan struct{}),
		bestEffort: s.defCtx.bestEffort,
		recvExpire: s.defCtx.recvExpire,
		sendExpire: s.defCtx.sendExpire,
	}
	s.contexts[c] = struct{}{}
	return c, nil
}

func (s *socket) RecvMsg() (*protocol.Message, error) {
	return s.defCtx.RecvMsg()
}

func (s *socket) SendMsg(m *protocol.Message) error {
	return s.defCtx.SendMsg(m)
}

// NewProtocol allocates a protocol state for the RESPONDENT protocol.
func NewProtocol() protocol.Protocol {
	s := &socket{
		ttl:      8,
		contexts: make(map[*context]struct{}),
		recvQLen: defaultQLen,
		recvQ:    make(chan msg, defaultQLen),
		closeQ:   make(chan struct{}),
		sizeQ:    make(chan struct{}),
		defCtx: &context{
			closeQ: make(chan struct{}),
		},
	}
	s.defCtx.s = s
	s.contexts[s.defCtx] = struct{}{}
	return s
}

// NewSocket allocates a new Socket using the REP protocol.
func NewSocket() (protocol.Socket, error) {
	return protocol.MakeSocket(NewProtocol()), nil
}
