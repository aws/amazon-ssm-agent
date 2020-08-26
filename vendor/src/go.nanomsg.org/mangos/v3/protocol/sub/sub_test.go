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

package sub

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3"
	. "go.nanomsg.org/mangos/v3/internal/test"
	. "go.nanomsg.org/mangos/v3/protocol"
	"go.nanomsg.org/mangos/v3/protocol/pub"
	_ "go.nanomsg.org/mangos/v3/transport/inproc"
)

func TestSubIdentity(t *testing.T) {
	id := GetSocket(t, NewSocket).Info()
	MustBeTrue(t, id.Self == ProtoSub)
	MustBeTrue(t, id.Peer == ProtoPub)
	MustBeTrue(t, id.SelfName == "sub")
	MustBeTrue(t, id.PeerName == "pub")
}

func TestSubCooked(t *testing.T) {
	VerifyCooked(t, NewSocket)
}

func TestSubNoSend(t *testing.T) {
	CannotSend(t, NewSocket)
}

func TestSubClosed(t *testing.T) {
	VerifyClosedRecv(t, NewSocket)
	VerifyClosedClose(t, NewSocket)
	VerifyClosedDial(t, NewSocket)
	VerifyClosedListen(t, NewSocket)
	VerifyClosedAddPipe(t, NewSocket)
	VerifyClosedContext(t, NewSocket)
}

func TestSubOptions(t *testing.T) {
	VerifyInvalidOption(t, NewSocket)
	VerifyOptionDuration(t, NewSocket, OptionRecvDeadline)
	VerifyOptionInt(t, NewSocket, OptionReadQLen)
}

func TestSubRecvDeadline(t *testing.T) {
	s := GetSocket(t, NewSocket)
	defer MustClose(t, s)
	MustSucceed(t, s.SetOption(OptionRecvDeadline, time.Millisecond))
	MustNotRecv(t, s, ErrRecvTimeout)
}

func TestSubSubscribe(t *testing.T) {
	s, e := NewSocket()
	MustSucceed(t, e)
	MustNotBeNil(t, s)
	MustSucceed(t, s.SetOption(OptionSubscribe, "topic"))
	MustSucceed(t, s.SetOption(OptionSubscribe, "topic"))
	MustSucceed(t, s.SetOption(OptionSubscribe, []byte{0, 1}))
	MustBeError(t, s.SetOption(OptionSubscribe, 1), ErrBadValue)

	MustBeError(t, s.SetOption(OptionUnsubscribe, "nope"), ErrBadValue)
	MustSucceed(t, s.SetOption(OptionUnsubscribe, "topic"))
	MustSucceed(t, s.SetOption(OptionUnsubscribe, []byte{0, 1}))
	MustBeError(t, s.SetOption(OptionUnsubscribe, false), ErrBadValue)
	MustSucceed(t, s.Close())
}

func TestSubUnsubscribeDrops(t *testing.T) {
	s, e := NewSocket()
	MustSucceed(t, e)
	p, e := pub.NewSocket()
	MustSucceed(t, e)
	a := AddrTestInp()

	MustSucceed(t, s.SetOption(OptionReadQLen, 50))
	MustSucceed(t, p.Listen(a))
	MustSucceed(t, s.Dial(a))

	time.Sleep(time.Millisecond * 20)

	MustSucceed(t, s.SetOption(OptionSubscribe, "1"))
	MustSucceed(t, s.SetOption(OptionSubscribe, "2"))

	for i := 0; i < 10; i++ {
		MustSucceed(t, p.Send([]byte("1")))
		MustSucceed(t, p.Send([]byte("2")))
	}

	time.Sleep(time.Millisecond * 10)
	MustSucceed(t, s.SetOption(OptionUnsubscribe, "1"))
	for i := 0; i < 10; i++ {
		v, e := s.Recv()
		MustSucceed(t, e)
		MustBeTrue(t, string(v) == "2")
	}

	MustSucceed(t, p.Close())
	MustSucceed(t, s.Close())
}

func TestSubUnsubscribeLoad(t *testing.T) {
	s := GetSocket(t, NewSocket)
	p := GetSocket(t, pub.NewSocket)
	MustSucceed(t, s.SetOption(OptionReadQLen, 1))
	MustSucceed(t, p.SetOption(OptionWriteQLen, 100))

	ConnectPair(t, s, p)
	ConnectPair(t, s, p)

	MustSucceed(t, s.SetOption(OptionSubscribe, "1"))
	MustSucceed(t, s.SetOption(OptionSubscribe, "2"))

	for i := 0; i < 10; i++ {
		MustSucceed(t, p.Send([]byte("1")))
		MustSucceed(t, p.Send([]byte("2")))
	}
	var wg sync.WaitGroup
	nPub := 3
	wg.Add(nPub)
	for i := 0; i < nPub; i++ {
		go func() {
			defer wg.Done()
			for {
				var err error
				if err = p.Send([]byte{'2'}); err == ErrClosed {
					return
				}
				MustSucceed(t, err)
			}
		}()
	}

	time.Sleep(time.Millisecond * 10)
	MustSucceed(t, s.SetOption(OptionUnsubscribe, "1"))
	time.Sleep(time.Millisecond * 50)
	for i := 0; i < 50; i++ {
		MustRecvString(t, s, "2")
	}

	MustSucceed(t, p.Close())
	MustSucceed(t, s.Close())
	wg.Wait()
}

func TestSubRecvQLen(t *testing.T) {
	s := GetSocket(t, NewSocket)
	defer MustClose(t, s)
	p := GetSocket(t, pub.NewSocket)
	defer MustClose(t, p)

	MustSucceed(t, s.SetOption(OptionRecvDeadline, time.Millisecond*10))
	MustSucceed(t, s.SetOption(OptionReadQLen, 2))
	MustSucceed(t, s.SetOption(OptionSubscribe, []byte{}))

	ConnectPair(t, s, p)
	time.Sleep(time.Millisecond * 50)

	MustSendString(t, p, "one")
	MustSendString(t, p, "two")
	MustSendString(t, p, "three")
	time.Sleep(time.Millisecond * 50)

	MustRecvString(t, s, "two")
	MustRecvString(t, s, "three")
	MustNotRecv(t, s, ErrRecvTimeout)
}

func TestSubRecvQLenResizeDiscard(t *testing.T) {
	s := GetSocket(t, NewSocket)
	p := GetSocket(t, pub.NewSocket)
	MustSucceed(t, s.SetOption(OptionRecvDeadline, time.Millisecond*100))
	MustSucceed(t, s.SetOption(OptionReadQLen, 10))
	MustSucceed(t, s.SetOption(OptionSubscribe, []byte{}))

	ConnectPair(t, s, p)

	MustSendString(t, p, "one")
	MustSendString(t, p, "two")
	MustSendString(t, p, "three")

	// Sleep allows the messages to arrive in the recvQ before we resize.
	time.Sleep(time.Millisecond * 50)

	// Resize it
	MustSucceed(t, s.SetOption(OptionReadQLen, 20))

	MustNotRecv(t, s, ErrRecvTimeout)
	MustSucceed(t, p.Close())
	MustSucceed(t, s.Close())
}

func TestSubRecvResizeContinue(t *testing.T) {
	s := GetSocket(t, NewSocket)
	defer MustClose(t, s)
	p := GetSocket(t, pub.NewSocket)
	defer MustClose(t, p)

	MustSucceed(t, s.SetOption(OptionRecvDeadline, time.Second*10))
	MustSucceed(t, s.SetOption(OptionReadQLen, 10))
	MustSucceed(t, s.SetOption(OptionSubscribe, []byte{}))

	ConnectPair(t, s, p)

	var wg sync.WaitGroup
	pass := false
	wg.Add(1)
	time.AfterFunc(time.Millisecond*50, func() {
		defer wg.Done()
		MustSucceed(t, s.SetOption(OptionReadQLen, 2))
		MustSendString(t, p, "ping")
		pass = true
	})

	MustRecvString(t, s, "ping")
	wg.Wait()
	MustBeTrue(t, pass)
}

func TestSubContextOpen(t *testing.T) {
	s, e := NewSocket()
	MustSucceed(t, e)
	c, e := s.OpenContext()
	MustSucceed(t, e)

	// Also test that we can't send on this.
	MustBeError(t, c.Send([]byte{}), ErrProtoOp)
	MustSucceed(t, c.Close())
	MustSucceed(t, s.Close())

	MustBeError(t, c.Close(), ErrClosed)
}

func TestSubSocketCloseContext(t *testing.T) {
	s, e := NewSocket()
	MustSucceed(t, e)
	c, e := s.OpenContext()
	MustSucceed(t, e)

	MustSucceed(t, s.Close())

	// Verify that the context is already closed (closing the socket
	// closes the context.)
	MustBeError(t, c.Close(), ErrClosed)
}

func TestSubMultiContexts(t *testing.T) {
	s, e := NewSocket()
	MustSucceed(t, e)
	MustNotBeNil(t, s)

	c1, e := s.OpenContext()
	MustSucceed(t, e)
	MustNotBeNil(t, c1)
	c2, e := s.OpenContext()
	MustSucceed(t, e)
	MustNotBeNil(t, c2)
	MustBeTrue(t, c1 != c2)

	MustSucceed(t, c1.SetOption(mangos.OptionSubscribe, "1"))
	MustSucceed(t, c2.SetOption(mangos.OptionSubscribe, "2"))
	MustSucceed(t, c1.SetOption(mangos.OptionSubscribe, "*"))
	MustSucceed(t, c2.SetOption(mangos.OptionSubscribe, "*"))

	p, e := pub.NewSocket()
	MustSucceed(t, e)
	MustNotBeNil(t, p)

	a := AddrTestInp()

	MustSucceed(t, p.Listen(a))
	MustSucceed(t, s.Dial(a))

	// Make sure we have dialed properly
	time.Sleep(time.Millisecond * 10)

	sent := []int{0, 0}
	recv := []int{0, 0}
	wildrecv := []int{0, 0}
	wildsent := 0
	mesg := []string{"1", "2"}
	var wg sync.WaitGroup
	wg.Add(2)
	fn := func(c mangos.Context, index int) {
		for {
			m, e := c.RecvMsg()
			if e == nil {
				switch string(m.Body) {
				case mesg[index]:
					recv[index]++
				case "*":
					wildrecv[index]++
				default:
					MustBeTrue(t, false)
				}
				continue
			}
			MustBeError(t, e, mangos.ErrClosed)
			wg.Done()
			return
		}
	}

	go fn(c1, 0)
	go fn(c2, 1)

	rng := rand.NewSource(32)

	// Choose an odd number so that it does not divide evenly, ensuring
	// that there will be a non-equal distribution.  Note that with our
	// fixed seed above, it works out to 41 & 60.
	for i := 0; i < 101; i++ {
		index := int(rng.Int63() & 1)
		if rng.Int63()&128 < 8 {
			MustSucceed(t, p.Send([]byte{'*'}))
			wildsent++
		} else {
			MustSucceed(t, p.Send([]byte(mesg[index])))
			sent[index]++
		}
	}

	// Give time for everything to be delivered.
	time.Sleep(time.Millisecond * 50)
	MustSucceed(t, c1.Close())
	MustSucceed(t, c2.Close())
	wg.Wait()

	MustBeTrue(t, sent[0] == recv[0])
	MustBeTrue(t, sent[1] == recv[1])
	MustBeTrue(t, wildsent == wildrecv[0])
	MustBeTrue(t, wildsent == wildrecv[1])

	MustSucceed(t, s.Close())
	MustSucceed(t, p.Close())
}
