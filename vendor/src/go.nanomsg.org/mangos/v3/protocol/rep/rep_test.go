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

package rep

import (
	"encoding/binary"
	"sync"
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3"
	. "go.nanomsg.org/mangos/v3/internal/test"
	"go.nanomsg.org/mangos/v3/protocol/req"
	"go.nanomsg.org/mangos/v3/protocol/xrep"
	"go.nanomsg.org/mangos/v3/protocol/xreq"
	_ "go.nanomsg.org/mangos/v3/transport/inproc"
)

func TestRepIdentity(t *testing.T) {
	s := GetSocket(t, NewSocket)
	id := s.Info()
	MustBeTrue(t, id.Self == mangos.ProtoRep)
	MustBeTrue(t, id.SelfName == "rep")
	MustBeTrue(t, id.Peer == mangos.ProtoReq)
	MustBeTrue(t, id.PeerName == "req")
	MustSucceed(t, s.Close())
}

func TestRepCooked(t *testing.T) {
	VerifyCooked(t, NewSocket)
}

func TestRepOptions(t *testing.T) {
	VerifyInvalidOption(t, NewSocket)
	VerifyOptionQLen(t, NewSocket, mangos.OptionWriteQLen)
	VerifyOptionDuration(t, NewSocket, mangos.OptionRecvDeadline)
	VerifyOptionDuration(t, NewSocket, mangos.OptionSendDeadline)
	VerifyOptionBool(t, NewSocket, mangos.OptionBestEffort)
	VerifyOptionInt(t, NewSocket, mangos.OptionTTL)
}

func TestRepClosed(t *testing.T) {
	VerifyClosedRecv(t, NewSocket)
	VerifyClosedSend(t, NewSocket)
	VerifyClosedClose(t, NewSocket)
	VerifyClosedDial(t, NewSocket)
	VerifyClosedListen(t, NewSocket)
	VerifyClosedAddPipe(t, NewSocket)
	VerifyClosedContext(t, NewSocket)
}

func TestRepTTLZero(t *testing.T) {
	SetTTLZero(t, NewSocket)
}

func TestRepTTLNegative(t *testing.T) {
	SetTTLNegative(t, NewSocket)
}

func TestRepTTLTooBig(t *testing.T) {
	SetTTLTooBig(t, NewSocket)
}

func TestRepTTLSet(t *testing.T) {
	SetTTL(t, NewSocket)
}

func TestRepTTLDrop(t *testing.T) {
	TTLDropTest(t, req.NewSocket, NewSocket, xreq.NewSocket, xrep.NewSocket)
}

func TestRepCloseRecv(t *testing.T) {
	self := GetSocket(t, NewSocket)

	time.AfterFunc(time.Millisecond*20, func() {
		MustSucceed(t, self.Close())
	})
	MustNotRecv(t, self, mangos.ErrClosed)
}

func TestRepSendState(t *testing.T) {
	s := GetSocket(t, NewSocket)
	MustBeError(t, s.Send([]byte{}), mangos.ErrProtoState)
	MustSucceed(t, s.Close())
}

func TestRepBestEffortSend(t *testing.T) {
	s := GetSocket(t, NewSocket)
	p := GetSocket(t, xreq.NewSocket)
	MustSucceed(t, s.SetOption(mangos.OptionWriteQLen, 1))
	MustSucceed(t, p.SetOption(mangos.OptionReadQLen, 1))
	MustSucceed(t, s.SetOption(mangos.OptionBestEffort, true))

	ConnectPair(t, s, p)
	for i := 0; i < 100; i++ {
		// We have to make a raw message when using xreq.  We
		// use xreq because normal req will simply discard
		// messages for requests it doesn't have outstanding.
		m := mangos.NewMessage(0)
		m.Header = make([]byte, 4)
		binary.BigEndian.PutUint32(m.Header, uint32(i)|0x80000000)
		MustSucceed(t, p.SendMsg(m))
		m = MustRecvMsg(t, s)
		MustSucceed(t, s.SendMsg(m))
		// NB: We never ask the peer to receive it -- this ensures we
		// encounter back-pressure.
	}
	MustSucceed(t, s.Close())
	MustSucceed(t, p.Close())
}

// This verifies that closing the socket aborts a blocking send.
// We use a context because closing the socket also closes pipes
// making it less reproducible.
func TestRepCloseContextSend(t *testing.T) {
	self := GetSocket(t, NewSocket)
	peer := GetSocket(t, xreq.NewSocket)
	MustSucceed(t, self.SetOption(mangos.OptionWriteQLen, 1))
	MustSucceed(t, peer.SetOption(mangos.OptionReadQLen, 1))
	c, e := self.OpenContext()
	MustSucceed(t, e)

	ConnectPair(t, self, peer)

	MustSucceed(t, c.SetOption(mangos.OptionSendDeadline, time.Millisecond*10))

	time.Sleep(time.Millisecond * 10)
	cnt := 0
	data := []byte{0x80, 0, 0, 1}
	for i := 0; i < 100; i++ {
		// We have to make a raw message when using xreq.
		m := mangos.NewMessage(0)
		m.Header = append(m.Header, data...)
		data[3]++
		MustSucceed(t, peer.SendMsg(m))
		m, e := c.RecvMsg()
		MustSucceed(t, e)
		MustNotBeNil(t, m)
		if e = c.SendMsg(m); e != nil {
			MustBeError(t, e, mangos.ErrSendTimeout)
			break
		}
		cnt++

		// NB: We never ask the peer to receive it -- this ensures we
		// encounter back-pressure.
	}
	MustBeTrue(t, cnt > 0) // Some in-flight sends possible.
	MustBeTrue(t, cnt < 10)

	m := mangos.NewMessage(0)
	m.Header = append(m.Header, data...)
	data[3]++
	MustSucceed(t, peer.SendMsg(m))

	MustSucceed(t, c.SetOption(mangos.OptionSendDeadline, time.Minute))
	m, e = c.RecvMsg()
	MustSucceed(t, e)
	MustNotBeNil(t, m)
	time.AfterFunc(time.Millisecond*20, func() { MustSucceed(t, c.Close()) })
	MustBeError(t, c.SendMsg(m), mangos.ErrClosed)

	MustSucceed(t, peer.Close())
	MustSucceed(t, self.Close())
}

func TestRepRecvJunk(t *testing.T) {
	self := GetSocket(t, NewSocket)
	mock, _ := MockConnect(t, self)

	// Absent header...
	MockMustSendStr(t, mock, "", time.Second)

	// Absent request id... (must have bit 31 set)
	MockMustSend(t, mock, []byte{1, 2, 3, 4}, time.Second)

	time.Sleep(time.Millisecond * 10)
	MustSucceed(t, self.Close())
}

func TestRepDoubleRecv(t *testing.T) {
	self := GetSocket(t, NewSocket)
	MustSucceed(t, self.SetOption(mangos.OptionRecvDeadline, time.Second))
	pass := false
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(time.Millisecond)
		MustNotRecv(t, self, mangos.ErrProtoState)
		MustSucceed(t, self.Close())
		pass = true
	}()
	MustNotRecv(t, self, mangos.ErrClosed)
	wg.Wait()
	MustBeTrue(t, pass)
}

func TestRepClosedReply(t *testing.T) {
	self := GetSocket(t, NewSocket)
	peer := GetSocket(t, req.NewSocket)
	ConnectPair(t, self, peer)

	MustSendString(t, peer, "ping")
	MustRecvString(t, self, "ping")
	MustSucceed(t, peer.Close())
	time.Sleep(time.Millisecond * 20)
	MustSendString(t, self, "nack")
	time.Sleep(time.Millisecond * 20)
	MustSucceed(t, self.Close())
}

// This exercises the discard of inbound messages waiting, when the pipe
// or socket is closed.
func TestRepPipeRecvClosePipe(t *testing.T) {
	self := GetSocket(t, NewSocket)
	peer := GetSocket(t, req.NewSocket)
	MustSucceed(t, peer.SetOption(mangos.OptionSendDeadline, time.Millisecond*10))
	ConnectPair(t, self, peer)

	MustSendString(t, peer, "")
	m := MustRecvMsg(t, self)

	for i := 0; i < 10; i++ {
		e := peer.Send([]byte{})
		if e != nil {
			MustBeError(t, e, mangos.ErrSendTimeout)
			break
		}
	}

	MustSucceed(t, m.Pipe.Close())
	time.Sleep(time.Millisecond * 10)
	MustSucceed(t, peer.Close())
	MustSucceed(t, self.Close())
}

func TestRepPipeRecvCloseSocket(t *testing.T) {
	self := GetSocket(t, NewSocket)
	peer := GetSocket(t, req.NewSocket)
	MustSucceed(t, peer.SetOption(mangos.OptionSendDeadline, time.Millisecond*10))
	ConnectPair(t, self, peer)

	for i := 0; i < 10; i++ {
		e := peer.Send([]byte{})
		if e != nil {
			MustBeError(t, e, mangos.ErrSendTimeout)
			break
		}
	}

	MustSucceed(t, self.Close())
}

// This sets up a bunch of contexts to run in parallel, and verifies that
// they all seem to run with no mis-deliveries.
func TestRespondentMultiContexts(t *testing.T) {
	count := 30
	repeat := 20

	s := GetSocket(t, NewSocket)
	p := GetSocket(t, req.NewSocket)

	ConnectPair(t, p, s)

	recv := make([]int, count)
	send := make([]int, count)

	var wg1 sync.WaitGroup
	fn := func(c1, c2 mangos.Context, index int) {
		defer wg1.Done()

		topic := make([]byte, 4)
		binary.BigEndian.PutUint32(topic, uint32(index))

		for i := 0; i < repeat; i++ {
			MustSucceed(t, c2.Send(topic))
			m, e := c1.RecvMsg()
			MustSucceed(t, e)
			MustBeTrue(t, len(m.Body) == 4)
			peer := binary.BigEndian.Uint32(m.Body)
			recv[int(peer)]++
			MustSucceed(t, c1.Send([]byte("answer")))
			b, e := c2.Recv()
			MustSucceed(t, e)
			MustBeTrue(t, string(b) == "answer")
			send[index]++
		}
	}

	wg1.Add(count)

	for i := 0; i < count; i++ {
		c1, e := s.OpenContext()
		MustSucceed(t, e)
		MustNotBeNil(t, c1)

		c2, e := p.OpenContext()
		MustSucceed(t, e)
		MustNotBeNil(t, c2)

		MustSucceed(t, c1.SetOption(mangos.OptionRecvDeadline, time.Minute/4))
		MustSucceed(t, c1.SetOption(mangos.OptionSendDeadline, time.Minute/4))
		MustSucceed(t, c2.SetOption(mangos.OptionSendDeadline, time.Minute/4))
		MustSucceed(t, c2.SetOption(mangos.OptionRecvDeadline, time.Minute/4))

		go fn(c1, c2, i)
	}

	// Give time for everything to be delivered.
	wg1.Wait()
	MustSucceed(t, p.Close())
	MustSucceed(t, s.Close())

	for i := 0; i < count; i++ {
		MustBeTrue(t, recv[i] == repeat)
		MustBeTrue(t, send[i] == repeat)
	}
}
