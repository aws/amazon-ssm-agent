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

package xbus

import (
	"bytes"
	"encoding/binary"
	"sync"
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3"
	. "go.nanomsg.org/mangos/v3/internal/test"
	. "go.nanomsg.org/mangos/v3/protocol"
	_ "go.nanomsg.org/mangos/v3/transport/inproc"
)

func TestXBusIdentity(t *testing.T) {
	id := MustGetInfo(t, NewSocket)
	MustBeTrue(t, id.Self == ProtoBus)
	MustBeTrue(t, id.SelfName == "bus")
	MustBeTrue(t, id.Peer == ProtoBus)
	MustBeTrue(t, id.PeerName == "bus")
}

func TestXBusRaw(t *testing.T) {
	VerifyRaw(t, NewSocket)
}

func TestXBusClosed(t *testing.T) {
	VerifyClosedRecv(t, NewSocket)
	VerifyClosedSend(t, NewSocket)
	VerifyClosedClose(t, NewSocket)
	VerifyClosedDial(t, NewSocket)
	VerifyClosedListen(t, NewSocket)
	VerifyClosedAddPipe(t, NewSocket)
}

func TestXBusOptions(t *testing.T) {
	VerifyInvalidOption(t, NewSocket)
	VerifyOptionDuration(t, NewSocket, OptionRecvDeadline)
	VerifyOptionInt(t, NewSocket, OptionReadQLen)
	VerifyOptionInt(t, NewSocket, OptionWriteQLen)
}

func TestXBusRecvDeadline(t *testing.T) {
	self := GetSocket(t, NewSocket)
	MustSucceed(t, self.SetOption(OptionRecvDeadline, time.Millisecond))
	MustNotRecv(t, self, ErrRecvTimeout)
	MustClose(t, self)
}

// This ensures we get our pipe ID on receive as the sole header.
func TestXBusRecvHeader(t *testing.T) {
	self := GetSocket(t, NewSocket)
	mock, _ := MockConnect(t, self)

	MockMustSendStr(t, mock, "abc", time.Second)
	recv := MustRecvMsg(t, self)
	MustBeTrue(t, string(recv.Body) == "abc")
	MustBeTrue(t, len(recv.Header) == 4)
	MustBeTrue(t, recv.Pipe.ID() == binary.BigEndian.Uint32(recv.Header))
	MustClose(t, self)
}

// This ensures that on send the header is discarded.
func TestXBusSendRecvHeader(t *testing.T) {
	self := GetSocket(t, NewSocket)
	peer := GetSocket(t, NewSocket)

	data := []byte{'a', 'b', 'c'}
	send := mangos.NewMessage(0)
	send.Body = append(send.Body, data...)
	send.Header = append(send.Header, 1, 2, 3, 4)
	ConnectPair(t, self, peer)
	MustSucceed(t, peer.SendMsg(send))
	recv := MustRecvMsg(t, self)
	MustBeTrue(t, bytes.Equal(data, recv.Body))
	MustBeTrue(t, len(recv.Header) == 4)
	MustBeTrue(t, recv.Pipe.ID() == binary.BigEndian.Uint32(recv.Header))
	MustSucceed(t, self.Close())
	MustSucceed(t, peer.Close())
}

// This tests that if we send down a message with a header matching a peer,
// it won't go back out to where it came from.
func TestXBusNoLoop(t *testing.T) {
	self := GetSocket(t, NewSocket)
	peer := GetSocket(t, NewSocket)

	data := []byte{'a', 'b', 'c'}
	send := mangos.NewMessage(0)
	send.Body = append(send.Body, data...)
	send.Header = append(send.Header, 1, 2, 3, 4)
	MustSucceed(t, self.SetOption(OptionRecvDeadline, time.Millisecond*5))
	MustSucceed(t, peer.SetOption(OptionRecvDeadline, time.Millisecond*5))
	ConnectPair(t, self, peer)
	MustSendMsg(t, peer, send)
	recv := MustRecvMsg(t, self)
	MustBeTrue(t, bytes.Equal(data, recv.Body))
	MustBeTrue(t, len(recv.Header) == 4)
	MustBeTrue(t, recv.Pipe.ID() == binary.BigEndian.Uint32(recv.Header))

	MustSucceed(t, self.SendMsg(recv))
	MustNotRecv(t, peer, ErrRecvTimeout)
	MustClose(t, self)
	MustClose(t, peer)
}

func TestXBusNonBlock(t *testing.T) {

	self := GetSocket(t, NewSocket)
	MustSucceed(t, self.SetOption(OptionWriteQLen, 1))
	for i := 0; i < 100; i++ {
		MustSendString(t, self, "abc")
	}
	MustSucceed(t, self.Close())
}

func TestXBusSendDrop(t *testing.T) {
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

func TestXBusResizeRecv(t *testing.T) {
	self := GetSocket(t, NewSocket)
	peer := GetSocket(t, NewSocket)
	MustSucceed(t, peer.SetOption(OptionWriteQLen, 20))
	MustSucceed(t, self.SetOption(OptionReadQLen, 20))
	MustSucceed(t, self.SetOption(OptionRecvDeadline, time.Minute))
	ConnectPair(t, self, peer)

	time.AfterFunc(time.Millisecond*20, func() {
		MustSucceed(t, self.SetOption(OptionReadQLen, 10))
		MustSendString(t, peer, "wakeup")
	})

	MustRecvString(t, self, "wakeup")
	MustClose(t, self)
}

func TestXBusResize(t *testing.T) {
	self := GetSocket(t, NewSocket)
	peer := GetSocket(t, NewSocket)
	MustSucceed(t, peer.SetOption(mangos.OptionWriteQLen, 10))
	MustSucceed(t, self.SetOption(mangos.OptionReadQLen, 2))
	MustSucceed(t, self.SetOption(mangos.OptionRecvDeadline, time.Millisecond))
	ConnectPair(t, self, peer)

	// Over-fill the pipe -- this gets the reader stuck in the right place.
	for i := 0; i < 10; i++ {
		// These all will work, but the back-pressure will go all the
		// way to the sender.
		MustSendString(t, peer, "fill me up")
	}

	time.Sleep(time.Millisecond * 200)
	MustSucceed(t, self.SetOption(OptionReadQLen, 10))
	// Sleep so the resize filler finishes
	time.Sleep(time.Millisecond * 20)

	MustNotRecv(t, self, ErrRecvTimeout)
	MustSucceed(t, self.Close())
}

func TestXBusBackPressure(t *testing.T) {
	self := GetSocket(t, NewSocket)
	MustSucceed(t, self.SetOption(OptionWriteQLen, 5))
	MockConnect(t, self)
	MockConnect(t, self)
	// We don't read at all from it.
	for i := 0; i < 100; i++ {
		MustSendString(t, self, "")
	}
	time.Sleep(time.Millisecond * 100)
	MustClose(t, self)
}

func TestXBusRecvPipeAbort(t *testing.T) {
	self := GetSocket(t, NewSocket)
	mp, p := MockConnect(t, self)

	wg := sync.WaitGroup{}
	wg.Add(1)

	closeQ := make(chan struct{})

	go func() {
		defer wg.Done()
		for {
			m := mangos.NewMessage(0)
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

func TestXBusRecvSockAbort(t *testing.T) {
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
			m := mangos.NewMessage(0)
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
