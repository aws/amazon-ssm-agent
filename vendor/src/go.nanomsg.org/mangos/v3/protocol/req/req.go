// Copyright 2020 The Mangos Authors
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

// Package req implements the REQ protocol, which is the request side of
// the request/response pattern.  (REP is the response.)
package req

import (
	"encoding/binary"
	"sync"
	"sync/atomic"
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
	closed bool
}

type context struct {
	s          *socket
	cond       *sync.Cond
	resendTime time.Duration     // tunable resend time
	sendExpire time.Duration     // how long to wait in send
	recvExpire time.Duration     // how long to wait in recv
	sendTimer  *time.Timer       // send timer
	recvTimer  *time.Timer       // recv timer
	resender   *time.Timer       // resend timeout
	reqMsg     *protocol.Message // message for transmit
	repMsg     *protocol.Message // received reply
	sendMsg    *protocol.Message // messaging waiting for send
	lastPipe   *pipe             // last pipe used for transmit
	reqID      uint32            // request ID
	recvWait   bool              // true if a thread is blocked in RecvMsg
	bestEffort bool              // if true, don't block waiting in send
	queued     bool              // true if we need to send a message
	closed     bool              // true if we are closed
}

type socket struct {
	sync.Mutex
	defCtx  *context              // default context
	ctxs    map[*context]struct{} // all contexts (set)
	ctxByID map[uint32]*context   // contexts by request ID
	nextID  uint32                // next request ID
	closed  bool                  // true if we are closed
	sendq   []*context            // contexts waiting to send
	readyq  []*pipe               // pipes available for sending
}

func (s *socket) send() {
	for len(s.sendq) != 0 && len(s.readyq) != 0 {
		c := s.sendq[0]
		s.sendq = s.sendq[1:]
		c.queued = false

		var m *protocol.Message
		if m = c.sendMsg; m != nil {
			c.reqMsg = m
			c.sendMsg = nil
			s.ctxByID[c.reqID] = c
			c.cond.Broadcast()
		} else {
			m = c.reqMsg
		}
		m.Clone()
		p := s.readyq[0]
		s.readyq = s.readyq[1:]

		// Schedule a retransmit for the future.
		c.lastPipe = p
		if c.resendTime > 0 {
			id := c.reqID
			c.resender = time.AfterFunc(c.resendTime, func() {
				c.resendMessage(id)
			})
		}
		go p.sendCtx(c, m)
	}
}

func (p *pipe) sendCtx(c *context, m *protocol.Message) {
	s := p.s

	// Send this message.  If an error occurs, we examine the
	// error.  If it is ErrClosed, we don't schedule our self.
	if err := p.p.SendMsg(m); err != nil {
		m.Free()
		if err == protocol.ErrClosed {
			return
		}
	}
	s.Lock()
	if !c.closed && !p.closed {
		s.readyq = append(s.readyq, p)
		s.send()
	}
	s.Unlock()
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
		// Since we just received a reply, stick our send at the
		// head of the list, since that's a good indication that
		// we're ready for another request.
		for i, rp := range s.readyq {
			if p == rp {
				s.readyq[0], s.readyq[i] = s.readyq[i], s.readyq[0]
				break
			}
		}

		if c, ok := s.ctxByID[id]; ok {
			c.unscheduleSend()
			c.reqMsg.Free()
			c.reqMsg = nil
			c.repMsg = m
			delete(s.ctxByID, id)
			if c.resender != nil {
				c.resender.Stop()
				c.resender = nil
			}
			c.cond.Broadcast()
		} else {
			// No matching receiver so just drop it.
			m.Free()
		}
		s.Unlock()
	}

	go p.Close()
}

func (p *pipe) Close() {
	_ = p.p.Close()
}

func (c *context) resendMessage(id uint32) {
	s := c.s
	s.Lock()
	defer s.Unlock()
	if c.reqID == id {
		if !c.queued {
			c.queued = true
			s.sendq = append(s.sendq, c)
			s.send()
		}
	}
}

func (c *context) unscheduleSend() {
	s := c.s
	if c.queued {
		c.queued = false
		for i, c2 := range s.sendq {
			if c2 == c {
				s.sendq = append(s.sendq[:i], s.sendq[i+1:]...)
				return
			}
		}
	}
}

func (c *context) cancel() {
	s := c.s
	c.unscheduleSend()
	if c.reqID != 0 {
		delete(s.ctxByID, c.reqID)
		c.reqID = 0
	}
	if c.repMsg != nil {
		c.repMsg.Free()
		c.repMsg = nil
	}
	if c.reqMsg != nil {
		c.reqMsg.Free()
		c.reqMsg = nil
	}
	if c.resender != nil {
		c.resender.Stop()
		c.resender = nil
	}
	if c.sendTimer != nil {
		c.sendTimer.Stop()
		c.sendTimer = nil
	}
	if c.recvTimer != nil {
		c.recvTimer.Stop()
		c.recvTimer = nil
	}
	c.cond.Broadcast()
}

func (c *context) SendMsg(m *protocol.Message) error {

	s := c.s

	id := atomic.AddUint32(&s.nextID, 1)
	id |= 0x80000000

	// cooked mode, we stash the header
	m.Header = append([]byte{},
		byte(id>>24), byte(id>>16), byte(id>>8), byte(id))

	s.Lock()
	defer s.Unlock()
	if s.closed || c.closed {
		return protocol.ErrClosed
	}

	c.cancel() // this cancels any pending send or recv calls
	c.unscheduleSend()

	c.reqID = id
	c.queued = true
	c.sendMsg = m

	s.sendq = append(s.sendq, c)

	if c.bestEffort {
		// for best effort case, we just immediately go the
		// reqMsg, and schedule it as a send.  No waiting.
		// This means that if the message cannot be delivered
		// immediately, it will still get a chance later.
		s.send()
		return nil
	}

	expired := false
	if c.sendExpire > 0 {
		c.sendTimer = time.AfterFunc(c.sendExpire, func() {
			s.Lock()
			if c.sendMsg == m {
				expired = true
				c.cancel() // also does a wake up
			}
			s.Unlock()
		})
	}

	s.send()

	// This sleeps until someone picks us up for scheduling.
	// It is responsible for providing the blocking semantic and
	// ultimately back-pressure.  Note that we will "continue" if
	// the send is canceled by a subsequent send.
	for c.sendMsg == m && !expired && !c.closed {
		c.cond.Wait()
	}
	if c.sendMsg == m {
		c.unscheduleSend()
		c.sendMsg = nil
		c.reqID = 0
		if c.closed {
			return protocol.ErrClosed
		}
		return protocol.ErrSendTimeout
	}
	return nil
}

func (c *context) RecvMsg() (*protocol.Message, error) {
	s := c.s
	s.Lock()
	defer s.Unlock()
	if s.closed || c.closed {
		return nil, protocol.ErrClosed
	}
	if c.recvWait || c.reqID == 0 {
		return nil, protocol.ErrProtoState
	}
	c.recvWait = true
	id := c.reqID
	expired := false

	if c.recvExpire > 0 {
		c.recvTimer = time.AfterFunc(c.recvExpire, func() {
			s.Lock()
			if c.reqID == id {
				expired = true
				c.cancel()
			}
			s.Unlock()
		})
	}

	for id == c.reqID && c.repMsg == nil {
		c.cond.Wait()
	}

	m := c.repMsg
	c.reqID = 0
	c.repMsg = nil
	c.recvWait = false
	c.cond.Broadcast()

	if m == nil {
		if expired {
			return nil, protocol.ErrRecvTimeout
		}
		if c.closed {
			return nil, protocol.ErrClosed
		}
		return nil, protocol.ErrCanceled
	}
	return m, nil
}

func (c *context) SetOption(name string, value interface{}) error {
	switch name {
	case protocol.OptionRetryTime:
		if v, ok := value.(time.Duration); ok {
			c.s.Lock()
			c.resendTime = v
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

	case protocol.OptionSendDeadline:
		if v, ok := value.(time.Duration); ok {
			c.s.Lock()
			c.sendExpire = v
			c.s.Unlock()
			return nil
		}
		return protocol.ErrBadValue

	case protocol.OptionBestEffort:
		if v, ok := value.(bool); ok {
			c.s.Lock()
			c.bestEffort = v
			c.s.Unlock()
			return nil
		}
		return protocol.ErrBadValue
	}

	return protocol.ErrBadOption
}

func (c *context) GetOption(option string) (interface{}, error) {
	switch option {
	case protocol.OptionRetryTime:
		c.s.Lock()
		v := c.resendTime
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
	case protocol.OptionBestEffort:
		c.s.Lock()
		v := c.bestEffort
		c.s.Unlock()
		return v, nil
	}

	return nil, protocol.ErrBadOption
}

func (c *context) Close() error {
	s := c.s
	c.s.Lock()
	defer c.s.Unlock()
	if c.closed {
		return protocol.ErrClosed
	}
	c.closed = true
	c.cancel()
	delete(s.ctxs, c)
	return nil
}

func (s *socket) GetOption(option string) (interface{}, error) {
	switch option {
	case protocol.OptionRaw:
		return false, nil
	default:
		return s.defCtx.GetOption(option)
	}
}
func (s *socket) SetOption(option string, value interface{}) error {
	return s.defCtx.SetOption(option, value)
}

func (s *socket) SendMsg(m *protocol.Message) error {
	return s.defCtx.SendMsg(m)
}

func (s *socket) RecvMsg() (*protocol.Message, error) {
	return s.defCtx.RecvMsg()
}

func (s *socket) Close() error {
	s.Lock()

	if s.closed {
		s.Unlock()
		return protocol.ErrClosed
	}
	s.closed = true
	for c := range s.ctxs {
		c.closed = true
		c.cancel()
		delete(s.ctxs, c)
	}
	s.Unlock()
	return nil
}

func (s *socket) OpenContext() (protocol.Context, error) {
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return nil, protocol.ErrClosed
	}
	c := &context{
		s:          s,
		cond:       sync.NewCond(s),
		bestEffort: s.defCtx.bestEffort,
		resendTime: s.defCtx.resendTime,
		sendExpire: s.defCtx.sendExpire,
		recvExpire: s.defCtx.recvExpire,
	}
	s.ctxs[c] = struct{}{}
	return c, nil
}

func (s *socket) AddPipe(pp protocol.Pipe) error {
	p := &pipe{
		p: pp,
		s: s,
	}
	pp.SetPrivate(p)
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return protocol.ErrClosed
	}
	s.readyq = append(s.readyq, p)
	s.send()
	go p.receiver()
	return nil
}

func (s *socket) RemovePipe(pp protocol.Pipe) {
	p := pp.GetPrivate().(*pipe)
	s.Lock()
	p.closed = true
	for i, rp := range s.readyq {
		if p == rp {
			s.readyq = append(s.readyq[:i], s.readyq[i+1:]...)
		}
	}
	for c := range s.ctxs {
		if c.lastPipe == p && c.reqMsg != nil {
			// We are closing this pipe, so we need to
			// immediately reschedule it.
			c.lastPipe = nil
			c.unscheduleSend()
			go c.resendMessage(c.reqID)
		}
	}
	s.Unlock()
}

func (*socket) Info() protocol.Info {
	return protocol.Info{
		Self:     Self,
		Peer:     Peer,
		SelfName: SelfName,
		PeerName: PeerName,
	}
}

// NewProtocol allocates a new protocol implementation.
func NewProtocol() protocol.Protocol {
	s := &socket{
		nextID:  uint32(time.Now().UnixNano()), // quasi-random
		ctxs:    make(map[*context]struct{}),
		ctxByID: make(map[uint32]*context),
	}
	s.defCtx = &context{
		s:          s,
		cond:       sync.NewCond(s),
		resendTime: time.Minute,
	}
	s.ctxs[s.defCtx] = struct{}{}
	return s
}

// NewSocket allocates a new Socket using the REQ protocol.
func NewSocket() (protocol.Socket, error) {
	return protocol.MakeSocket(NewProtocol()), nil
}
