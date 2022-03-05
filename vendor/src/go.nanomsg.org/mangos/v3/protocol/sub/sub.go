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

// Package sub implements the SUB protocol.  This protocol receives messages
// from publishers (PUB peers).  The messages are filtered based on
// subscription, such that only subscribed messages (see OptionSubscribe) are
// received.
//
// Note that in order to receive any messages, at least one subscription must
// be present.  If no subscription is present (the default state), receive
// operations will block forever.
package sub

import (
	"bytes"
	"sync"
	"time"

	"go.nanomsg.org/mangos/v3/protocol"
)

// Protocol identity information.
const (
	Self     = protocol.ProtoSub
	Peer     = protocol.ProtoPub
	SelfName = "sub"
	PeerName = "pub"
)

type socket struct {
	master *context
	ctxs   map[*context]struct{}
	closed bool
	sync.Mutex
}

type pipe struct {
	s *socket
	p protocol.Pipe
}

type context struct {
	recvQLen   int
	recvQ      chan *protocol.Message
	closeQ     chan struct{}
	sizeQ      chan struct{}
	recvExpire time.Duration
	closed     bool
	subs       [][]byte
	s          *socket
}

const defaultQLen = 128

func (*context) SendMsg(*protocol.Message) error {
	return protocol.ErrProtoOp
}

func (c *context) RecvMsg() (*protocol.Message, error) {

	s := c.s
	var timeQ <-chan time.Time
	s.Lock()
	recvQ := c.recvQ
	sizeQ := c.sizeQ
	closeQ := c.closeQ
	if c.recvExpire > 0 {
		timeQ = time.After(c.recvExpire)
	}
	s.Unlock()

	for {
		select {
		case <-timeQ:
			return nil, protocol.ErrRecvTimeout
		case <-closeQ:
			return nil, protocol.ErrClosed
		case <-sizeQ:
			s.Lock()
			sizeQ = c.sizeQ
			recvQ = c.recvQ
			s.Unlock()
			continue
		case m := <-recvQ:
			m = m.MakeUnique()
			return m, nil
		}
	}
}

func (c *context) Close() error {
	s := c.s
	s.Lock()
	if c.closed {
		s.Unlock()
		return protocol.ErrClosed
	}
	c.closed = true
	delete(s.ctxs, c)
	s.Unlock()
	close(c.closeQ)
	return nil
}

func (*socket) SendMsg(*protocol.Message) error {
	return protocol.ErrProtoOp
}

func (p *pipe) receiver() {
	s := p.s
	for {
		m := p.p.RecvMsg()
		if m == nil {
			break
		}
		s.Lock()
		for c := range s.ctxs {
			if c.matches(m) {
				// Matched, send it up.  Best effort.
				// As we are passing this to the user,
				// we need to ensure that the message
				// may be modified.
				m.Clone()
				select {
				case c.recvQ <- m:
				default:
					select {
					case m2 := <-c.recvQ:
						m2.Free()
					default:
					}
					// We have made room, and as we are
					// holding the lock, we are guaranteed
					// to be able to enqueue another
					// message. (No other pipe can
					// get in right now.)
					// NB: If we ever do work to break
					// up the locking, we will need to
					// revisit this.
					c.recvQ <- m
				}
			}
		}
		s.Unlock()
		m.Free()
	}

	p.close()
}

func (s *socket) AddPipe(pp protocol.Pipe) error {
	p := &pipe{
		p: pp,
		s: s,
	}
	s.Lock()
	defer s.Unlock()
	if s.closed {
		return protocol.ErrClosed
	}
	go p.receiver()
	return nil
}

func (s *socket) RemovePipe(protocol.Pipe) {
}

func (s *socket) Close() error {
	s.Lock()
	if s.closed {
		s.Unlock()
		return protocol.ErrClosed
	}
	ctxs := make([]*context, 0, len(s.ctxs))
	for c := range s.ctxs {
		ctxs = append(ctxs, c)
	}
	s.closed = true
	s.Unlock()
	for _, c := range ctxs {
		_ = c.Close()
	}
	return nil
}

func (p *pipe) close() {
	_ = p.p.Close()
}

func (c *context) matches(m *protocol.Message) bool {
	for _, sub := range c.subs {
		if bytes.HasPrefix(m.Body, sub) {
			return true
		}
	}
	return false

}

func (c *context) subscribe(topic []byte) error {
	for _, sub := range c.subs {
		if bytes.Equal(sub, topic) {
			// Already present
			return nil
		}
	}
	// We need a full data copy of our own.
	topic = append(make([]byte, 0, len(topic)), topic...)
	c.subs = append(c.subs, topic)
	return nil
}

func (c *context) unsubscribe(topic []byte) error {
	for i, sub := range c.subs {
		if !bytes.Equal(sub, topic) {
			continue
		}
		c.subs = append(c.subs[:i], c.subs[i+1:]...)

		// Because we have changed the subscription,
		// we may have messages in the channel that
		// we don't want any more.  Lets prune those.
		recvQ := make(chan *protocol.Message, c.recvQLen)
		sizeQ := make(chan struct{})
		recvQ, c.recvQ = c.recvQ, recvQ
		sizeQ, c.sizeQ = c.sizeQ, sizeQ
		close(sizeQ)
		for {
			select {
			case m := <-recvQ:
				if !c.matches(m) {
					m.Free()
					continue
				}
				// We're holding the lock, so nothing else
				// can contend for this (pipes must be
				// waiting) -- so this is guaranteed not to
				// block.
				c.recvQ <- m
			default:
				return nil
			}
		}
	}
	// Subscription not present
	return protocol.ErrBadValue
}

func (c *context) SetOption(name string, value interface{}) error {
	s := c.s

	var fn func([]byte) error

	switch name {
	case protocol.OptionReadQLen:
		if v, ok := value.(int); ok {
			recvQ := make(chan *protocol.Message, v)
			sizeQ := make(chan struct{})
			c.s.Lock()
			c.recvQ = recvQ
			sizeQ, c.sizeQ = c.sizeQ, sizeQ
			c.recvQ = recvQ
			c.recvQLen = v
			close(sizeQ)
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

	case protocol.OptionSubscribe:
		fn = c.subscribe
	case protocol.OptionUnsubscribe:
		fn = c.unsubscribe
	default:
		return protocol.ErrBadOption
	}

	var vb []byte

	switch v := value.(type) {
	case []byte:
		vb = v
	case string:
		vb = []byte(v)
	default:
		return protocol.ErrBadValue
	}

	s.Lock()
	defer s.Unlock()

	return fn(vb)
}

func (c *context) GetOption(name string) (interface{}, error) {
	switch name {
	case protocol.OptionReadQLen:
		c.s.Lock()
		v := c.recvQLen
		c.s.Unlock()
		return v, nil
	case protocol.OptionRecvDeadline:
		c.s.Lock()
		v := c.recvExpire
		c.s.Unlock()
		return v, nil
	}
	return nil, protocol.ErrBadOption
}

func (s *socket) RecvMsg() (*protocol.Message, error) {
	return s.master.RecvMsg()
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
		sizeQ:      make(chan struct{}),
		recvQ:      make(chan *protocol.Message, s.master.recvQLen),
		recvQLen:   s.master.recvQLen,
		recvExpire: s.master.recvExpire,
		subs:       [][]byte{},
	}
	s.ctxs[c] = struct{}{}
	return c, nil
}

func (s *socket) GetOption(name string) (interface{}, error) {
	switch name {
	case protocol.OptionRaw:
		return false, nil
	default:
		return s.master.GetOption(name)
	}
}

func (s *socket) SetOption(name string, val interface{}) error {
	return s.master.SetOption(name, val)
}

func (s *socket) Info() protocol.Info {
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
		ctxs: make(map[*context]struct{}),
	}
	s.master = &context{
		s:        s,
		recvQ:    make(chan *protocol.Message, defaultQLen),
		closeQ:   make(chan struct{}),
		sizeQ:    make(chan struct{}),
		recvQLen: defaultQLen,
	}
	s.ctxs[s.master] = struct{}{}
	return s
}

// NewSocket allocates a new Socket using the SUB protocol.
func NewSocket() (protocol.Socket, error) {
	return protocol.MakeSocket(NewProtocol()), nil
}
