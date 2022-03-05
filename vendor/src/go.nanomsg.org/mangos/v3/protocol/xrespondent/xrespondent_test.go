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

package xrespondent

import (
	"encoding/binary"
	"sync"
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3"
	. "go.nanomsg.org/mangos/v3/internal/test"
	"go.nanomsg.org/mangos/v3/protocol"
	. "go.nanomsg.org/mangos/v3/protocol"
	"go.nanomsg.org/mangos/v3/protocol/surveyor"
	"go.nanomsg.org/mangos/v3/protocol/xsurveyor"
	_ "go.nanomsg.org/mangos/v3/transport/inproc"
)

func TestXRespondentIdentity(t *testing.T) {
	id := MustGetInfo(t, NewSocket)
	MustBeTrue(t, id.Peer == ProtoSurveyor)
	MustBeTrue(t, id.PeerName == "surveyor")
	MustBeTrue(t, id.Self == ProtoRespondent)
	MustBeTrue(t, id.SelfName == "respondent")
}

func TestXRespondentRaw(t *testing.T) {
	VerifyRaw(t, NewSocket)
}

func TestXRespondentClosed(t *testing.T) {
	VerifyClosedRecv(t, NewSocket)
	VerifyClosedSend(t, NewSocket)
	VerifyClosedClose(t, NewSocket)
	VerifyClosedDial(t, NewSocket)
	VerifyClosedListen(t, NewSocket)
	VerifyClosedAddPipe(t, NewSocket)
}

func TestXRespondentNoHeader(t *testing.T) {
	s := GetSocket(t, NewSocket)
	defer MustClose(t, s)
	m := mangos.NewMessage(0)

	MustSucceed(t, s.SendMsg(m))
}

func TestXRespondentMismatchHeader(t *testing.T) {
	s, err := NewSocket()
	MustSucceed(t, err)

	m := mangos.NewMessage(0)
	m.Header = append(m.Header, []byte{1, 1, 1, 1}...)

	MustSucceed(t, s.SendMsg(m))
	MustSucceed(t, s.Close())
}

func TestXRespondentRecvTimeout(t *testing.T) {
	s, err := NewSocket()
	MustSucceed(t, err)

	MustSucceed(t, s.SetOption(mangos.OptionRecvDeadline, time.Millisecond))
	m, err := s.RecvMsg()
	MustBeNil(t, m)
	MustFail(t, err)
	MustBeTrue(t, err == protocol.ErrRecvTimeout)
	MustSucceed(t, s.Close())
}

func TestXRespondentTTLZero(t *testing.T) {
	SetTTLZero(t, NewSocket)
}

func TestXRespondentTTLNegative(t *testing.T) {
	SetTTLNegative(t, NewSocket)
}

func TestXRespondentTTLTooBig(t *testing.T) {
	SetTTLTooBig(t, NewSocket)
}

func TestXRespondentTTLNotInt(t *testing.T) {
	SetTTLNotInt(t, NewSocket)
}

func TestXRespondentTTLSet(t *testing.T) {
	SetTTL(t, NewSocket)
}

func TestXRespondentTTLDrop(t *testing.T) {
	TTLDropTest(t, surveyor.NewSocket, NewSocket, xsurveyor.NewSocket, NewSocket)
}

func TestXRespondentOptions(t *testing.T) {
	VerifyInvalidOption(t, NewSocket)
	VerifyOptionDuration(t, NewSocket, mangos.OptionRecvDeadline)
	VerifyOptionDuration(t, NewSocket, mangos.OptionSendDeadline)
	VerifyOptionQLen(t, NewSocket, mangos.OptionReadQLen)
	VerifyOptionQLen(t, NewSocket, mangos.OptionWriteQLen)
	VerifyOptionInt(t, NewSocket, mangos.OptionTTL)
	VerifyOptionBool(t, NewSocket, mangos.OptionBestEffort)
}

func TestXRespondentSendTimeout(t *testing.T) {
	timeout := time.Millisecond * 10

	s, err := NewSocket()
	MustSucceed(t, err)
	MustNotBeNil(t, s)

	r, err := xsurveyor.NewSocket()
	MustSucceed(t, err)
	MustNotBeNil(t, r)

	MustSucceed(t, s.SetOption(mangos.OptionWriteQLen, 1))
	MustSucceed(t, r.SetOption(mangos.OptionReadQLen, 0))
	MustSucceed(t, s.SetOption(mangos.OptionRecvDeadline, timeout))
	MustSucceed(t, s.SetOption(mangos.OptionSendDeadline, timeout))
	MustSucceed(t, r.SetOption(mangos.OptionRecvDeadline, timeout))

	// We need to setup a connection so that we can get a meaningful
	// pipe ID.  We get this by receiving a message.
	a := AddrTestInp()
	MustSucceed(t, s.Listen(a))
	MustSucceed(t, r.Dial(a))

	time.Sleep(time.Millisecond * 20)
	MustSucceed(t, r.Send([]byte{0x80, 0, 0, 1})) // Request ID #1
	m, e := s.RecvMsg()
	MustSucceed(t, e)
	MustBeTrue(t, len(m.Header) >= 8) // request ID and pipe ID

	// Because of vagaries in the queuing, we slam messages until we
	// hit a timeout.  We expect to do so after only a modest number
	// of messages, as we have no reader on the other side.
	for i := 0; i < 100; i++ {
		e = s.SendMsg(m.Dup())
		if e != nil {
			break
		}
	}
	MustBeError(t, s.SendMsg(m.Dup()), mangos.ErrSendTimeout)
	MustSucceed(t, s.Close())
	MustSucceed(t, r.Close())
}

func TestXRespondentBestEffort(t *testing.T) {
	timeout := time.Millisecond * 10

	s, err := NewSocket()
	MustSucceed(t, err)
	MustNotBeNil(t, s)

	r, err := xsurveyor.NewSocket()
	MustSucceed(t, err)
	MustNotBeNil(t, r)

	MustSucceed(t, s.SetOption(mangos.OptionWriteQLen, 1))
	MustSucceed(t, r.SetOption(mangos.OptionReadQLen, 0))
	MustSucceed(t, s.SetOption(mangos.OptionRecvDeadline, timeout))
	MustSucceed(t, s.SetOption(mangos.OptionSendDeadline, timeout))
	MustSucceed(t, r.SetOption(mangos.OptionRecvDeadline, timeout))

	// We need to setup a connection so that we can get a meaningful
	// pipe ID.  We get this by receiving a message.
	a := AddrTestInp()
	MustSucceed(t, s.Listen(a))
	MustSucceed(t, r.Dial(a))

	time.Sleep(time.Millisecond * 20)
	MustSucceed(t, r.Send([]byte{0x80, 0, 0, 1})) // Request ID #1
	m, e := s.RecvMsg()
	MustSucceed(t, e)
	MustBeTrue(t, len(m.Header) >= 8) // request ID and pipe ID
	MustSucceed(t, s.SetOption(mangos.OptionBestEffort, true))

	// Because of vagaries in the queuing, we slam messages until we
	// hit a timeout.  We expect to do so after only a modest number
	// of messages, as we have no reader on the other side.
	for i := 0; i < 100; i++ {
		e = s.SendMsg(m.Dup())
		if e != nil {
			break
		}
	}
	MustSucceed(t, e)
	MustSucceed(t, s.Close())
	MustSucceed(t, r.Close())
}

func TestXRespondentRecvNoHeader(t *testing.T) {
	self := GetSocket(t, NewSocket)
	mock, _ := MockConnect(t, self)

	MustSucceed(t, self.SetOption(mangos.OptionReadQLen, 2))
	MustSucceed(t, self.SetOption(mangos.OptionRecvDeadline, time.Millisecond*50))
	MockMustSend(t, mock, []byte{}, time.Millisecond*5)
	MustNotRecv(t, self, mangos.ErrRecvTimeout)
	MustSucceed(t, self.Close())
}

func TestXRespondentRecvJunk(t *testing.T) {
	self := GetSocket(t, NewSocket)
	MustSucceed(t, self.SetOption(OptionReadQLen, 20))
	MustSucceed(t, self.SetOption(OptionRecvDeadline, time.Millisecond*10))

	mp, _ := MockConnect(t, self)
	MockMustSend(t, mp, []byte{}, time.Second)
	MockMustSend(t, mp, []byte{0, 1}, time.Second)
	MockMustSend(t, mp, []byte{0, 1, 2, 3}, time.Second)
	MockMustSend(t, mp, []byte{0, 1, 2, 3, 0x80}, time.Second)

	MustNotRecv(t, self, ErrRecvTimeout)
	MustClose(t, self)
}

func newRequest(id uint32, content string) *mangos.Message {
	m := mangos.NewMessage(len(content) + 8)
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b[:4], id|0x80000000)
	// Requests (coming in) will be entirely on the body.
	m.Body = append(m.Body, b...)
	m.Body = append(m.Body, []byte(content)...)
	return m
}

func newReply(id uint32, p mangos.Pipe, content string) *mangos.Message {
	m := mangos.NewMessage(len(content))
	b := make([]byte, 8)
	binary.BigEndian.PutUint32(b, p.ID())            // outgoing pipe ID
	binary.BigEndian.PutUint32(b[4:], id|0x80000000) // request ID
	m.Header = append(m.Header, b...)
	m.Body = append(m.Body, []byte(content)...)
	return m
}

func TestXRespondentPipeCloseSendAbort(t *testing.T) {
	self := GetSocket(t, NewSocket)
	defer MustClose(t, self)

	MustSucceed(t, self.SetOption(OptionWriteQLen, 0))
	MustSucceed(t, self.SetOption(OptionSendDeadline, time.Second))

	_, p := MockConnect(t, self)
	var wg sync.WaitGroup
	wg.Add(1)
	pass := false
	finishQ := make(chan struct{})
	go func() {
		defer wg.Done()
		i := uint32(0)
		for {
			i++
			m := newReply(i, p, "")
			MustSendMsg(t, self, m)
			select {
			case <-finishQ:
				pass = true
				return
			default:
			}
		}
	}()
	time.Sleep(time.Millisecond * 50)
	_ = p.Close()
	time.Sleep(time.Millisecond * 50)
	close(finishQ)
	wg.Wait()
	MustBeTrue(t, pass)

}

func TestXRespondentPipeCloseRecvAbort(t *testing.T) {
	self := GetSocket(t, NewSocket)
	defer MustClose(t, self)

	MustSucceed(t, self.SetOption(OptionReadQLen, 1))
	MustSucceed(t, self.SetOption(OptionSendDeadline, time.Second))

	mock, p := MockConnect(t, self)
	var wg sync.WaitGroup
	wg.Add(1)
	pass := false
	go func() {
		defer wg.Done()
		i := uint32(0)
		for {
			i++
			m := newRequest(i, "")
			e := mock.MockSendMsg(m, time.Second)
			if e != nil {
				MustBeError(t, e, ErrClosed)
				pass = true
				return
			}
			MustSucceed(t, e)
		}
	}()
	time.Sleep(time.Millisecond * 50)
	_ = p.Close()
	time.Sleep(time.Millisecond * 50)
	_ = mock.Close()

	wg.Wait()
	MustBeTrue(t, pass)

}

func TestXRespondentRecv1(t *testing.T) {
	self := GetSocket(t, NewSocket)
	mp, _ := MockConnect(t, self)
	MustSucceed(t, self.SetOption(OptionReadQLen, 0))
	MustSucceed(t, self.SetOption(OptionRecvDeadline, time.Millisecond))
	MockMustSendMsg(t, mp, newRequest(1, "hello"), time.Second)

	time.Sleep(time.Millisecond * 50)
	MustSucceed(t, self.SetOption(OptionReadQLen, 2))
	MustNotRecv(t, self, ErrRecvTimeout)
	MustClose(t, self)
}

func TestXRespondentResizeRecv2(t *testing.T) {
	self := GetSocket(t, NewSocket)
	mp, _ := MockConnect(t, self)
	MustSucceed(t, self.SetOption(OptionReadQLen, 1))
	MustSucceed(t, self.SetOption(OptionRecvDeadline, time.Second))

	time.AfterFunc(time.Millisecond*50, func() {
		MustSucceed(t, self.SetOption(OptionReadQLen, 2))
		time.Sleep(time.Millisecond * 50)
		MockMustSendMsg(t, mp, newRequest(1, "hello"), time.Second)
	})
	MustRecvString(t, self, "hello")
	MustClose(t, self)
}
