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

// Package surveyor implements the SURVEYOR protocol. This sends messages
// out to RESPONDENT partners, and receives their responses.
package surveyor

import (
	"encoding/binary"
	"sync"
	"sync/atomic"
	"time"

	"go.nanomsg.org/mangos/v3/protocol"
)

// Protocol identity information.
const (
	Self     = protocol.ProtoSurveyor
	Peer     = protocol.ProtoRespondent
	SelfName = "surveyor"
	PeerName = "respondent"
)

const defaultSurveyTime = time.Second

type pipe struct {
	s      *socket
	p      protocol.Pipe
	closeQ chan struct{}
	sendQ  chan *protocol.Message
}

type survey struct {
	timer  *time.Timer
	recvQ  chan *protocol.Message
	active bool
	id     uint32
	ctx    *context
	sock   *socket
	err    error
	once   sync.Once
}

type context struct {
	s          *socket
	closed     bool
	closeQ     chan struct{}
	recvQLen   int
	recvExpire time.Duration
	survExpire time.Duration
	surv       *survey
}

type socket struct {
	master   *context              // default context
	ctxs     map[*context]struct{} // all contexts
	surveys  map[uint32]*survey    // contexts by survey ID
	pipes    map[uint32]*pipe      // all pipes by pipe ID
	nextID   uint32                // next survey ID
	closed   bool                  // true if closed
	sendQLen int                   // send Q depth
	sync.Mutex
}

var (
	nilQ <-chan time.Time
)

const defaultQLen = 128

func (s *survey) cancel(err error) {

	s.once.Do(func() {
		sock := s.sock
		ctx := s.ctx

		s.err = err
		sock.Lock()
		s.timer.Stop()
		if ctx.surv == s {
			ctx.surv = nil
		}
		delete(sock.surveys, s.id)
		sock.Unlock()

		// Don't close this until after we have removed it from
		// the list of pending surveys, to prevent the receiver
		// from trying to write to a closed channel.
		close(s.recvQ)
		for m := range s.recvQ {
			m.Free()
		}
	})
}

func (s *survey) start(qLen int, expire time.Duration) {
	// NB: Called with the socket lock held
	s.recvQ = make(chan *protocol.Message, qLen)
	s.sock.surveys[s.id] = s
	s.ctx.surv = s
	s.timer = time.AfterFunc(expire, func() {
		s.cancel(protocol.ErrProtoState)
	})
}

func (c *context) SendMsg(m *protocol.Message) error {
	s := c.s

	newsurv := &survey{
		active: true,
		id:     atomic.AddUint32(&s.nextID, 1) | 0x80000000,
		ctx:    c,
		sock:   s,
	}

	m.MakeUnique()
	m.Header = make([]byte, 4)
	binary.BigEndian.PutUint32(m.Header, newsurv.id)

	s.Lock()
	if s.closed || c.closed {
		s.Unlock()
		return protocol.ErrClosed
	}
	oldsurv := c.surv
	newsurv.start(c.recvQLen, c.survExpire)
	if oldsurv != nil {
		go oldsurv.cancel(protocol.ErrCanceled)
	}
	pipes := make([]*pipe, 0, len(s.pipes))
	for _, p := range s.pipes {
		pipes = append(pipes, p)
	}
	s.Unlock()

	// Best-effort broadcast on all pipes
	for _, p := range pipes {
		m.Clone()
		select {
		case p.sendQ <- m:
		default:
			m.Free()
		}
	}
	m.Free()
	return nil
}

func (c *context) RecvMsg() (*protocol.Message, error) {
	s := c.s

	s.Lock()
	if s.closed {
		s.Unlock()
		return nil, protocol.ErrClosed
	}
	surv := c.surv
	timeq := nilQ
	if c.recvExpire > 0 {
		timeq = time.After(c.recvExpire)
	}
	s.Unlock()

	if surv == nil {
		return nil, protocol.ErrProtoState
	}
	select {
	case <-c.closeQ:
		return nil, protocol.ErrClosed

	case m := <-surv.recvQ:
		if m == nil {
			// Sometimes the recvQ can get closed ahead of
			// the closeQ, but the closeQ takes precedence.
			return nil, surv.err
		}
		return m, nil

	case <-timeq:
		return nil, protocol.ErrRecvTimeout
	}
}

func (c *context) close() {
	if !c.closed {
		c.closed = true
		close(c.closeQ)
		if surv := c.surv; surv != nil {
			c.surv = nil
			go surv.cancel(protocol.ErrClosed)
		}
	}
}

func (c *context) Close() error {
	c.s.Lock()
	defer c.s.Unlock()
	if c.closed {
		return protocol.ErrClosed
	}
	c.close()
	return nil
}

func (c *context) SetOption(name string, value interface{}) error {
	switch name {
	case protocol.OptionSurveyTime:
		if v, ok := value.(time.Duration); ok {
			c.s.Lock()
			c.survExpire = v
			c.s.Unlock()
			return nil
		}
		return protocol.ErrBadValue

	case protocol.OptionRecvDeadline:
		if v, ok := value.(time.Duration); ok {
			c.s.Lock()
			c.recvExpire = v
			c.s.Unlock()
			return nil
		}
		return protocol.ErrBadValue

	case protocol.OptionReadQLen:
		if v, ok := value.(int); ok && v >= 0 {
			// this will only affect new surveys
			c.s.Lock()
			c.recvQLen = v
			c.s.Unlock()
			return nil
		}
		return protocol.ErrBadValue
	}

	return protocol.ErrBadOption
}

func (c *context) GetOption(option string) (interface{}, error) {
	switch option {
	case protocol.OptionSurveyTime:
		c.s.Lock()
		v := c.survExpire
		c.s.Unlock()
		return v, nil
	case protocol.OptionRecvDeadline:
		c.s.Lock()
		v := c.recvExpire
		c.s.Unlock()
		return v, nil
	case protocol.OptionReadQLen:
		c.s.Lock()
		v := c.recvQLen
		c.s.Unlock()
		return v, nil
	}

	return nil, protocol.ErrBadOption
}

func (p *pipe) close() {
	_ = p.p.Close()
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
	p.close()
}

func (p *pipe) receiver() {
	s := p.s
	for {
		m := p.p.RecvMsg()
		if m == nil {
			break
		}
		if len(m.Body) < 4 {
			m.Free()
			continue
		}
		m.Header = append(m.Header, m.Body[:4]...)
		m.Body = m.Body[4:]

		id := binary.BigEndian.Uint32(m.Header)

		s.Lock()
		if surv, ok := s.surveys[id]; ok {
			select {
			case surv.recvQ <- m:
				m = nil
			default:
			}
		}
		s.Unlock()

		if m != nil {
			m.Free()
		}
	}
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
		survExpire: s.master.survExpire,
		recvExpire: s.master.recvExpire,
		recvQLen:   s.master.recvQLen,
	}
	s.ctxs[c] = struct{}{}
	return c, nil
}

func (s *socket) SendMsg(m *protocol.Message) error {
	return s.master.SendMsg(m)
}

func (s *socket) RecvMsg() (*protocol.Message, error) {
	return s.master.RecvMsg()
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
	defer s.Unlock()
	if s.closed {
		return protocol.ErrClosed
	}
	s.pipes[p.p.ID()] = p
	go p.receiver()
	go p.sender()
	return nil
}

func (s *socket) RemovePipe(pp protocol.Pipe) {
	p := pp.GetPrivate().(*pipe)
	close(p.closeQ)
	s.Lock()
	delete(s.pipes, pp.ID())
	s.Unlock()
}

func (s *socket) Close() error {
	s.Lock()
	if s.closed {
		s.Unlock()
		return protocol.ErrClosed
	}
	s.closed = true
	for c := range s.ctxs {
		c.close()
	}
	s.Unlock()
	return nil
}

func (s *socket) GetOption(option string) (interface{}, error) {
	switch option {
	case protocol.OptionRaw:
		return false, nil
	case protocol.OptionWriteQLen:
		s.Lock()
		v := s.sendQLen
		s.Unlock()
		return v, nil

	default:
		return s.master.GetOption(option)
	}
}

func (s *socket) SetOption(option string, value interface{}) error {
	switch option {
	case protocol.OptionWriteQLen:
		if v, ok := value.(int); ok && v >= 0 {
			s.Lock()
			s.sendQLen = v
			s.Unlock()
			return nil
		}
		return protocol.ErrBadValue
	}
	return s.master.SetOption(option, value)
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
		surveys:  make(map[uint32]*survey),
		ctxs:     make(map[*context]struct{}),
		sendQLen: defaultQLen,
		nextID:   uint32(time.Now().UnixNano()), // quasi-random
	}
	s.master = &context{
		s:          s,
		closeQ:     make(chan struct{}),
		recvQLen:   defaultQLen,
		survExpire: defaultSurveyTime,
	}
	s.ctxs[s.master] = struct{}{}
	return s
}

// NewSocket allocates a new Socket using the RESPONDENT protocol.
func NewSocket() (protocol.Socket, error) {
	return protocol.MakeSocket(NewProtocol()), nil
}
