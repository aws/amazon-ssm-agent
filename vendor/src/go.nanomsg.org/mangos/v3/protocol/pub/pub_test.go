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

package pub

import (
	"sync"
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3"
	. "go.nanomsg.org/mangos/v3/internal/test"
	"go.nanomsg.org/mangos/v3/protocol/sub"
	_ "go.nanomsg.org/mangos/v3/transport/inproc"
)

func TestPubIdentity(t *testing.T) {
	s, e := NewSocket()
	MustSucceed(t, e)
	id := s.Info()
	MustBeTrue(t, id.Self == mangos.ProtoPub)
	MustBeTrue(t, id.Peer == mangos.ProtoSub)
	MustBeTrue(t, id.SelfName == "pub")
	MustBeTrue(t, id.PeerName == "sub")
	MustSucceed(t, s.Close())
}

func TestPubCooked(t *testing.T) {
	VerifyCooked(t, NewSocket)
}

func TestPubNoRecv(t *testing.T) {
	CannotRecv(t, NewSocket)
}

func TestPubClosed(t *testing.T) {
	VerifyClosedSend(t, NewSocket)
	VerifyClosedClose(t, NewSocket)
	VerifyClosedDial(t, NewSocket)
	VerifyClosedListen(t, NewSocket)
}

func TestPubOptions(t *testing.T) {
	VerifyInvalidOption(t, NewSocket)
	// Note we can't test the actual impact yet, as we need to have
	// a way to block a sending pipe.
	VerifyOptionQLen(t, NewSocket, mangos.OptionWriteQLen)
}

func TestPubNonBlock(t *testing.T) {
	maxqlen := 2

	p, err := NewSocket()
	MustSucceed(t, err)
	MustNotBeNil(t, p)

	MustSucceed(t, p.SetOption(mangos.OptionWriteQLen, maxqlen))
	MustSucceed(t, p.Listen(AddrTestInp()))

	msg := []byte{'A', 'B', 'C'}

	for i := 0; i < maxqlen*10; i++ {
		MustSucceed(t, p.Send(msg))
	}
	MustSucceed(t, p.Close())
}

func TestPubBroadcast(t *testing.T) {
	p, err := NewSocket()
	MustSucceed(t, err)
	MustNotBeNil(t, p)

	s1, err := sub.NewSocket()
	MustSucceed(t, err)
	MustNotBeNil(t, s1)

	s2, err := sub.NewSocket()
	MustSucceed(t, err)
	MustNotBeNil(t, s2)

	a := AddrTestInp()
	MustSucceed(t, p.Listen(a))
	MustSucceed(t, s1.Dial(a))
	MustSucceed(t, s2.Dial(a))

	time.Sleep(time.Millisecond * 50)

	MustSucceed(t, s1.SetOption(mangos.OptionSubscribe, "topic1"))
	MustSucceed(t, s2.SetOption(mangos.OptionSubscribe, "topic2"))
	MustSucceed(t, s1.SetOption(mangos.OptionSubscribe, "both"))
	MustSucceed(t, s2.SetOption(mangos.OptionSubscribe, "both"))
	MustSucceed(t, s1.SetOption(mangos.OptionRecvDeadline, time.Millisecond*50))
	MustSucceed(t, s2.SetOption(mangos.OptionRecvDeadline, time.Millisecond*50))

	MustSucceed(t, p.Send([]byte("topic1one")))
	MustSucceed(t, p.Send([]byte("neither")))
	MustSucceed(t, p.Send([]byte("topic2two")))
	MustSucceed(t, p.Send([]byte("bothan")))
	MustSucceed(t, p.Send([]byte("topic2again")))
	MustSucceed(t, p.Send([]byte("topic1again")))
	MustSucceed(t, p.Send([]byte("garbage")))

	var wg sync.WaitGroup
	wg.Add(2)
	pass1 := false
	pass2 := false

	go func() { // Subscriber one
		defer wg.Done()
		v, e := s1.Recv()
		MustSucceed(t, e)
		MustBeTrue(t, string(v) == "topic1one")

		v, e = s1.Recv()
		MustSucceed(t, e)
		MustBeTrue(t, string(v) == "bothan")

		v, e = s1.Recv()
		MustSucceed(t, e)
		MustBeTrue(t, string(v) == "topic1again")

		v, e = s1.Recv()
		MustBeError(t, e, mangos.ErrRecvTimeout)
		MustBeNil(t, v)
		pass1 = true
	}()

	go func() { // Subscriber two
		defer wg.Done()
		v, e := s2.Recv()
		MustSucceed(t, e)
		MustBeTrue(t, string(v) == "topic2two")

		v, e = s2.Recv()
		MustSucceed(t, e)
		MustBeTrue(t, string(v) == "bothan")

		v, e = s2.Recv()
		MustSucceed(t, e)
		MustBeTrue(t, string(v) == "topic2again")

		v, e = s2.Recv()
		MustBeError(t, e, mangos.ErrRecvTimeout)
		MustBeNil(t, v)
		pass2 = true
	}()

	wg.Wait()

	MustBeTrue(t, pass1)
	MustBeTrue(t, pass2)

	MustSucceed(t, p.Close())
	MustSucceed(t, s1.Close())
	MustSucceed(t, s2.Close())
}
