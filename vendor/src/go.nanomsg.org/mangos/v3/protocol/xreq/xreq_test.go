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

package xreq

import (
	"sync"
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3"
	. "go.nanomsg.org/mangos/v3/internal/test"
	"go.nanomsg.org/mangos/v3/protocol/rep"
	"go.nanomsg.org/mangos/v3/protocol/req"
	"go.nanomsg.org/mangos/v3/protocol/xrep"
	_ "go.nanomsg.org/mangos/v3/transport/inproc"
)

func TestXReqIdentity(t *testing.T) {
	id := MustGetInfo(t, NewSocket)
	MustBeTrue(t, id.Self == mangos.ProtoReq)
	MustBeTrue(t, id.SelfName == "req")
	MustBeTrue(t, id.Peer == mangos.ProtoRep)
	MustBeTrue(t, id.PeerName == "rep")
}

func TestXReqRaw(t *testing.T) {
	VerifyRaw(t, NewSocket)
}

func TestXReqClosed(t *testing.T) {
	VerifyClosedRecv(t, NewSocket)
	VerifyClosedSend(t, NewSocket)
	VerifyClosedClose(t, NewSocket)
	VerifyClosedDial(t, NewSocket)
	VerifyClosedListen(t, NewSocket)
	VerifyClosedAddPipe(t, NewSocket)
}

func TestXReqOptions(t *testing.T) {
	VerifyInvalidOption(t, NewSocket)
	VerifyOptionDuration(t, NewSocket, mangos.OptionRecvDeadline)
	VerifyOptionDuration(t, NewSocket, mangos.OptionSendDeadline)
	VerifyOptionInt(t, NewSocket, mangos.OptionReadQLen)
	VerifyOptionInt(t, NewSocket, mangos.OptionWriteQLen)
	VerifyOptionBool(t, NewSocket, mangos.OptionBestEffort)
}

func TestXReqBestEffort(t *testing.T) {
	timeout := time.Millisecond
	msg := []byte{'0', '1', '2', '3'}

	s := GetSocket(t, NewSocket)
	MustSucceed(t, s.SetOption(mangos.OptionWriteQLen, 0))
	MustSucceed(t, s.SetOption(mangos.OptionSendDeadline, timeout))
	MustSucceed(t, s.Listen(AddrTestInp()))
	MustSucceed(t, s.SetOption(mangos.OptionBestEffort, true))
	MustSucceed(t, s.Send(msg))
	MustSucceed(t, s.Send(msg))
	MustSucceed(t, s.SetOption(mangos.OptionBestEffort, false))
	MustBeError(t, s.Send(msg), mangos.ErrSendTimeout)
	MustBeError(t, s.Send(msg), mangos.ErrSendTimeout)
}

func TestXReqDevice(t *testing.T) {
	r1, e := req.NewSocket()
	MustSucceed(t, e)
	r2, e := rep.NewSocket()
	MustSucceed(t, e)

	r3, e := xrep.NewSocket()
	MustSucceed(t, e)
	r4, e := NewSocket()
	MustSucceed(t, e)

	a1 := AddrTestInp()
	a2 := AddrTestInp()

	MustSucceed(t, mangos.Device(r3, r4))

	// r1 -> r3 / r4 -> r2
	MustSucceed(t, r3.Listen(a1))
	MustSucceed(t, r4.Listen(a2))

	MustSucceed(t, r1.Dial(a1))
	MustSucceed(t, r2.Dial(a2))

	MustSucceed(t, r1.SetOption(mangos.OptionRecvDeadline, time.Second))
	MustSucceed(t, r1.SetOption(mangos.OptionSendDeadline, time.Second))
	MustSucceed(t, r2.SetOption(mangos.OptionRecvDeadline, time.Second))
	MustSucceed(t, r2.SetOption(mangos.OptionSendDeadline, time.Second))
	MustSucceed(t, r1.Send([]byte("PING")))
	ping, e := r2.Recv()
	MustSucceed(t, e)
	MustBeTrue(t, string(ping) == "PING")
	MustSucceed(t, r2.Send([]byte("PONG")))
	pong, e := r1.Recv()
	MustSucceed(t, e)
	MustBeTrue(t, string(pong) == "PONG")
	_ = r1.Close()
	_ = r2.Close()
	_ = r3.Close()
	_ = r4.Close()
}

func TestXReqRecvDeadline(t *testing.T) {
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

func TestXReqResizeSendDiscard(t *testing.T) {
	self := GetSocket(t, NewSocket)
	MustSucceed(t, self.SetOption(mangos.OptionWriteQLen, 1))
	MustSucceed(t, self.Send([]byte{}))
	MustSucceed(t, self.SetOption(mangos.OptionSendDeadline, time.Millisecond*10))
	MustBeError(t, self.Send([]byte{}), mangos.ErrSendTimeout)
	MustSucceed(t, self.SetOption(mangos.OptionSendDeadline, time.Millisecond*50))
	time.AfterFunc(time.Millisecond*5, func() {
		MustSucceed(t, self.SetOption(mangos.OptionWriteQLen, 1))
	})
	MustSucceed(t, self.Send([]byte{}))
	MustSucceed(t, self.Close())
}

func TestXReqResizeRecvOk(t *testing.T) {
	self := GetSocket(t, NewSocket)
	mock, _ := MockConnect(t, self)
	var wg sync.WaitGroup
	wg.Add(1)
	pass := false
	MustSucceed(t, self.SetOption(mangos.OptionRecvDeadline, time.Second))
	go func() {
		defer wg.Done()
		_ = MustRecvMsg(t, self)
		pass = true
	}()
	time.Sleep(time.Millisecond * 50)
	MustSucceed(t, self.SetOption(mangos.OptionReadQLen, 1))
	time.Sleep(time.Millisecond * 10)
	MockMustSend(t, mock, []byte{0x80, 0, 0, 1}, time.Second)
	wg.Wait()
	MustSucceed(t, self.Close())
	MustBeTrue(t, pass)
}

func TestXReqRecvNoHeader(t *testing.T) {
	self := GetSocket(t, NewSocket)
	mock, _ := MockConnect(t, self)

	MustSucceed(t, self.SetOption(mangos.OptionReadQLen, 2))
	MustSucceed(t, self.SetOption(mangos.OptionRecvDeadline, time.Millisecond*50))
	MockMustSend(t, mock, []byte{}, time.Second)
	MustNotRecv(t, self, mangos.ErrRecvTimeout)
	MustSucceed(t, self.Close())
}

func TestXReqRecvResizeDiscard(t *testing.T) {
	self := GetSocket(t, NewSocket)
	defer MustClose(t, self)
	mock, _ := MockConnect(t, self)
	MustSucceed(t, self.SetOption(mangos.OptionReadQLen, 1))

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		for {
			m := mangos.NewMessage(0)
			m.Body = append(m.Body, 0x80, 0, 0, 1)
			e := mock.MockSendMsg(m, time.Second)
			if e != nil {
				MustBeError(t, e, mangos.ErrClosed)
				break
			}

		}
	}()
	time.Sleep(time.Millisecond * 50)
	MustSucceed(t, self.SetOption(mangos.OptionReadQLen, 2))
	time.Sleep(time.Millisecond * 50)
	MustSucceed(t, mock.Close())
	wg.Wait()
}

func TestXReqCloseRecv(t *testing.T) {
	self := GetSocket(t, NewSocket)
	MustSucceed(t, self.SetOption(mangos.OptionReadQLen, 0))
	mock, pipe := MockConnect(t, self)
	MockMustSend(t, mock, []byte{0x80, 0, 0, 1}, time.Second)
	time.Sleep(time.Millisecond * 10)
	MustSucceed(t, pipe.Close())
	time.Sleep(time.Millisecond * 10)
	MustSucceed(t, self.Close())
}

func TestXReqCloseSend0(t *testing.T) {
	self := GetSocket(t, NewSocket)
	_, pipe := MockConnect(t, self)
	time.Sleep(time.Millisecond * 10)
	MustSucceed(t, pipe.Close())
	time.Sleep(time.Millisecond * 10)
}

func TestXReqCloseSend1(t *testing.T) {
	self := GetSocket(t, NewSocket)
	_, _ = MockConnect(t, self)
	time.Sleep(time.Millisecond * 10)
	MustSucceed(t, self.Close())
	time.Sleep(time.Millisecond * 10)
}

func TestXReqCloseSend2(t *testing.T) {
	self := GetSocket(t, NewSocket)
	MustSucceed(t, self.SetOption(mangos.OptionWriteQLen, 2))
	_, _ = MockConnect(t, self)
	MustSucceed(t, self.Send([]byte{0x80, 1, 1, 1}))
	MustSucceed(t, self.Send([]byte{0x80, 1, 1, 2}))
	time.Sleep(time.Millisecond * 10)
	MustSucceed(t, self.Close())
	time.Sleep(time.Millisecond * 10)
}
