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

package xpull

import (
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3"
	. "go.nanomsg.org/mangos/v3/internal/test"
	"go.nanomsg.org/mangos/v3/protocol/push"
	_ "go.nanomsg.org/mangos/v3/transport/inproc"
)

func TestXPullIdentity(t *testing.T) {
	s := GetSocket(t, NewSocket)
	id := s.Info()
	MustBeTrue(t, id.Self == mangos.ProtoPull)
	MustBeTrue(t, id.SelfName == "pull")
	MustBeTrue(t, id.Peer == mangos.ProtoPush)
	MustBeTrue(t, id.PeerName == "push")
	MustSucceed(t, s.Close())
}

func TestXPullRaw(t *testing.T) {
	VerifyRaw(t, NewSocket)
}

func TestXPullReadOnly(t *testing.T) {
	CannotSend(t, NewSocket)
}

func TestXPullClosed(t *testing.T) {
	VerifyClosedListen(t, NewSocket)
	VerifyClosedDial(t, NewSocket)
	VerifyClosedClose(t, NewSocket)
	VerifyClosedRecv(t, NewSocket)
	VerifyClosedAddPipe(t, NewSocket)
}

func TestXPullOptions(t *testing.T) {
	VerifyInvalidOption(t, NewSocket)
	VerifyOptionDuration(t, NewSocket, mangos.OptionRecvDeadline)
	VerifyOptionQLen(t, NewSocket, mangos.OptionReadQLen)

}

func TestXPullRecvDeadline(t *testing.T) {
	s := GetSocket(t, NewSocket)
	MustSucceed(t, s.SetOption(mangos.OptionRecvDeadline, time.Millisecond))
	d, e := s.Recv()
	MustBeError(t, e, mangos.ErrRecvTimeout)
	MustBeNil(t, d)
	MustSucceed(t, s.Close())
}

func TestXPullRecvClosePipe(t *testing.T) {
	s := GetSocket(t, NewSocket)
	p := GetSocket(t, push.NewSocket)
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

func TestXPullRecvCloseSocket(t *testing.T) {
	s := GetSocket(t, NewSocket)
	p := GetSocket(t, push.NewSocket)
	MustSucceed(t, p.SetOption(mangos.OptionWriteQLen, 1))
	MustSucceed(t, s.SetOption(mangos.OptionReadQLen, 3))
	MustSucceed(t, s.SetOption(mangos.OptionRecvDeadline, time.Minute))
	MustSucceed(t, p.SetOption(mangos.OptionSendDeadline, time.Millisecond))
	ConnectPair(t, s, p)

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
	MustSucceed(t, s.Close())
}

func TestXPullRecv(t *testing.T) {
	s := GetSocket(t, NewSocket)
	p := GetSocket(t, push.NewSocket)
	MustSucceed(t, s.SetOption(mangos.OptionRecvDeadline, time.Second))
	ConnectPair(t, s, p)

	for i := 0; i < 100; i++ {
		MustSucceed(t, p.Send([]byte{byte(i)}))
		v, e := s.Recv()
		MustSucceed(t, e)
		MustBeTrue(t, len(v) == 1)
		MustBeTrue(t, v[0] == byte(i))
	}
	MustSucceed(t, s.Close())
	MustSucceed(t, p.Close())
}

func TestXPullResizeRecv(t *testing.T) {
	s := GetSocket(t, NewSocket)
	p := GetSocket(t, push.NewSocket)
	MustSucceed(t, p.SetOption(mangos.OptionWriteQLen, 1))
	MustSucceed(t, s.SetOption(mangos.OptionReadQLen, 20))
	MustSucceed(t, s.SetOption(mangos.OptionRecvDeadline, time.Millisecond))
	ConnectPair(t, s, p)

	// Fill the pipe
	for i := 0; i < 20; i++ {
		// These all will work, but the back-pressure will go all the
		// way to the sender.
		MustSucceed(t, p.Send([]byte{byte(i)}))
	}

	time.Sleep(time.Millisecond * 10)
	MustSucceed(t, s.SetOption(mangos.OptionReadQLen, 1))
	// Sleep so the resize filler finishes
	time.Sleep(time.Millisecond * 20)

	for i := 0; i < 20; i++ {
		if _, e := s.Recv(); e != nil {
			MustBeError(t, e, mangos.ErrRecvTimeout)
			break
		}
	}
	MustSucceed(t, s.Close())
}

func TestXPullResizeRecv2(t *testing.T) {
	s := GetSocket(t, NewSocket)
	p := GetSocket(t, push.NewSocket)
	MustSucceed(t, p.SetOption(mangos.OptionWriteQLen, 1))
	MustSucceed(t, s.SetOption(mangos.OptionReadQLen, 3))
	MustSucceed(t, s.SetOption(mangos.OptionRecvDeadline, time.Millisecond))
	MustSucceed(t, p.SetOption(mangos.OptionSendDeadline, time.Millisecond))
	ConnectPair(t, s, p)

	// Fill the pipe
	for i := 0; i < 20; i++ {
		// These all will work, but the back-pressure will go all the
		// way to the sender.
		if e := p.Send([]byte{byte(i)}); e != nil {
			MustBeError(t, e, mangos.ErrSendTimeout)
			break
		}
	}

	MustSucceed(t, p.SetOption(mangos.OptionSendDeadline, time.Minute))
	go func() {
		MustSucceed(t, p.Send([]byte{'A'}))
	}()

	time.Sleep(time.Millisecond * 10)
	MustSucceed(t, s.SetOption(mangos.OptionReadQLen, 10))
	// Sleep so the resize filler finishes
	time.Sleep(time.Millisecond * 20)

	for i := 0; i < 20; i++ {
		if _, e := s.Recv(); e != nil {
			MustBeError(t, e, mangos.ErrRecvTimeout)
			break
		}
	}
	MustSucceed(t, s.Close())
}

func TestXPullResizeRecv3(t *testing.T) {
	s := GetSocket(t, NewSocket)
	p := GetSocket(t, push.NewSocket)
	MustSucceed(t, p.SetOption(mangos.OptionWriteQLen, 1))
	MustSucceed(t, s.SetOption(mangos.OptionReadQLen, 3))
	MustSucceed(t, s.SetOption(mangos.OptionRecvDeadline, time.Minute))
	MustSucceed(t, p.SetOption(mangos.OptionSendDeadline, time.Millisecond))
	MustSucceed(t, s.SetOption("_resizeDiscards", true))
	ConnectPair(t, s, p)

	time.AfterFunc(time.Millisecond*20, func() {
		MustSucceed(t, s.SetOption(mangos.OptionReadQLen, 10))
		time.Sleep(time.Millisecond * 10)
		MustSucceed(t, p.Send([]byte{}))
	})

	b, e := s.Recv()
	MustSucceed(t, e)
	MustNotBeNil(t, b)
	MustSucceed(t, s.Close())
}

func TestXPullResizeRecv4(t *testing.T) {
	s := GetSocket(t, NewSocket)
	p := GetSocket(t, push.NewSocket)
	MustSucceed(t, p.SetOption(mangos.OptionWriteQLen, 1))
	MustSucceed(t, s.SetOption(mangos.OptionReadQLen, 3))
	MustSucceed(t, s.SetOption(mangos.OptionRecvDeadline, time.Minute))
	MustSucceed(t, p.SetOption(mangos.OptionSendDeadline, time.Millisecond))
	MustSucceed(t, s.SetOption("_resizeDiscards", true))
	ConnectPair(t, s, p)

	// Fill the pipe
	for i := 0; i < 20; i++ {
		if e := p.Send([]byte{byte(i)}); e != nil {
			MustBeError(t, e, mangos.ErrSendTimeout)
			break
		}
	}

	time.AfterFunc(time.Millisecond*20, func() {
		MustSucceed(t, s.SetOption(mangos.OptionReadQLen, 10))
	})

	time.Sleep(time.Millisecond * 50)
	MustSucceed(t, s.Close())
}
