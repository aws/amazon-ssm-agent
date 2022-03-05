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

package req

import (
	"bytes"
	"encoding/binary"
	"sync"
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3"
	. "go.nanomsg.org/mangos/v3/internal/test"
	"go.nanomsg.org/mangos/v3/protocol/rep"
	_ "go.nanomsg.org/mangos/v3/transport/inproc"
)

func TestReqIdentity(t *testing.T) {
	id := MustGetInfo(t, NewSocket)
	MustBeTrue(t, id.Self == mangos.ProtoReq)
	MustBeTrue(t, id.SelfName == "req")
	MustBeTrue(t, id.Peer == mangos.ProtoRep)
	MustBeTrue(t, id.PeerName == "rep")
}

func TestReqCooked(t *testing.T) {
	VerifyCooked(t, NewSocket)
}

func TestReqOptions(t *testing.T) {
	VerifyInvalidOption(t, NewSocket)
	VerifyOptionDuration(t, NewSocket, mangos.OptionRecvDeadline)
	VerifyOptionDuration(t, NewSocket, mangos.OptionSendDeadline)
	VerifyOptionDuration(t, NewSocket, mangos.OptionRetryTime)
	VerifyOptionBool(t, NewSocket, mangos.OptionBestEffort)
}

func TestReqClosed(t *testing.T) {
	VerifyClosedRecv(t, NewSocket)
	VerifyClosedSend(t, NewSocket)
	VerifyClosedClose(t, NewSocket)
	VerifyClosedDial(t, NewSocket)
	VerifyClosedListen(t, NewSocket)
	VerifyClosedAddPipe(t, NewSocket)
}

func TestReqRecvState(t *testing.T) {
	s := GetSocket(t, NewSocket)
	v, e := s.Recv()
	MustBeError(t, e, mangos.ErrProtoState)
	MustBeNil(t, v)
	MustSucceed(t, s.Close())
}

func TestReqRecvDeadline(t *testing.T) {
	self := GetSocket(t, NewSocket)
	peer := GetSocket(t, rep.NewSocket)
	ConnectPair(t, self, peer)
	MustSucceed(t, self.SetOption(mangos.OptionRecvDeadline, time.Millisecond))
	MustSucceed(t, self.Send([]byte{}))
	_ = MustRecv(t, peer)
	m, e := self.RecvMsg()
	MustBeError(t, e, mangos.ErrRecvTimeout)
	MustBeNil(t, m)
	MustSucceed(t, self.Close())
	MustSucceed(t, peer.Close())
}

func TestReqContextClosed(t *testing.T) {
	s := GetSocket(t, NewSocket)
	c, e := s.OpenContext()
	MustSucceed(t, e)
	MustNotBeNil(t, c)
	MustSucceed(t, c.Close())
	MustBeError(t, c.Close(), mangos.ErrClosed)

	c, e = s.OpenContext()
	MustSucceed(t, e)
	MustNotBeNil(t, c)

	MustSucceed(t, s.Close())

	MustBeError(t, c.Close(), mangos.ErrClosed)

	_, e = s.OpenContext()
	MustBeError(t, e, mangos.ErrClosed)
}

// This test demonstrates that sending a second survey cancels any Rx on the
// earlier outstanding ones.
func TestReqCancel(t *testing.T) {
	s := GetSocket(t, NewSocket)
	MustSucceed(t, s.SetOption(mangos.OptionRecvDeadline, time.Second))
	MustSucceed(t, s.SetOption(mangos.OptionBestEffort, true))
	MustSendString(t, s, "first")
	var wg sync.WaitGroup
	wg.Add(1)
	pass := false
	go func() {
		defer wg.Done()
		v, e := s.Recv()
		MustBeError(t, e, mangos.ErrCanceled)
		MustBeNil(t, v)
		pass = true
	}()
	time.Sleep(time.Millisecond * 50) // to allow go routine to run
	MustSendString(t, s, "second")
	wg.Wait()
	MustBeTrue(t, pass)
	MustSucceed(t, s.Close())
}

// This test demonstrates cancellation before calling receive but after the
// message is received causes the original message to be discarded.
func TestReqCancelReply(t *testing.T) {
	self := GetSocket(t, NewSocket)
	peer := GetSocket(t, rep.NewSocket)
	ConnectPair(t, self, peer)
	MustSucceed(t, self.SetOption(mangos.OptionRecvDeadline, time.Second))
	MustSucceed(t, peer.SetOption(mangos.OptionRecvDeadline, time.Second))

	MustSendString(t, self, "query1")
	MustRecvString(t, peer, "query1")
	MustSendString(t, peer, "reply1")
	// And we don't pick up the reply

	time.Sleep(time.Millisecond * 50)

	MustSendString(t, self, "query2")
	MustRecvString(t, peer, "query2")
	MustSendString(t, peer, "reply2")
	MustRecvString(t, self, "reply2")

	MustSucceed(t, self.Close())
	MustSucceed(t, peer.Close())
}

func TestReqBestEffort(t *testing.T) {
	timeout := time.Millisecond
	msg := []byte{'0', '1', '2', '3'}

	s := GetSocket(t, NewSocket)
	MustSucceed(t, s.SetOption(mangos.OptionSendDeadline, timeout))
	MustSucceed(t, s.Listen(AddrTestInp()))
	MustSucceed(t, s.SetOption(mangos.OptionBestEffort, true))
	MustSucceed(t, s.Send(msg))
	MustSucceed(t, s.Send(msg))
	MustSucceed(t, s.SetOption(mangos.OptionBestEffort, false))
	MustBeError(t, s.Send(msg), mangos.ErrSendTimeout)
	MustBeError(t, s.Send(msg), mangos.ErrSendTimeout)
}

// This test demonstrates cancellation before calling receive but after the
// message is received causes the original message to be discarded.
func TestReqRetry(t *testing.T) {
	self := GetSocket(t, NewSocket)
	peer := GetSocket(t, rep.NewSocket)
	ConnectPair(t, self, peer)

	MustSucceed(t, self.SetOption(mangos.OptionRetryTime, time.Millisecond*10))
	MustSucceed(t, self.SetOption(mangos.OptionRecvDeadline, time.Millisecond*10))
	MustSucceed(t, peer.SetOption(mangos.OptionRecvDeadline, time.Millisecond*200))

	start := time.Now()

	MustSendString(t, self, "query")
	MustRecvString(t, peer, "query")
	MustRecvString(t, peer, "query")
	MustSendString(t, peer, "reply")

	MustBeTrue(t, time.Since(start) < time.Second)
	MustNotRecv(t, peer, mangos.ErrRecvTimeout)
	MustBeTrue(t, time.Since(start) < time.Second)

	MustSucceed(t, self.Close())
	MustSucceed(t, peer.Close())
}

// This test repeats he retry at very frequent intervals.  The idea here is
// to demonstrate that there are multiple resend entries in the queue.
// This case covers github issue #179.
func TestReqRetryFast(t *testing.T) {
	self := GetSocket(t, NewSocket)
	peer := GetSocket(t, rep.NewSocket)
	ConnectPair(t, self, peer)

	mp, p := MockConnect(t, self)
	MustSucceed(t, self.SetOption(mangos.OptionRetryTime, time.Nanosecond))
	MustSucceed(t, self.SetOption(mangos.OptionRecvDeadline, time.Millisecond*10))
	MustSucceed(t, peer.SetOption(mangos.OptionRecvDeadline, time.Millisecond*200))

	start := time.Now()

	MustSendString(t, self, "query")
	MockMustRecvStr(t, mp, "query", time.Second)
	MockMustRecvStr(t, mp, "query", time.Second)
	time.Sleep(time.Millisecond * 10)
	MustSucceed(t, p.Close())

	ConnectPair(t, self, peer)
	MustRecvString(t, peer, "query")
	MustRecvString(t, peer, "query")
	MustSendString(t, peer, "reply")

	MustBeTrue(t, time.Since(start) < time.Second)
	MustBeTrue(t, time.Since(start) < time.Second)

	MustSucceed(t, self.Close())
	MustSucceed(t, peer.Close())
}

func TestReqRetryLateConnect(t *testing.T) {
	self := GetSocket(t, NewSocket)
	peer := GetSocket(t, rep.NewSocket)

	MustSucceed(t, self.SetOption(mangos.OptionReconnectTime,
		time.Millisecond*100))

	MustSucceed(t, self.SetOption(mangos.OptionBestEffort, true))
	MustSendString(t, self, "hello")

	ConnectPair(t, self, peer)

	MustRecvString(t, peer, "hello")
	MustSendString(t, peer, "world")

	MustRecvString(t, self, "world")

	MustSucceed(t, self.Close())
	MustSucceed(t, peer.Close())
}

func TestReqRetryReconnect(t *testing.T) {
	self := GetSocket(t, NewSocket)
	peer1 := GetSocket(t, rep.NewSocket)
	peer2 := GetSocket(t, rep.NewSocket)

	MustSucceed(t, self.SetOption(mangos.OptionReconnectTime, time.Second))
	MustSucceed(t, self.SetOption(mangos.OptionRetryTime, time.Second*10))

	ConnectPair(t, self, peer1)

	start := time.Now()

	MustSendString(t, self, "ping")
	MustRecvString(t, peer1, "ping")

	MustSucceed(t, peer1.Close())
	time.Sleep(time.Millisecond * 10)

	ConnectPair(t, self, peer2)
	MustRecvString(t, peer2, "ping")
	MustSendString(t, peer2, "pong")
	MustRecvString(t, self, "pong")

	// Reconnect needs to happen immediately, and start a timely reply.
	MustBeTrue(t, time.Since(start) < time.Second)

	MustSucceed(t, self.Close())
	MustSucceed(t, peer2.Close())
}

// This demonstrates receiving a frame with garbage data.
func TestReqRecvGarbage(t *testing.T) {
	self := GetSocket(t, NewSocket)
	mock, pipe := MockConnect(t, self)
	expire := time.Millisecond * 10

	MustSucceed(t, self.SetOption(mangos.OptionRecvDeadline, expire))
	MustSucceed(t, self.SetOption(mangos.OptionBestEffort, true))

	MustSendString(t, self, "")
	MockMustSendStr(t, mock, "abc", expire)
	MustNotRecv(t, self, mangos.ErrRecvTimeout)

	var msg []byte
	// No header
	MustSendString(t, self, "")
	MockMustSend(t, mock, msg, expire)
	MustNotRecv(t, self, mangos.ErrRecvTimeout)

	// No request ID
	MustSendString(t, self, "")
	msg = append(msg, 0, 1, 2, 3)
	MockMustSend(t, mock, msg, expire)
	MustNotRecv(t, self, mangos.ErrRecvTimeout)

	// Incorrect pipe ID
	MustSendString(t, self, "")
	binary.BigEndian.PutUint32(msg, pipe.ID()^0xff)
	msg = append(msg, 0x80, 4, 3, 2)
	MockMustSend(t, mock, msg, expire)
	MustNotRecv(t, self, mangos.ErrRecvTimeout)

	// Also send a bogus header -- no request ID
	MustSendString(t, self, "")
	MockMustSendStr(t, mock, "\001\002\003\004", time.Millisecond*10)
	MustNotRecv(t, self, mangos.ErrRecvTimeout)

	MustSucceed(t, self.Close())
}

func TestReqCtxCloseSend(t *testing.T) {
	self := GetSocket(t, NewSocket)
	_, _ = MockConnect(t, self)

	c, e := self.OpenContext()
	MustSucceed(t, e)

	// This gets something on the pipe.
	MustSendString(t, self, "")
	var wg sync.WaitGroup
	wg.Add(1)

	time.AfterFunc(time.Millisecond*10, func() {
		MustSucceed(t, c.Close())
		wg.Done()
	})
	MustBeError(t, c.Send([]byte{}), mangos.ErrClosed)
	wg.Wait()

	MustSucceed(t, self.Close())
}

func TestReqCtxCloseRecv(t *testing.T) {
	self := GetSocket(t, NewSocket)
	peer := GetSocket(t, rep.NewSocket)

	ConnectPair(t, self, peer)
	c, e := self.OpenContext()
	MustSucceed(t, e)

	MustSucceed(t, c.Send([]byte{}))
	MustSucceed(t, self.SetOption(mangos.OptionRecvDeadline, time.Second))

	var wg sync.WaitGroup
	wg.Add(1)

	time.AfterFunc(time.Millisecond*20, func() {
		MustSucceed(t, c.Close())
		wg.Done()
	})
	b, e := c.Recv()
	MustBeError(t, e, mangos.ErrClosed)
	MustBeNil(t, b)
	wg.Wait()
	MustSucceed(t, self.Close())
	MustSucceed(t, peer.Close())
}

// This sets up a bunch of contexts to run in parallel, and verifies that
// they all seem to run with no mis-deliveries.
func TestReqMultiContexts(t *testing.T) {
	count := 30
	repeat := 20

	self := GetSocket(t, NewSocket)
	peer := GetSocket(t, rep.NewSocket)

	ConnectPair(t, self, peer)

	recv := make([]int, count)
	ctxs := make([]mangos.Context, 0, count)
	var wg1 sync.WaitGroup
	var wg2 sync.WaitGroup
	wg1.Add(count)
	fn := func(index int) {
		defer wg1.Done()
		c, e := self.OpenContext()
		MustSucceed(t, e)
		MustNotBeNil(t, c)

		ctxs = append(ctxs, c)
		topic := make([]byte, 4)
		binary.BigEndian.PutUint32(topic, uint32(index))

		MustSucceed(t, c.SetOption(mangos.OptionRecvDeadline, time.Second))

		for i := 0; i < repeat; i++ {
			MustSucceed(t, c.Send(topic))
			m, e := c.RecvMsg()
			MustSucceed(t, e)
			MustBeTrue(t, len(m.Body) == 4)
			MustBeTrue(t, bytes.Equal(m.Body, topic))
			recv[index]++
		}
	}

	for i := 0; i < count; i++ {
		go fn(i)
	}

	wg2.Add(1)
	go func() {
		defer wg2.Done()
		var e error
		var m *mangos.Message
		for {
			if m, e = peer.RecvMsg(); e != nil {
				break
			}
			if e = peer.SendMsg(m); e != nil {
				break
			}
		}
		MustBeError(t, e, mangos.ErrClosed)
	}()

	// Give time for everything to be delivered.
	wg1.Wait()
	MustSucceed(t, peer.Close())
	wg2.Wait()
	MustSucceed(t, self.Close())

	for i := 0; i < count; i++ {
		MustBeTrue(t, recv[i] == repeat)
	}
}
