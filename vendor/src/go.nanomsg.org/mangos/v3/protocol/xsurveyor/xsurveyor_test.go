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

package xsurveyor

import (
	"encoding/binary"
	"sync"
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3"
	. "go.nanomsg.org/mangos/v3/internal/test"
	"go.nanomsg.org/mangos/v3/protocol"
	"go.nanomsg.org/mangos/v3/protocol/respondent"
	_ "go.nanomsg.org/mangos/v3/transport/inproc"
)

func TestXSurveyorIdentity(t *testing.T) {
	self := GetSocket(t, NewSocket)
	id := self.Info()
	MustBeTrue(t, id.Self == mangos.ProtoSurveyor)
	MustBeTrue(t, id.SelfName == "surveyor")
	MustBeTrue(t, id.Peer == mangos.ProtoRespondent)
	MustBeTrue(t, id.PeerName == "respondent")
	MustSucceed(t, self.Close())
}

func TestXSurveyorRaw(t *testing.T) {
	VerifyRaw(t, NewSocket)
}

func TestXSurveyorClosed(t *testing.T) {
	VerifyClosedRecv(t, NewSocket)
	VerifyClosedSend(t, NewSocket)
	VerifyClosedClose(t, NewSocket)
	VerifyClosedDial(t, NewSocket)
	VerifyClosedListen(t, NewSocket)
	VerifyClosedAddPipe(t, NewSocket)
}

func makeSurveyMsg(id uint32, s string) *mangos.Message {
	m := mangos.NewMessage(0)
	m.Header = append(m.Header, make([]byte, 4)...)
	binary.BigEndian.PutUint32(m.Header, id|0x80000000)
	m.Body = append(m.Body, []byte(s)...)
	return m
}

func TestXSurveyorNoHeader(t *testing.T) {
	self := GetSocket(t, NewSocket)

	m := mangos.NewMessage(0)

	MustSucceed(t, self.SendMsg(m))
	MustSucceed(t, self.Close())
}

// We can send even with no pipes
func TestXSurveyorNonBlock1(t *testing.T) {
	self := GetSocket(t, NewSocket)
	for i := 0; i < 100; i++ {
		m := makeSurveyMsg(uint32(i), "ping")
		MustSucceed(t, self.SendMsg(m))
	}
	time.Sleep(time.Millisecond * 10)
	MustSucceed(t, self.Close())
}

// We can send even if we have a pipe that is blocked
func TestXSurveyorNonBlock2(t *testing.T) {
	self := GetSocket(t, NewSocket)
	MustSucceed(t, self.SetOption(mangos.OptionWriteQLen, 1))
	_, _ = MockConnect(t, self)

	for i := 0; i < 100; i++ {
		m := makeSurveyMsg(uint32(i), "ping")
		MustSucceed(t, self.SendMsg(m))
	}
	time.Sleep(time.Millisecond * 50)
	MustSucceed(t, self.Close())
}

func TestXSurveyorRecvTimeout(t *testing.T) {
	s, err := NewSocket()
	MustSucceed(t, err)

	MustSucceed(t, s.SetOption(mangos.OptionRecvDeadline, time.Millisecond))
	m, err := s.RecvMsg()
	MustBeNil(t, m)
	MustBeError(t, err, protocol.ErrRecvTimeout)
	MustSucceed(t, s.Close())
}

// XSurveyor needs a surveyor ID or it will discard.
func TestXSurveyorRecvJunk(t *testing.T) {
	self := GetSocket(t, NewSocket)
	mock, _ := MockConnect(t, self)

	MockMustSendStr(t, mock, "", time.Second)
	time.Sleep(time.Millisecond * 10)
	MustSucceed(t, self.Close())
}

func TestXSurveyorOptions(t *testing.T) {
	VerifyInvalidOption(t, NewSocket)
	VerifyOptionDuration(t, NewSocket, mangos.OptionRecvDeadline)
	VerifyOptionQLen(t, NewSocket, mangos.OptionReadQLen)
	VerifyOptionQLen(t, NewSocket, mangos.OptionWriteQLen)
}

func TestXSurveyorRecvQLenResizeDiscard(t *testing.T) {
	self := GetSocket(t, NewSocket)
	peer := GetSocket(t, respondent.NewSocket)

	MustSucceed(t, self.SetOption(mangos.OptionRecvDeadline, time.Second))
	MustSucceed(t, self.SetOption(mangos.OptionReadQLen, 3))
	MustSucceed(t, peer.SetOption(mangos.OptionWriteQLen, 100))
	ConnectPair(t, self, peer)

	// Now do some exchanges, but don't collect the answers -- let our
	// receive queue fill up.
	for i := uint32(0); i < 100; i++ {
		MustSucceed(t, self.SendMsg(makeSurveyMsg(i, "")))
		_ = MustRecvMsg(t, peer)
		MustSendString(t, peer, "reply")
	}

	time.Sleep(time.Millisecond * 50)
	// Shrink it
	MustSucceed(t, self.SetOption(mangos.OptionReadQLen, 2))
	// This can fail or succeed -- depending on how much back-pressure we
	// have applied; it usually succeeds.
	time.Sleep(time.Millisecond * 50)
	_, _ = self.RecvMsg()
	time.Sleep(time.Millisecond * 10)
	MustSucceed(t, peer.Close())
	MustSucceed(t, self.Close())
}

func TestXSurveyorRecvClosePipeDiscard(t *testing.T) {
	self := GetSocket(t, NewSocket)
	peer := GetSocket(t, respondent.NewSocket)

	MustSucceed(t, self.SetOption(mangos.OptionRecvDeadline, time.Second))
	MustSucceed(t, self.SetOption(mangos.OptionReadQLen, 3))
	MustSucceed(t, peer.SetOption(mangos.OptionWriteQLen, 100))
	ConnectPair(t, self, peer)

	// One exchange to get the pipe ID to close
	MustSucceed(t, self.SendMsg(makeSurveyMsg(0, "")))
	_ = MustRecvMsg(t, peer)
	MustSendString(t, peer, "junk")
	r, e := self.RecvMsg()
	MustSucceed(t, e)
	pipe := r.Pipe

	// Now do some exchanges, but don't collect the answers -- let our
	// receive queue fill up.
	for i := uint32(0); i < 100; i++ {
		MustSucceed(t, self.SendMsg(makeSurveyMsg(i, "")))
		_ = MustRecvMsg(t, peer)
		MustSendString(t, peer, "reply")
	}

	MustSucceed(t, pipe.Close())
	time.Sleep(time.Millisecond * 20)
	MustSucceed(t, self.Close())
	MustSucceed(t, peer.Close())

}

// This ensures that the upper RecvMsg sees the new queue after a resize
func TestXSurveyorResizeRecv(t *testing.T) {
	self := GetSocket(t, NewSocket)
	peer := GetSocket(t, respondent.NewSocket)
	MustSucceed(t, peer.SetOption(mangos.OptionWriteQLen, 20))
	MustSucceed(t, self.SetOption(mangos.OptionReadQLen, 20))
	MustSucceed(t, self.SetOption(mangos.OptionRecvDeadline, time.Minute))
	ConnectPair(t, self, peer)

	m := makeSurveyMsg(1, "hello")
	MustSucceed(t, self.SendMsg(m))
	MustRecvString(t, peer, "hello")

	time.AfterFunc(time.Millisecond*20, func() {
		MustSucceed(t, self.SetOption(mangos.OptionReadQLen, 10))
		time.Sleep(time.Millisecond)
		MustSendString(t, peer, "world")
	})

	r, e := self.RecvMsg()
	MustSucceed(t, e)
	MustBeTrue(t, binary.BigEndian.Uint32(r.Header) == 0x80000001)
	MustBeTrue(t, string(r.Body) == "world")

	MustSucceed(t, self.Close())
}

func TestXSurveyorBroadcast(t *testing.T) {
	s, err := NewSocket()
	MustSucceed(t, err)
	MustNotBeNil(t, s)

	s1, err := respondent.NewSocket()
	MustSucceed(t, err)
	MustNotBeNil(t, s1)

	s2, err := respondent.NewSocket()
	MustSucceed(t, err)
	MustNotBeNil(t, s2)

	a := AddrTestInp()
	MustSucceed(t, s.Listen(a))
	MustSucceed(t, s1.Dial(a))
	MustSucceed(t, s2.Dial(a))

	time.Sleep(time.Millisecond * 50)

	MustSucceed(t, s1.SetOption(mangos.OptionRecvDeadline, time.Millisecond*50))
	MustSucceed(t, s2.SetOption(mangos.OptionRecvDeadline, time.Millisecond*50))

	MustSucceed(t, s.SendMsg(makeSurveyMsg(1, "one")))
	MustSucceed(t, s.SendMsg(makeSurveyMsg(2, "two")))
	MustSucceed(t, s.SendMsg(makeSurveyMsg(3, "three")))

	var wg sync.WaitGroup
	wg.Add(2)
	pass1 := false
	pass2 := false

	f := func(s mangos.Socket, pass *bool) {
		defer wg.Done()
		v, e := s.Recv()
		MustSucceed(t, e)
		MustBeTrue(t, string(v) == "one")

		v, e = s.Recv()
		MustSucceed(t, e)
		MustBeTrue(t, string(v) == "two")

		v, e = s.Recv()
		MustSucceed(t, e)
		MustBeTrue(t, string(v) == "three")

		v, e = s.Recv()
		MustBeError(t, e, mangos.ErrRecvTimeout)
		MustBeNil(t, v)
		*pass = true
	}

	go f(s1, &pass1)
	go f(s2, &pass2)

	wg.Wait()

	MustBeTrue(t, pass1)
	MustBeTrue(t, pass2)

	MustSucceed(t, s.Close())
	MustSucceed(t, s1.Close())
	MustSucceed(t, s2.Close())
}
