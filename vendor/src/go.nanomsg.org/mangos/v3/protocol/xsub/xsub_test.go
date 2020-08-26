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

package xsub

import (
	"sync"
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3"
	. "go.nanomsg.org/mangos/v3/internal/test"
	"go.nanomsg.org/mangos/v3/protocol/pub"
	_ "go.nanomsg.org/mangos/v3/transport/inproc"
)

func TestXSubIdentity(t *testing.T) {
	s := GetSocket(t, NewSocket)
	id := s.Info()
	MustBeTrue(t, id.Self == mangos.ProtoSub)
	MustBeTrue(t, id.SelfName == "sub")
	MustBeTrue(t, id.Peer == mangos.ProtoPub)
	MustBeTrue(t, id.PeerName == "pub")
	MustSucceed(t, s.Close())
}

func TestXSubRaw(t *testing.T) {
	VerifyRaw(t, NewSocket)
}

func TestXSubClosed(t *testing.T) {
	VerifyClosedRecv(t, NewSocket)
	VerifyClosedClose(t, NewSocket)
	VerifyClosedDial(t, NewSocket)
	VerifyClosedListen(t, NewSocket)
	VerifyClosedAddPipe(t, NewSocket)
}

func TestXSubCannotSend(t *testing.T) {
	CannotSend(t, NewSocket)
}

func TestXSubCannotSubscribe(t *testing.T) {
	// Raw sockets cannot subscribe or unsubscribe.
	s, e := NewSocket()
	MustSucceed(t, e)
	e = s.SetOption(mangos.OptionSubscribe, []byte("topic"))
	MustFail(t, e)
	MustBeTrue(t, e == mangos.ErrBadOption)
	_ = s.Close()
}

func TestXSubCannotUnsubscribe(t *testing.T) {
	// Raw sockets cannot subscribe or unsubscribe.
	s, e := NewSocket()
	MustSucceed(t, e)
	e = s.SetOption(mangos.OptionUnsubscribe, []byte("topic"))
	MustFail(t, e)
	MustBeTrue(t, e == mangos.ErrBadOption)
	_ = s.Close()
}

func TestXSubRecvDeadline(t *testing.T) {
	s, e := NewSocket()
	MustSucceed(t, e)
	e = s.SetOption(mangos.OptionRecvDeadline, time.Millisecond)
	MustSucceed(t, e)
	m, e := s.RecvMsg()
	MustFail(t, e)
	MustBeTrue(t, e == mangos.ErrRecvTimeout)
	MustBeNil(t, m)
	_ = s.Close()
}

func TestXSubRecvClean(t *testing.T) {
	s, e := NewSocket()
	MustSucceed(t, e)
	p, e := pub.NewSocket()
	MustSucceed(t, e)
	addr := AddrTestInp()
	MustSucceed(t, s.Listen(addr))
	MustSucceed(t, p.Dial(addr))
	MustSucceed(t, s.SetOption(mangos.OptionRecvDeadline, time.Second))
	m := mangos.NewMessage(0)
	m.Body = append(m.Body, []byte("Hello world")...)
	e = p.SendMsg(m)
	MustSucceed(t, e)
	m, e = s.RecvMsg()
	MustSucceed(t, e)
	MustNotBeNil(t, m)
	MustBeTrue(t, string(m.Body) == "Hello world")
	_ = p.Close()
	_ = s.Close()
}

func TestXSubRecvQLen(t *testing.T) {
	s, e := NewSocket()
	MustSucceed(t, e)
	p, e := pub.NewSocket()
	MustSucceed(t, e)
	addr := AddrTestInp()
	MustSucceed(t, s.SetOption(mangos.OptionRecvDeadline, time.Millisecond*10))
	MustSucceed(t, s.SetOption(mangos.OptionReadQLen, 2))
	MustSucceed(t, s.Listen(addr))
	MustSucceed(t, p.Dial(addr))
	time.Sleep(time.Millisecond * 50)

	MustSucceed(t, p.Send([]byte("one")))
	MustSucceed(t, p.Send([]byte("two")))
	MustSucceed(t, p.Send([]byte("three")))
	time.Sleep(time.Millisecond * 50)

	MustSucceed(t, e)
	m, e := s.RecvMsg()
	MustSucceed(t, e)
	MustNotBeNil(t, m)
	m, e = s.RecvMsg()
	MustSucceed(t, e)
	MustNotBeNil(t, m)
	MustNotRecv(t, s, mangos.ErrRecvTimeout)
	MustClose(t, p)
	MustClose(t, s)
}

func TestXSubRecvQLenResize(t *testing.T) {
	s := GetSocket(t, NewSocket)
	p := GetSocket(t, pub.NewSocket)

	MustSucceed(t, s.SetOption(mangos.OptionRecvDeadline, time.Millisecond*20))
	MustSucceed(t, s.SetOption(mangos.OptionReadQLen, 4))
	MustSucceed(t, p.SetOption(mangos.OptionWriteQLen, 10))
	ConnectPair(t, s, p)
	time.Sleep(time.Millisecond * 50)
	MustSendString(t, p, "one")
	MustSendString(t, p, "two")
	MustSendString(t, p, "three")
	time.Sleep(time.Millisecond * 100)
	MustRecvString(t, s, "one")
	// Shrink it
	MustSucceed(t, s.SetOption(mangos.OptionReadQLen, 20))
	MustNotRecv(t, s, mangos.ErrRecvTimeout)

	MustSucceed(t, s.SetOption(mangos.OptionRecvDeadline, time.Second))

	// Now make sure it still works
	MustSendString(t, p, "four")
	MustRecvString(t, s, "four")

	// Now try a posted recv and asynchronous resize.
	var wg sync.WaitGroup
	wg.Add(1)
	pass := false
	go func() {
		defer wg.Done()
		time.Sleep(time.Millisecond * 20)
		MustSucceed(t, s.SetOption(mangos.OptionReadQLen, 5))
		MustSendString(t, p, "five")
		pass = true
	}()

	MustRecvString(t, s, "five")
	wg.Wait()
	MustSucceed(t, p.Close())
	MustSucceed(t, s.Close())
	MustBeTrue(t, pass)
}

func TestXSubOptions(t *testing.T) {
	VerifyInvalidOption(t, NewSocket)
	VerifyOptionDuration(t, NewSocket, mangos.OptionRecvDeadline)
	VerifyOptionInt(t, NewSocket, mangos.OptionReadQLen)
}

func TestXSubPoundPipes(t *testing.T) {
	self := GetSocket(t, NewSocket)
	var peers []mangos.Socket
	nPeers := 20
	repeat := 100
	var wg sync.WaitGroup
	wg.Add(nPeers)

	startQ := make(chan struct{})
	for i := 0; i < nPeers; i++ {
		peer := GetSocket(t, pub.NewSocket)
		peers = append(peers, peer)
		ConnectPair(t, self, peer)

		go func(s mangos.Socket) {
			defer wg.Done()
			<-startQ
			for j := 0; j < repeat; j++ {
				MustSendString(t, s, "yes")
			}
			time.Sleep(time.Millisecond * 10)
		}(peer)
	}
	close(startQ)
	wg.Wait()
	for _, peer := range peers {
		MustSucceed(t, peer.Close())
	}
	MustSucceed(t, self.Close())
}

func TestXSubPoundClose(t *testing.T) {
	self := GetSocket(t, NewSocket)
	var peers []mangos.Socket
	nPeers := 20
	var wg sync.WaitGroup
	wg.Add(nPeers)

	startQ := make(chan struct{})
	for i := 0; i < nPeers; i++ {
		peer := GetSocket(t, pub.NewSocket)
		peers = append(peers, peer)
		ConnectPair(t, self, peer)

		go func(s mangos.Socket) {
			defer wg.Done()
			<-startQ
			for {
				e := s.Send([]byte("yes"))
				if e != nil {
					MustBeError(t, e, mangos.ErrClosed)
					break
				}
			}
			time.Sleep(time.Millisecond * 10)
		}(peer)
	}
	close(startQ)
	time.Sleep(time.Millisecond * 10)
	MustSucceed(t, self.Close())
	time.Sleep(time.Millisecond * 100)
	for _, peer := range peers {
		MustSucceed(t, peer.Close())
	}
	wg.Wait()

}

func TestXSubPoundRecv(t *testing.T) {
	self := GetSocket(t, NewSocket)
	MustSucceed(t, self.SetOption(mangos.OptionReadQLen, 0))
	var peers []mangos.Socket
	nPeers := 20
	nReaders := 20
	var wg1 sync.WaitGroup
	var wg2 sync.WaitGroup
	wg1.Add(nPeers)
	wg2.Add(nReaders)

	for i := 0; i < nReaders; i++ {
		go func() {
			defer wg2.Done()
			for {
				_, e := self.RecvMsg()
				if e != nil {
					break
				}
			}
		}()
	}

	for i := 0; i < nPeers; i++ {
		peer := GetSocket(t, pub.NewSocket)
		peers = append(peers, peer)
		ConnectPair(t, self, peer)

		go func(s mangos.Socket) {
			defer wg1.Done()
			for {
				e := s.Send([]byte("yes"))
				if e != nil {
					break
				}
			}
			time.Sleep(time.Millisecond * 10)
		}(peer)

		// ramp up slowly
		time.Sleep(time.Millisecond)
	}
	time.Sleep(time.Millisecond * 10)

	for _, peer := range peers {
		MustSucceed(t, peer.Close())
	}
	MustSucceed(t, self.Close())
	wg1.Wait()
	wg2.Wait()
}

func TestXSubRecvNoQ(t *testing.T) {
	self := GetSocket(t, NewSocket)
	MustSucceed(t, self.SetOption(mangos.OptionReadQLen, 0))
	var peers []mangos.Socket
	nPeers := 20
	var wg1 sync.WaitGroup
	wg1.Add(nPeers)

	for i := 0; i < nPeers; i++ {
		peer := GetSocket(t, pub.NewSocket)
		peers = append(peers, peer)
		ConnectPair(t, self, peer)

		go func(s mangos.Socket) {
			defer wg1.Done()
			for {
				e := s.Send([]byte("yes"))
				if e != nil {
					break
				}
			}
			time.Sleep(time.Millisecond * 10)
		}(peer)

		// ramp up slowly
		time.Sleep(time.Millisecond)
	}
	time.Sleep(time.Millisecond * 10)

	for _, peer := range peers {
		MustSucceed(t, peer.Close())
	}
	MustSucceed(t, self.Close())
	wg1.Wait()
}
