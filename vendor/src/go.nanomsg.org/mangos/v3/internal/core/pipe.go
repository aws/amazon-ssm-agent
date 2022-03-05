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

package core

import (
	"crypto/rand"
	"encoding/binary"
	"sync"

	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/transport"
)

// This is an application-wide global ID allocator.  Unfortunately we need
// to have unique pipe IDs globally to permit certain things to work
// correctly.

type pipeIDAllocator struct {
	used map[uint32]struct{}
	next uint32
	lock sync.Mutex
}

func (p *pipeIDAllocator) Get() uint32 {
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.used == nil {
		b := make([]byte, 4)
		// The following could in theory fail, but in that case
		// we will wind up with IDs starting at zero.  It should
		// not happen unless the platform can't get good entropy.
		_, _ = rand.Read(b)
		p.used = make(map[uint32]struct{})
		p.next = binary.BigEndian.Uint32(b)
	}
	for {
		id := p.next & 0x7fffffff
		p.next++
		if id == 0 {
			continue
		}
		if _, ok := p.used[id]; ok {
			continue
		}
		p.used[id] = struct{}{}
		return id
	}
}

func (p *pipeIDAllocator) Free(id uint32) {
	p.lock.Lock()
	if _, ok := p.used[id]; !ok {
		panic("free of unused pipe ID")
	}
	delete(p.used, id)
	p.lock.Unlock()
}

var pipeIDs pipeIDAllocator

type pipeList struct {
	pipes map[uint32]*pipe
	lock  sync.Mutex
}

func (l *pipeList) Add(p *pipe) {
	l.lock.Lock()
	if l.pipes == nil {
		l.pipes = make(map[uint32]*pipe)
	}
	l.pipes[p.id] = p
	l.lock.Unlock()
}

func (l *pipeList) Remove(p *pipe) {
	l.lock.Lock()
	delete(l.pipes, p.id)
	l.lock.Unlock()
}

// CloseAll closes all pipes, asynchronously.
func (l *pipeList) CloseAll() {
	l.lock.Lock()
	for _, p := range l.pipes {
		go p.close()
	}
	l.lock.Unlock()
}

// pipe wraps the Pipe data structure with the stuff we need to keep
// for the core.  It implements the Pipe interface.
type pipe struct {
	id        uint32
	p         transport.Pipe
	l         *listener
	d         *dialer
	s         *socket
	closeOnce sync.Once
	data      interface{} // Protocol private
	added     bool
	closing   bool
	lock      sync.Mutex // held across calls to remPipe and addPipe
}

func newPipe(tp transport.Pipe, s *socket, d *dialer, l *listener) *pipe {
	p := &pipe{
		p:  tp,
		d:  d,
		l:  l,
		s:  s,
		id: pipeIDs.Get(),
	}
	return p
}

func (p *pipe) ID() uint32 {
	return p.id
}

func (p *pipe) close() {
	_ = p.Close()
}

func (p *pipe) Close() error {
	p.closeOnce.Do(func() {
		// Close the underlying transport pipe first.
		_ = p.p.Close()

		// Deregister it from the socket.  This will also arrange
		// for asynchronously running the event callback, and
		// releasing the pipe ID for reuse.
		p.lock.Lock()
		p.closing = true
		if p.added {
			p.s.remPipe(p)
		}
		p.lock.Unlock()

		if p.d != nil {
			// Inform the dialer so that it will redial.
			go p.d.pipeClosed()
		}
	})
	return nil
}

func (p *pipe) SendMsg(msg *mangos.Message) error {

	if err := p.p.Send(msg); err != nil {
		_ = p.Close()
		return err
	}
	return nil
}

func (p *pipe) RecvMsg() *mangos.Message {

	msg, err := p.p.Recv()
	if err != nil {
		_ = p.Close()
		return nil
	}
	msg.Pipe = p
	return msg
}

func (p *pipe) Address() string {
	switch {
	case p.l != nil:
		return p.l.Address()
	case p.d != nil:
		return p.d.Address()
	}
	return ""
}

func (p *pipe) GetOption(name string) (interface{}, error) {
	val, err := p.p.GetOption(name)
	if err == mangos.ErrBadOption {
		if p.d != nil {
			val, err = p.d.GetOption(name)
		} else if p.l != nil {
			val, err = p.l.GetOption(name)
		}
	}
	return val, err
}

func (p *pipe) Dialer() mangos.Dialer {
	if p.d == nil {
		return nil
	}
	return p.d
}

func (p *pipe) Listener() mangos.Listener {
	if p.l == nil {
		return nil
	}
	return p.l
}

func (p *pipe) SetPrivate(i interface{}) {
	p.data = i
}

func (p *pipe) GetPrivate() interface{} {
	return p.data
}
