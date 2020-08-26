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

package xpush

import (
	"sync"
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3"
	. "go.nanomsg.org/mangos/v3/internal/test"
	"go.nanomsg.org/mangos/v3/protocol/pull"
	_ "go.nanomsg.org/mangos/v3/transport/inproc"
)

func TestXPushIdentity(t *testing.T) {
	s := GetSocket(t, NewSocket)
	id := s.Info()
	MustBeTrue(t, id.Self == mangos.ProtoPush)
	MustBeTrue(t, id.SelfName == "push")
	MustBeTrue(t, id.Peer == mangos.ProtoPull)
	MustBeTrue(t, id.PeerName == "pull")
	MustSucceed(t, s.Close())
}

func TestXPushRaw(t *testing.T) {
	VerifyRaw(t, NewSocket)
}

func TestXPushOptions(t *testing.T) {
	VerifyInvalidOption(t, NewSocket)
	VerifyOptionBool(t, NewSocket, mangos.OptionBestEffort)
	VerifyOptionDuration(t, NewSocket, mangos.OptionSendDeadline)
	VerifyOptionQLen(t, NewSocket, mangos.OptionWriteQLen)
}

func TestXPushNoRecv(t *testing.T) {
	CannotRecv(t, NewSocket)
}

func TestXPushClosed(t *testing.T) {
	VerifyClosedListen(t, NewSocket)
	VerifyClosedDial(t, NewSocket)
	VerifyClosedClose(t, NewSocket)
	VerifyClosedSend(t, NewSocket)
	VerifyClosedAddPipe(t, NewSocket)
}

func TestXPushSendBestEffort(t *testing.T) {
	s := GetSocket(t, NewSocket)
	MustSucceed(t, s.SetOption(mangos.OptionBestEffort, true))
	MustSucceed(t, s.SetOption(mangos.OptionWriteQLen, 1))
	for i := 0; i < 20; i++ {
		MustSucceed(t, s.Send([]byte{}))
	}
	MustSucceed(t, s.Close())
}

func TestXPushSendTimeout(t *testing.T) {
	s := GetSocket(t, NewSocket)
	MustSucceed(t, s.SetOption(mangos.OptionSendDeadline, time.Millisecond))
	MustSucceed(t, s.SetOption(mangos.OptionWriteQLen, 1))
	pass := false
	for i := 0; i < 20; i++ {
		if e := s.Send([]byte{}); e != nil {
			MustBeError(t, e, mangos.ErrSendTimeout)
			pass = true
			break
		}
	}
	MustBeTrue(t, pass)
	MustSucceed(t, s.Close())
}

func TestXPushSendResize(t *testing.T) {
	s := GetSocket(t, NewSocket)
	p := GetSocket(t, pull.NewSocket)
	MustSucceed(t, s.SetOption(mangos.OptionSendDeadline, time.Millisecond))
	MustSucceed(t, s.SetOption(mangos.OptionWriteQLen, 20))
	MustSucceed(t, p.SetOption(mangos.OptionRecvDeadline, time.Millisecond*20))
	for i := 0; i < 10; i++ {
		MustSucceed(t, s.Send([]byte{byte(i)}))
	}

	MustSucceed(t, s.SetOption(mangos.OptionWriteQLen, 3))

	// Now connect them so they can drain -- we should only have 3 messages
	// that arrive at the peer.
	ConnectPair(t, s, p)

	for i := 0; i < 3; i++ {
		m, e := p.Recv()
		MustSucceed(t, e)
		MustNotBeNil(t, m)
	}
	_, e := p.Recv()
	MustBeError(t, e, mangos.ErrRecvTimeout)
	MustSucceed(t, s.Close())
	MustSucceed(t, p.Close())
}

func TestXPushCloseAbort(t *testing.T) {
	s := GetSocket(t, NewSocket)
	MustSucceed(t, s.SetOption(mangos.OptionSendDeadline, time.Minute))
	MustSucceed(t, s.SetOption(mangos.OptionWriteQLen, 1))
	pass := false
	time.AfterFunc(time.Millisecond*10, func() {
		MustSucceed(t, s.Close())
	})
	for i := 0; i < 20; i++ {
		if e := s.Send([]byte{}); e != nil {
			MustBeError(t, e, mangos.ErrClosed)
			pass = true
			break
		}
	}
	MustBeTrue(t, pass)
}

// This tests that with nPeers, the load sharing will be "fair".  It can't
// verify ordering, because goroutines that return peers to the readyq can
// race against each other.
func TestXPushFairShare(t *testing.T) {
	count := 20
	nPeer := 5
	peers := make([]mangos.Socket, nPeer)
	s := GetSocket(t, NewSocket)

	for i := 0; i < nPeer; i++ {
		p := GetSocket(t, pull.NewSocket)
		MustSucceed(t, p.SetOption(mangos.OptionRecvDeadline, time.Second))
		MustSucceed(t, p.SetOption(mangos.OptionReadQLen, 0))
		ConnectPair(t, s, p)
		peers[i] = p
	}
	MustSucceed(t, s.SetOption(mangos.OptionWriteQLen, count*nPeer))

	for i := 0; i < count; i++ {
		for j := 0; j < nPeer; j++ {
			MustSucceed(t, s.Send([]byte{byte(j)}))
		}
	}

	for i := 0; i < count; i++ {
		for j := 0; j < nPeer; j++ {
			p := peers[j]
			m, e := p.Recv()
			MustSucceed(t, e)
			MustBeTrue(t, len(m) == 1)
			// We have to sleep a bit to let the go routine
			// that returns the readyq complete
			time.Sleep(time.Millisecond)
		}
	}
	for i := 0; i < nPeer; i++ {
		MustSucceed(t, peers[i].Close())
	}
	MustSucceed(t, s.Close())
}

func TestXPushRecvDiscard(t *testing.T) {
	s := GetSocket(t, NewSocket)
	l, mc := GetMockListener(t, s)
	mp := mc.NewPipe(s.Info().Peer)
	MustSucceed(t, l.Listen())
	MustSucceed(t, mc.AddPipe(mp))

	pass := true
outer:
	for i := 0; i < 10; i++ {
		m := mangos.NewMessage(0)
		m.Body = append(m.Body, 'a', 'b', 'c')
		select {
		case <-time.After(time.Second):
			pass = false
			break outer
		case mp.RecvQ() <- m:
		}
	}
	MustBeTrue(t, pass)
	MustSucceed(t, s.Close())
}

func TestXPushSendFail(t *testing.T) {
	s := GetSocket(t, NewSocket)
	l, mc := GetMockListener(t, s)
	mp := mc.NewPipe(s.Info().Peer)
	MustSucceed(t, l.Listen())
	nAdd := uint32(0)
	nRem := uint32(0)
	wg := sync.WaitGroup{}
	wg.Add(1)
	hook := func(ev mangos.PipeEvent, p mangos.Pipe) {
		switch ev {
		case mangos.PipeEventAttached:
			nAdd++
		case mangos.PipeEventDetached:
			nRem++
			wg.Done()
		}
	}
	s.SetPipeEventHook(hook)

	MustSucceed(t, mc.AddPipe(mp))
	mp.InjectSendError(mangos.ErrBadHeader)
	MustSucceed(t, s.Send([]byte{}))
	wg.Wait()
	MustSucceed(t, s.Close())
	MustBeTrue(t, nAdd == 1)
	MustBeTrue(t, nRem == 1)
	MustBeTrue(t, len(mp.SendQ()) == 0)
}
