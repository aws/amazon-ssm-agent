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

package xstar

import (
	"encoding/binary"
	"sync"
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3"
	. "go.nanomsg.org/mangos/v3/internal/test"
	. "go.nanomsg.org/mangos/v3/protocol"
	_ "go.nanomsg.org/mangos/v3/transport/inproc"
)

func TestXStarIdentity(t *testing.T) {
	id := MustGetInfo(t, NewSocket)
	MustBeTrue(t, id.Self == ProtoStar)
	MustBeTrue(t, id.SelfName == "star")
	MustBeTrue(t, id.Peer == ProtoStar)
	MustBeTrue(t, id.PeerName == "star")
}

func TestXStarRaw(t *testing.T) {
	VerifyRaw(t, NewSocket)
}

func TestXStarClosed(t *testing.T) {
	VerifyClosedRecv(t, NewSocket)
	VerifyClosedSend(t, NewSocket)
	VerifyClosedClose(t, NewSocket)
	VerifyClosedDial(t, NewSocket)
	VerifyClosedListen(t, NewSocket)
	VerifyClosedAddPipe(t, NewSocket)
}

func TestXStarOptions(t *testing.T) {
	VerifyInvalidOption(t, NewSocket)
	VerifyOptionDuration(t, NewSocket, OptionRecvDeadline)
	VerifyOptionInt(t, NewSocket, OptionReadQLen)
	VerifyOptionInt(t, NewSocket, OptionWriteQLen)
	VerifyOptionInt(t, NewSocket, OptionTTL)
}

func TestXStarRecvDeadline(t *testing.T) {
	self := GetSocket(t, NewSocket)
	MustSucceed(t, self.SetOption(OptionRecvDeadline, time.Millisecond))
	MustNotRecv(t, self, ErrRecvTimeout)
	MustClose(t, self)
}

func TestXStarNoHeader(t *testing.T) {
	s, err := NewSocket()
	MustSucceed(t, err)
	defer MustClose(t, s)

	m := mangos.NewMessage(0)

	MustSucceed(t, s.SendMsg(m))
}

func TestXStarTTL(t *testing.T) {
	SetTTLZero(t, NewSocket)
	SetTTLNegative(t, NewSocket)
	SetTTLTooBig(t, NewSocket)
	SetTTLNotInt(t, NewSocket)
	SetTTL(t, NewSocket)
}

func TestXStarNonBlock(t *testing.T) {

	self := GetSocket(t, NewSocket)
	defer MustClose(t, self)

	MustSucceed(t, self.SetOption(OptionWriteQLen, 2))
	MustSucceed(t, self.Listen(AddrTestInp()))

	start := time.Now()
	for i := 0; i < 100; i++ {
		MustSendString(t, self, "abc")
	}
	end := time.Now()
	MustBeTrue(t, end.Sub(start) < time.Second/10)
}

func TestXStarSendDrop(t *testing.T) {
	self := GetSocket(t, NewSocket)
	MustSucceed(t, self.SetOption(OptionWriteQLen, 1))
	l, ml := GetMockListener(t, self)
	MustSucceed(t, l.Listen())
	mp := ml.NewPipe(self.Info().Peer)
	MustSucceed(t, ml.AddPipe(mp))

	for i := 0; i < 100; i++ {
		MustSucceed(t, self.Send([]byte{byte(i)}))
	}
	time.Sleep(time.Millisecond * 10)
	MustSucceed(t, self.Close())
}

func TestXStarRecvNoHeader(t *testing.T) {
	self := GetSocket(t, NewSocket)
	mock, _ := MockConnect(t, self)

	MustSucceed(t, self.SetOption(OptionReadQLen, 2))
	MustSucceed(t, self.SetOption(OptionRecvDeadline, time.Millisecond*50))
	MockMustSend(t, mock, []byte{}, time.Second)
	MustNotRecv(t, self, ErrRecvTimeout)
	MustSucceed(t, self.Close())
}

func TestXStarRecvHeader(t *testing.T) {
	self := GetSocket(t, NewSocket)
	mock, _ := MockConnect(t, self)

	// First byte is non-zero
	MockMustSendMsg(t, mock, newMessage(0x1000000, "1"), time.Second)
	// Second byte is non-zero
	MockMustSendMsg(t, mock, newMessage(0x10000, "2"), time.Second)
	// Third byte is non-zero
	MockMustSendMsg(t, mock, newMessage(0x100, "3"), time.Second)
	// Fourth gets through
	MockMustSendMsg(t, mock, newMessage(0x1, "4"), time.Second)

	m := MustRecvMsg(t, self)
	MustBeTrue(t, string(m.Body) == "4")
	MustBeTrue(t, len(m.Header) == 4)

	// The incoming hop count gets incremented on receive.
	MustBeTrue(t, binary.BigEndian.Uint32(m.Header) == 2)
	MustClose(t, self)
}

func TestXStarSendRecv(t *testing.T) {
	s1 := GetSocket(t, NewSocket)
	defer MustClose(t, s1)
	s2 := GetSocket(t, NewSocket)
	defer MustClose(t, s2)

	ConnectPair(t, s1, s2)

	m := mangos.NewMessage(0)

	// Write an empty hop count to the header
	m.Header = append(m.Header, 0, 0, 0, 0)
	m.Body = append(m.Body, []byte("test")...)

	MustSendMsg(t, s1, m)
	m2 := MustRecvMsg(t, s2)

	MustBeTrue(t, len(m2.Header) == 4)
	MustBeTrue(t, binary.BigEndian.Uint32(m2.Header) == 1)
	MustBeTrue(t, string(m2.Body) == "test")
}

func TestXStarRedistribute(t *testing.T) {
	s1 := GetSocket(t, NewSocket)
	defer MustClose(t, s1)
	s2 := GetSocket(t, NewSocket)
	defer MustClose(t, s2)
	s3 := GetSocket(t, NewSocket)

	MustSucceed(t, s1.SetOption(OptionRecvDeadline, time.Millisecond*100))
	MustSucceed(t, s2.SetOption(OptionRecvDeadline, time.Millisecond*100))
	MustSucceed(t, s3.SetOption(OptionRecvDeadline, time.Millisecond*100))
	MustSucceed(t, s1.SetOption(OptionReadQLen, 5))
	MustSucceed(t, s2.SetOption(OptionReadQLen, 5))
	MustSucceed(t, s3.SetOption(OptionReadQLen, 5))

	ConnectPair(t, s1, s2)
	ConnectPair(t, s3, s2)

	m := mangos.NewMessage(0)

	// Write an empty hop count to the header
	m.Header = append(m.Header, 0, 0, 0, 0)
	m.Body = append(m.Body, []byte("test")...)

	MustSendMsg(t, s1, m)
	m2 := MustRecvMsg(t, s3)

	MustBeTrue(t, len(m2.Header) == 4)
	// We go through two hops to get here.
	MustBeTrue(t, binary.BigEndian.Uint32(m2.Header) == 2)
	MustBeTrue(t, string(m2.Body) == "test")

	MustNotRecv(t, s1, mangos.ErrRecvTimeout)
}

func TestXStarRedistributeBackPressure(t *testing.T) {
	s1 := GetSocket(t, NewSocket)
	defer MustClose(t, s1)
	s2 := GetSocket(t, NewSocket)
	defer MustClose(t, s2)
	s3 := GetSocket(t, NewSocket)

	MustSucceed(t, s1.SetOption(OptionRecvDeadline, time.Millisecond*100))
	MustSucceed(t, s2.SetOption(OptionRecvDeadline, time.Millisecond*100))
	MustSucceed(t, s3.SetOption(OptionRecvDeadline, time.Millisecond*100))
	MustSucceed(t, s1.SetOption(OptionReadQLen, 5))
	MustSucceed(t, s2.SetOption(OptionReadQLen, 5))
	MustSucceed(t, s3.SetOption(OptionReadQLen, 5))
	MustSucceed(t, s1.SetOption(OptionWriteQLen, 2))
	MustSucceed(t, s3.SetOption(OptionWriteQLen, 2))
	// Make the intermediate queue size too small to force back-pressure.
	MustSucceed(t, s2.SetOption(OptionWriteQLen, 1))

	ConnectPair(t, s1, s2)
	ConnectPair(t, s3, s2)

	for i := 0; i < 10; i++ {
		m := mangos.NewMessage(0)

		// Write an empty hop count to the header
		m.Header = append(m.Header, 0, 0, 0, 0)
		m.Body = append(m.Body, []byte("test")...)

		MustSendMsg(t, s1, m)
	}

	m2 := MustRecvMsg(t, s3)

	MustBeTrue(t, len(m2.Header) == 4)
	// We go through two hops to get here.
	MustBeTrue(t, binary.BigEndian.Uint32(m2.Header) == 2)
	MustBeTrue(t, string(m2.Body) == "test")

	MustNotRecv(t, s1, ErrRecvTimeout)
}

// newMessage creates a message as it would come from a pipe.  The
// hops will be part of the body.
func newMessage(hops uint32, content string) *mangos.Message {
	m := mangos.NewMessage(len(content) + 4)
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b[:4], hops)
	// Requests (coming in) will be entirely on the body.
	m.Body = append(m.Body, b...)
	m.Body = append(m.Body, []byte(content)...)
	return m
}

func TestXStarRecvPipeAbort(t *testing.T) {
	self := GetSocket(t, NewSocket)
	mp, p := MockConnect(t, self)

	wg := sync.WaitGroup{}
	wg.Add(1)

	closeQ := make(chan struct{})

	go func() {
		defer wg.Done()
		for {
			m := newMessage(1, "")
			select {
			case mp.RecvQ() <- m:
			case <-closeQ:
				return
			}
		}
	}()

	time.Sleep(time.Millisecond * 20)
	MustSucceed(t, p.Close())
	time.Sleep(time.Millisecond * 20)
	MustSucceed(t, self.Close())

	close(closeQ)
	wg.Wait()
}

func TestXStarRecvSockAbort(t *testing.T) {
	self := GetSocket(t, NewSocket)
	l, mc := GetMockListener(t, self)

	MustSucceed(t, l.Listen())
	mp := mc.NewPipe(self.Info().Peer)
	mp.DeferClose(true)
	MockAddPipe(t, self, mc, mp)

	wg := sync.WaitGroup{}
	wg.Add(1)

	closeQ := make(chan struct{})

	go func() {
		defer wg.Done()
		for {
			m := newMessage(1, "")
			select {
			case mp.RecvQ() <- m:
			case <-closeQ:
				return
			}
		}
	}()

	time.Sleep(time.Millisecond * 20)
	MustSucceed(t, self.Close())
	mp.DeferClose(true)
	close(closeQ)
	wg.Wait()
}

func TestXStarBackPressure(t *testing.T) {
	self := GetSocket(t, NewSocket)
	MustSucceed(t, self.SetOption(OptionWriteQLen, 5))
	MockConnect(t, self)
	MockConnect(t, self)
	m := mangos.NewMessage(0)
	m.Header = append(m.Header, 0, 0, 0, 0)
	m.Body = append(m.Body, 42)
	// We don't read at all from it.
	for i := 0; i < 100; i++ {
		MustSendMsg(t, self, m.Dup())
	}
	time.Sleep(time.Millisecond * 100)
	MustClose(t, self)
}
