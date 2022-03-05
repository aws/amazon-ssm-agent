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

package xpair

import (
	"sync/atomic"
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3"
	. "go.nanomsg.org/mangos/v3/internal/test"
	_ "go.nanomsg.org/mangos/v3/transport/inproc"
)

func TestXBusIdentity(t *testing.T) {
	s, err := NewSocket()
	MustSucceed(t, err)
	id := s.Info()
	MustBeTrue(t, id.Self == mangos.ProtoPair)
	MustBeTrue(t, id.SelfName == "pair")
	MustBeTrue(t, id.Peer == mangos.ProtoPair)
	MustBeTrue(t, id.PeerName == "pair")
	MustSucceed(t, s.Close())
}

func TestXPairRaw(t *testing.T) {
	VerifyRaw(t, NewSocket)
}

func TestXPairClosed(t *testing.T) {
	VerifyClosedRecv(t, NewSocket)
	VerifyClosedSend(t, NewSocket)
	VerifyClosedClose(t, NewSocket)
	VerifyClosedDial(t, NewSocket)
	VerifyClosedListen(t, NewSocket)
	VerifyClosedAddPipe(t, NewSocket)
}

func TestXPairOptions(t *testing.T) {
	VerifyInvalidOption(t, NewSocket)
	VerifyOptionDuration(t, NewSocket, mangos.OptionRecvDeadline)
	VerifyOptionDuration(t, NewSocket, mangos.OptionSendDeadline)
	VerifyOptionInt(t, NewSocket, mangos.OptionReadQLen)
	VerifyOptionInt(t, NewSocket, mangos.OptionWriteQLen)
	VerifyOptionBool(t, NewSocket, mangos.OptionBestEffort)
}

func TestXPairRecvDeadline(t *testing.T) {
	self := GetSocket(t, NewSocket)
	MustSucceed(t, self.SetOption(mangos.OptionRecvDeadline, time.Millisecond*10))
	MustNotRecv(t, self, mangos.ErrRecvTimeout)
	MustSucceed(t, self.Close())
}

func TestXPairSendDeadline(t *testing.T) {
	self := GetSocket(t, NewSocket)
	MustSucceed(t, self.SetOption(mangos.OptionWriteQLen, 0))
	MustSucceed(t, self.SetOption(mangos.OptionSendDeadline, time.Millisecond*10))
	MustBeError(t, self.Send([]byte{}), mangos.ErrSendTimeout)
	MustSucceed(t, self.Close())
}

func TestXPairSendBestEffort(t *testing.T) {
	self := GetSocket(t, NewSocket)
	MustSucceed(t, self.SetOption(mangos.OptionBestEffort, true))
	MustSucceed(t, self.SetOption(mangos.OptionWriteQLen, 0))
	MustSucceed(t, self.SetOption(mangos.OptionSendDeadline, time.Millisecond*10))
	for i := 0; i < 100; i++ {
		MustSendString(t, self, "yep")
	}
	MustSucceed(t, self.Close())
}

func TestXPairRejectSecondPipe(t *testing.T) {
	self := GetSocket(t, NewSocket)
	peer1 := GetSocket(t, NewSocket)
	peer2 := GetSocket(t, NewSocket)

	ConnectPair(t, self, peer1)
	a := AddrTestInp()
	MustSucceed(t, self.Listen(a))

	con := int32(0)
	add := int32(0)
	dis := int32(0)
	peer2.SetPipeEventHook(func(ev mangos.PipeEvent, p mangos.Pipe) {
		switch ev {
		case mangos.PipeEventAttaching:
			atomic.AddInt32(&con, 1)
		case mangos.PipeEventAttached:
			atomic.AddInt32(&add, 1)
		case mangos.PipeEventDetached:
			atomic.AddInt32(&dis, 1)
		}
	})
	MustSucceed(t, peer2.Dial(a))
	time.Sleep(time.Millisecond * 10)
	MustBeTrue(t, atomic.LoadInt32(&con) > 0)
	MustBeTrue(t, atomic.LoadInt32(&add) > 0)
	MustBeTrue(t, atomic.LoadInt32(&dis) > 0)
	MustSucceed(t, peer2.Close())
	MustSucceed(t, peer1.Close())
	MustSucceed(t, self.Close())
}

func TestXPairCloseAbort(t *testing.T) {
	self := GetSocket(t, NewSocket)
	MustSucceed(t, self.SetOption(mangos.OptionSendDeadline, time.Minute))
	MustSucceed(t, self.SetOption(mangos.OptionWriteQLen, 1))
	pass := false
	time.AfterFunc(time.Millisecond*10, func() {
		MustSucceed(t, self.Close())
	})
	for i := 0; i < 20; i++ {
		if e := self.Send([]byte{}); e != nil {
			MustBeError(t, e, mangos.ErrClosed)
			pass = true
			break
		}
	}
	MustBeTrue(t, pass)
}

func TestXPairClosePipe(t *testing.T) {
	s := GetSocket(t, NewSocket)
	p := GetSocket(t, NewSocket)
	MustSucceed(t, p.SetOption(mangos.OptionWriteQLen, 1))
	MustSucceed(t, s.SetOption(mangos.OptionReadQLen, 3))
	MustSucceed(t, s.SetOption(mangos.OptionRecvDeadline, time.Minute))
	ConnectPair(t, s, p)
	MustSucceed(t, p.Send([]byte{}))
	m, e := s.RecvMsg()
	MustSucceed(t, e)
	MustSucceed(t, p.SetOption(mangos.OptionSendDeadline, time.Millisecond))

	// Fill the pipe
	for i := 0; i < 20; i++ {
		// These all will work, but the back-pressure will go all the
		// way to the sender.
		if e := p.Send([]byte{byte(i)}); e != nil {
			MustBeError(t, e, mangos.ErrSendTimeout)
			break
		}
	}

	time.Sleep(time.Millisecond * 10)
	MustSucceed(t, m.Pipe.Close())

	time.Sleep(time.Millisecond * 10)
	MustSucceed(t, s.Close())
}

func TestXPairResizeRecv(t *testing.T) {
	self := GetSocket(t, NewSocket)
	peer := GetSocket(t, NewSocket)
	MustSucceed(t, peer.SetOption(mangos.OptionWriteQLen, 1))
	MustSucceed(t, self.SetOption(mangos.OptionReadQLen, 0))
	MustSucceed(t, self.SetOption(mangos.OptionRecvDeadline, time.Millisecond))
	ConnectPair(t, self, peer)

	MustSendString(t, peer, "one")
	time.Sleep(time.Millisecond * 10)
	MustSucceed(t, self.SetOption(mangos.OptionReadQLen, 2))
	MustNotRecv(t, self, mangos.ErrRecvTimeout)
	MustSucceed(t, self.Close())
	MustSucceed(t, peer.Close())
}

func TestXPairResizeRecv1(t *testing.T) {
	self := GetSocket(t, NewSocket)
	peer := GetSocket(t, NewSocket)
	MustSucceed(t, peer.SetOption(mangos.OptionWriteQLen, 1))
	MustSucceed(t, self.SetOption(mangos.OptionReadQLen, 0))
	MustSucceed(t, self.SetOption(mangos.OptionRecvDeadline, time.Second))
	ConnectPair(t, self, peer)

	time.AfterFunc(time.Millisecond*20, func() {
		MustSucceed(t, self.SetOption(mangos.OptionReadQLen, 2))
		MustSendString(t, peer, "one")
	})
	MustRecvString(t, self, "one")
	MustSucceed(t, self.Close())
	MustSucceed(t, peer.Close())
}

func TestXPairResizeRecv2(t *testing.T) {
	self := GetSocket(t, NewSocket)
	peer := GetSocket(t, NewSocket)
	MustSucceed(t, peer.SetOption(mangos.OptionWriteQLen, 0))
	MustSucceed(t, self.SetOption(mangos.OptionReadQLen, 20))
	MustSucceed(t, self.SetOption(mangos.OptionRecvDeadline, time.Millisecond))
	ConnectPair(t, self, peer)

	// Fill the pipe
	for i := 0; i < 20; i++ {
		MustSucceed(t, peer.Send([]byte{byte(i)}))
	}

	time.Sleep(time.Millisecond * 10)
	MustSucceed(t, self.SetOption(mangos.OptionReadQLen, 1))
	// Sleep so the resize filler finishes
	time.Sleep(time.Millisecond * 20)

	MustNotRecv(t, self, mangos.ErrRecvTimeout)
	MustSucceed(t, self.Close())
	MustSucceed(t, peer.Close())
}

func TestXPairResizeSend(t *testing.T) {
	self := GetSocket(t, NewSocket)
	_, _ = MockConnect(t, self)
	MustSucceed(t, self.SetOption(mangos.OptionWriteQLen, 0))
	MustSucceed(t, self.SetOption(mangos.OptionSendDeadline, time.Second))

	cq := make(chan struct{})
	time.AfterFunc(time.Millisecond*50, func() {
		defer func() { close(cq) }()
		MustSucceed(t, self.SetOption(mangos.OptionWriteQLen, 2))
	})
	MustSendString(t, self, "one")
	MustSendString(t, self, "two")
	<-cq
	MustSucceed(t, self.Close())
}
