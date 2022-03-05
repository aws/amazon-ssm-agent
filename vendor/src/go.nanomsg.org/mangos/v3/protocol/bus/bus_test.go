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

package bus

import (
	"encoding/binary"
	"math/rand"
	"sync"
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3"
	. "go.nanomsg.org/mangos/v3/internal/test"
	"go.nanomsg.org/mangos/v3/protocol/xbus"
	_ "go.nanomsg.org/mangos/v3/transport/inproc"
)

func TestBusIdentity(t *testing.T) {
	s := GetSocket(t, NewSocket)
	id := s.Info()
	MustBeTrue(t, id.Self == mangos.ProtoBus)
	MustBeTrue(t, id.Peer == mangos.ProtoBus)
	MustBeTrue(t, id.SelfName == "bus")
	MustBeTrue(t, id.PeerName == "bus")
	MustSucceed(t, s.Close())
}

func TestBusCooked(t *testing.T) {
	VerifyCooked(t, NewSocket)
}

func TestBusOptions(t *testing.T) {
	VerifyInvalidOption(t, NewSocket)
	VerifyOptionQLen(t, NewSocket, mangos.OptionWriteQLen)
}

func TestBusClosed(t *testing.T) {
	VerifyClosedSend(t, NewSocket)
	VerifyClosedRecv(t, NewSocket)
	VerifyClosedClose(t, NewSocket)
	VerifyClosedDial(t, NewSocket)
	VerifyClosedListen(t, NewSocket)
}

func TestBusDevice(t *testing.T) {
	hops := 25 // number of devices
	count := 10

	var socks []mangos.Socket

	head := GetSocket(t, NewSocket)
	tail := GetSocket(t, NewSocket)

	MustSucceed(t, head.SetOption(mangos.OptionRecvDeadline, time.Second))
	MustSucceed(t, tail.SetOption(mangos.OptionRecvDeadline, time.Second))

	socks = append(socks, head)
	for i := 0; i < hops; i++ {
		s1 := GetSocket(t, xbus.NewSocket)
		s2 := GetSocket(t, xbus.NewSocket)
		ConnectPair(t, socks[len(socks)-1], s1)
		socks = append(socks, s1, s2)
		go func() { _ = mangos.Device(s1, s2) }()
	}
	ConnectPair(t, socks[len(socks)-1], tail)
	socks = append(socks, tail)

	rng := rand.NewSource(32)
	for i := 0; i < count; i++ {
		var src, dst mangos.Socket
		if rng.Int63()&1 != 0 {
			src = head
			dst = tail
		} else {
			src = tail
			dst = head
		}
		msg := make([]byte, 4)
		val := uint32(rng.Int63())
		binary.BigEndian.PutUint32(msg, val)
		MustSucceed(t, src.Send(msg))
		res := MustRecv(t, dst)
		MustBeTrue(t, len(res) == 4)
		MustBeTrue(t, binary.BigEndian.Uint32(res) == val)
	}
	for _, s := range socks {
		MustSucceed(t, s.Close())
	}
}

func TestBusFanOut(t *testing.T) {
	count := 20
	nPeers := 100

	peers := make([]mangos.Socket, nPeers)
	self := GetSocket(t, NewSocket)
	MustSucceed(t, self.SetOption(mangos.OptionWriteQLen, count))

	for i := 0; i < nPeers; i++ {
		peers[i] = GetSocket(t, NewSocket)
		MustSucceed(t, peers[i].SetOption(mangos.OptionRecvDeadline, time.Second))
		ConnectPair(t, self, peers[i])
	}

	wg := sync.WaitGroup{}
	wg.Add(nPeers)

	pass := make([]bool, nPeers)
	for i := 0; i < nPeers; i++ {
		go func(index int) {
			defer wg.Done()
			s := peers[index]
			num := uint32(0)
			for j := 0; j < count; j++ {
				m := MustRecv(t, s)
				MustBeTrue(t, len(m) == 4)
				MustBeTrue(t, binary.BigEndian.Uint32(m) == num)
				num++
			}
			MustSucceed(t, s.SetOption(mangos.OptionRecvDeadline,
				time.Millisecond*10))
			_, e := s.Recv()
			MustBeError(t, e, mangos.ErrRecvTimeout)
			pass[index] = true
		}(i)
	}

	for i := 0; i < count; i++ {
		msg := make([]byte, 4)
		binary.BigEndian.PutUint32(msg, uint32(i))
		MustSucceed(t, self.Send(msg))
	}

	wg.Wait()
	for i := 0; i < count; i++ {
		MustBeTrue(t, pass[i])
		MustSucceed(t, peers[i].Close())
	}
	MustSucceed(t, self.Close())
}

func TestBusFanIn(t *testing.T) {
	count := 20
	nPeers := 100

	peers := make([]mangos.Socket, nPeers)
	self := GetSocket(t, NewSocket)
	MustSucceed(t, self.SetOption(mangos.OptionReadQLen, count*nPeers))
	MustSucceed(t, self.SetOption(mangos.OptionRecvDeadline, time.Second))

	for i := 0; i < nPeers; i++ {
		peers[i] = GetSocket(t, NewSocket)
		MustSucceed(t, self.SetOption(mangos.OptionWriteQLen, count))
		ConnectPair(t, self, peers[i])
	}

	wg := sync.WaitGroup{}
	wg.Add(nPeers)

	for i := 0; i < nPeers; i++ {
		go func(index int) {
			defer wg.Done()
			s := peers[index]
			msg := make([]byte, 8)
			binary.BigEndian.PutUint32(msg, uint32(index))
			num := uint32(0)
			for j := 0; j < count; j++ {
				binary.BigEndian.PutUint32(msg[4:], num)
				MustSucceed(t, s.Send(msg))
				num++
			}
		}(i)
	}

	counts := make([]uint32, nPeers)
	for i := 0; i < count*nPeers; i++ {
		m := MustRecv(t, self)
		MustBeTrue(t, len(m) == 8)
		index := binary.BigEndian.Uint32(m)
		num := binary.BigEndian.Uint32(m[4:])
		MustBeTrue(t, index < uint32(nPeers))
		MustBeTrue(t, num == counts[index])
		MustBeTrue(t, num < uint32(count))
		counts[index]++
	}
	MustSucceed(t, self.SetOption(mangos.OptionRecvDeadline,
		time.Millisecond*10))
	_, e := self.Recv()
	MustBeError(t, e, mangos.ErrRecvTimeout)

	wg.Wait()
	for i := 0; i < nPeers; i++ {
		MustBeTrue(t, counts[i] == uint32(count))
		MustSucceed(t, peers[i].Close())
	}
	MustSucceed(t, self.Close())
}

func TestBusDiscardHeader(t *testing.T) {
	self := GetSocket(t, NewSocket)
	peer := GetSocket(t, NewSocket)
	ConnectPair(t, self, peer)

	m := mangos.NewMessage(0)
	m.Header = append(m.Header, 0, 1, 2, 3)
	m.Body = append(m.Body, []byte("abc")...)
	MustSucceed(t, self.SendMsg(m))
	recv := MustRecvMsg(t, peer)
	MustBeTrue(t, len(recv.Header) == 0)
	MustBeTrue(t, string(recv.Body) == "abc")
	MustSucceed(t, self.Close())
	MustSucceed(t, peer.Close())
}
