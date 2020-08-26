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

package respondent

import (
	"encoding/binary"
	"sync"
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3"
	. "go.nanomsg.org/mangos/v3/internal/test"
	"go.nanomsg.org/mangos/v3/protocol/surveyor"
	"go.nanomsg.org/mangos/v3/protocol/xrespondent"
	"go.nanomsg.org/mangos/v3/protocol/xsurveyor"
	_ "go.nanomsg.org/mangos/v3/transport/inproc"
)

func TestRespondentIdentity(t *testing.T) {
	s, err := NewSocket()
	MustSucceed(t, err)
	id := s.Info()
	MustBeTrue(t, id.Self == mangos.ProtoRespondent)
	MustBeTrue(t, id.SelfName == "respondent")
	MustBeTrue(t, id.Peer == mangos.ProtoSurveyor)
	MustBeTrue(t, id.PeerName == "surveyor")
	MustSucceed(t, s.Close())
}

func TestRespondentCooked(t *testing.T) {
	VerifyCooked(t, NewSocket)
}

func TestRespondentOptions(t *testing.T) {
	VerifyInvalidOption(t, NewSocket)
	VerifyOptionDuration(t, NewSocket, mangos.OptionRecvDeadline)
	VerifyOptionDuration(t, NewSocket, mangos.OptionSendDeadline)
	VerifyOptionInt(t, NewSocket, mangos.OptionTTL)
	VerifyOptionQLen(t, NewSocket, mangos.OptionReadQLen)
	VerifyOptionQLen(t, NewSocket, mangos.OptionWriteQLen)
	VerifyOptionBool(t, NewSocket, mangos.OptionBestEffort)
}

func TestRespondentClosed(t *testing.T) {
	VerifyClosedRecv(t, NewSocket)
	VerifyClosedSend(t, NewSocket)
	VerifyClosedClose(t, NewSocket)
	VerifyClosedDial(t, NewSocket)
	VerifyClosedListen(t, NewSocket)
	VerifyClosedAddPipe(t, NewSocket)
}

func TestRespondentTTLZero(t *testing.T) {
	SetTTLZero(t, NewSocket)
}

func TestRespondentTTLNegative(t *testing.T) {
	SetTTLNegative(t, NewSocket)
}

func TestRespondentTTLTooBig(t *testing.T) {
	SetTTLTooBig(t, NewSocket)
}

func TestRespondentTTLNotInt(t *testing.T) {
	SetTTLNotInt(t, NewSocket)
}

func TestRespondentTTLSet(t *testing.T) {
	SetTTL(t, NewSocket)
}

func TestRespondentTTLDrop(t *testing.T) {
	TTLDropTest(t, surveyor.NewSocket, NewSocket, xsurveyor.NewSocket, xrespondent.NewSocket)
}

// This test demonstrates that closing the socket aborts outstanding receives.
func TestRespondentCloseRx(t *testing.T) {
	s := GetSocket(t, NewSocket)
	var wg sync.WaitGroup
	wg.Add(1)
	pass := false
	go func() {
		defer wg.Done()
		v, e := s.Recv()
		MustBeError(t, e, mangos.ErrClosed)
		MustBeNil(t, v)
		pass = true
	}()
	time.Sleep(time.Millisecond * 10) // to allow go routine to run
	MustSucceed(t, s.Close())
	wg.Wait()
	MustBeTrue(t, pass)
}

func TestRespondentCloseSocketRecv(t *testing.T) {
	s := GetSocket(t, NewSocket)
	p := GetSocket(t, surveyor.NewSocket)
	ConnectPair(t, s, p)
	MustSucceed(t, s.SetOption(mangos.OptionReadQLen, 1))

	// Fill the pipe
	for i := 0; i < 10; i++ {
		// These all will work, but the back-pressure will go all the
		// way to the sender.
		MustSucceed(t, p.Send([]byte("")))
	}
	MustSucceed(t, s.Close())
}

func TestRepRespondentJunk(t *testing.T) {
	self := GetSocket(t, NewSocket)
	mock, _ := MockConnect(t, self)

	// Absent header...
	MockMustSendStr(t, mock, "", time.Second)

	// Absent request id... (must have bit 31 set)
	MockMustSend(t, mock, []byte{1, 2, 3, 4}, time.Second)

	time.Sleep(time.Millisecond * 10)
	MustSucceed(t, self.Close())
}

// This test fills our receive queue, then truncates it.
func TestRespondentResizeRecv(t *testing.T) {
	s := GetSocket(t, NewSocket)
	p := GetSocket(t, surveyor.NewSocket)
	MustSucceed(t, p.SetOption(mangos.OptionWriteQLen, 0))
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

	cnt := 0
	for i := 0; i < 20; i++ {
		if _, e := s.Recv(); e != nil {
			MustBeError(t, e, mangos.ErrRecvTimeout)
			break
		}
		cnt++
	}
	MustSucceed(t, s.Close())
}

// This tests that a waiting pipe reader will shift to the new pipe.
func TestRespondentResizeRecv2(t *testing.T) {
	s := GetSocket(t, NewSocket)
	p := GetSocket(t, surveyor.NewSocket)
	MustSucceed(t, p.SetOption(mangos.OptionWriteQLen, 0))
	MustSucceed(t, s.SetOption(mangos.OptionReadQLen, 2))
	MustSucceed(t, s.SetOption(mangos.OptionRecvDeadline, time.Millisecond))
	ConnectPair(t, s, p)

	// Fill the pipe
	for i := 0; i < 20; i++ {
		// These all will work, but the back-pressure will go all the
		// way to the sender.
		MustSucceed(t, p.Send([]byte{byte(i)}))
		time.Sleep(time.Millisecond * 5)
	}

	time.Sleep(time.Millisecond * 20)
	MustSucceed(t, s.SetOption(mangos.OptionReadQLen, 1))
	cnt := 0
	for i := 0; i < 20; i++ {
		if _, e := s.Recv(); e != nil {
			MustBeError(t, e, mangos.ErrRecvTimeout)
			break
		}
		cnt++
	}
	MustSucceed(t, s.Close())
}

// This tests that a posted recv sees the resize and moves to the new queue.
func TestRespondentResizeRecv3(t *testing.T) {
	s := GetSocket(t, NewSocket)
	p := GetSocket(t, surveyor.NewSocket)
	MustSucceed(t, p.SetOption(mangos.OptionWriteQLen, 0))
	MustSucceed(t, s.SetOption(mangos.OptionReadQLen, 10))
	MustSucceed(t, s.SetOption(mangos.OptionRecvDeadline, time.Minute))
	ConnectPair(t, s, p)

	MustSucceed(t, s.SetOption(mangos.OptionReadQLen, 1))
	time.AfterFunc(time.Millisecond*10, func() {
		MustSucceed(t, s.SetOption(mangos.OptionReadQLen, 2))
		time.Sleep(10 * time.Millisecond)
		MustSucceed(t, p.Send([]byte{}))
	})
	m, e := s.Recv()
	MustSucceed(t, e)
	MustNotBeNil(t, m)
	MustSucceed(t, s.Close())
	MustSucceed(t, p.Close())
}

func TestRespondentClosePipeRecv(t *testing.T) {
	s := GetSocket(t, NewSocket)
	p := GetSocket(t, surveyor.NewSocket)
	ConnectPair(t, s, p)
	MustSucceed(t, s.SetOption(mangos.OptionReadQLen, 1))
	MustSucceed(t, p.Send([]byte("")))
	m, e := s.RecvMsg()
	MustSucceed(t, e)

	// Fill the pipe
	for i := 0; i < 10; i++ {
		// These all will work, but the back-pressure will go all the
		// way to the sender.
		MustSucceed(t, p.Send([]byte("")))
	}
	MustSucceed(t, m.Pipe.Close())
	time.Sleep(time.Millisecond * 10)
	MustSucceed(t, s.Close())
	MustSucceed(t, p.Close())
}

func TestRespondentClosePipeSend(t *testing.T) {
	s := GetSocket(t, NewSocket)
	p := GetSocket(t, surveyor.NewSocket)
	ConnectPair(t, s, p)
	MustSucceed(t, s.SetOption(mangos.OptionReadQLen, 1))
	MustSucceed(t, p.Send([]byte("")))
	m, e := s.RecvMsg()
	MustSucceed(t, e)
	MustSucceed(t, m.Pipe.Close())
	time.Sleep(time.Millisecond * 10)
	MustSucceed(t, s.SendMsg(m))
	time.Sleep(time.Millisecond * 10)
	MustSucceed(t, s.Close())
	MustSucceed(t, p.Close())
}

func TestRespondentRecvExpire(t *testing.T) {
	s := GetSocket(t, NewSocket)
	MustSucceed(t, s.SetOption(mangos.OptionRecvDeadline, time.Millisecond))
	v, e := s.RecvMsg()
	MustBeError(t, e, mangos.ErrRecvTimeout)
	MustBeNil(t, v)
	MustSucceed(t, s.Close())
}

func TestRespondentSendState(t *testing.T) {
	s := GetSocket(t, NewSocket)
	MustBeError(t, s.Send([]byte{}), mangos.ErrProtoState)
	MustSucceed(t, s.Close())
}

func TestRespondentBestEffortSend(t *testing.T) {
	s := GetSocket(t, NewSocket)
	p := GetSocket(t, xsurveyor.NewSocket)
	MustSucceed(t, s.SetOption(mangos.OptionWriteQLen, 1))
	MustSucceed(t, p.SetOption(mangos.OptionReadQLen, 1))
	MustSucceed(t, s.SetOption(mangos.OptionBestEffort, true))

	ConnectPair(t, s, p)
	for i := 0; i < 100; i++ {
		// We have to make a raw message when using xsurveyor.  We
		// use xsurveyor because normal surveyor will simply discard
		// messages for surveys it doesn't have outstanding.
		m := mangos.NewMessage(0)
		m.Header = make([]byte, 4)
		binary.BigEndian.PutUint32(m.Header, uint32(i)|0x80000000)
		MustSucceed(t, p.SendMsg(m))
		m, e := s.RecvMsg()
		MustSucceed(t, e)
		MustNotBeNil(t, m)
		MustSucceed(t, s.SendMsg(m))
		// NB: We never ask the peer to receive it -- this ensures we
		// encounter backpressure.
	}
	MustSucceed(t, s.Close())
	MustSucceed(t, p.Close())
}

func TestRespondentSendBackPressure(t *testing.T) {
	s := GetSocket(t, NewSocket)
	p := GetSocket(t, xsurveyor.NewSocket)
	MustSucceed(t, s.SetOption(mangos.OptionWriteQLen, 1))
	MustSucceed(t, p.SetOption(mangos.OptionReadQLen, 1))

	ConnectPair(t, s, p)

	MustSucceed(t, s.SetOption(mangos.OptionSendDeadline, time.Millisecond*10))

	cnt := 0
	for i := 0; i < 100; i++ {
		// We have to make a raw message when using xsurveyor.  We
		// use xsurveyor because normal surveyor will simply discard
		// messages for surveys it doesn't have outstanding.
		m := mangos.NewMessage(0)
		m.Header = make([]byte, 4)
		binary.BigEndian.PutUint32(m.Header, uint32(i)|0x80000000)
		MustSucceed(t, p.SendMsg(m))
		m, e := s.RecvMsg()
		MustSucceed(t, e)
		MustNotBeNil(t, m)
		if e = s.SendMsg(m); e != nil {
			MustBeError(t, e, mangos.ErrSendTimeout)
			break
		}
		cnt++
		// NB: We never ask the peer to receive it -- this ensures we
		// encounter back-pressure.
	}
	MustBeTrue(t, cnt > 0) // Some in-flight sends possible.
	MustBeTrue(t, cnt < 10)
	MustSucceed(t, s.Close())
	MustSucceed(t, p.Close())
}

// This verifies that closing the socket aborts a blocking send.
// We use a context because closing the socket also closes pipes
// making it less reproducible.
func TestRespondentCloseContextSend(t *testing.T) {
	s := GetSocket(t, NewSocket)
	p := GetSocket(t, xsurveyor.NewSocket)
	MustSucceed(t, s.SetOption(mangos.OptionWriteQLen, 1))
	MustSucceed(t, p.SetOption(mangos.OptionReadQLen, 1))
	c, e := s.OpenContext()
	MustSucceed(t, e)

	ConnectPair(t, s, p)

	MustSucceed(t, c.SetOption(mangos.OptionSendDeadline, time.Millisecond*10))

	time.Sleep(time.Millisecond * 10)
	cnt := 0
	data := []byte{0x80, 0, 0, 1}
	for i := 0; i < 100; i++ {
		// We have to make a raw message when using xsurveyor.  We
		// use xsurveyor because normal surveyor will simply discard
		// messages for surveys it doesn't have outstanding.
		m := mangos.NewMessage(0)
		m.Header = append(m.Header, data...)
		data[3]++
		MustSucceed(t, p.SendMsg(m))
		m, e := c.RecvMsg()
		MustSucceed(t, e)
		MustNotBeNil(t, m)
		if e = c.SendMsg(m); e != nil {
			MustBeError(t, e, mangos.ErrSendTimeout)
			break
		}
		cnt++

		// NB: We never ask the peer to receive it -- this ensures we
		// encounter back-pressure.
	}
	MustBeTrue(t, cnt > 0) // Some in-flight sends possible.
	MustBeTrue(t, cnt < 10)

	m := mangos.NewMessage(0)
	m.Header = append(m.Header, data...)
	data[3]++
	MustSucceed(t, p.SendMsg(m))

	MustSucceed(t, c.SetOption(mangos.OptionSendDeadline, time.Minute))
	m, e = c.RecvMsg()
	MustSucceed(t, e)
	MustNotBeNil(t, m)
	time.AfterFunc(time.Millisecond*20, func() { MustSucceed(t, c.Close()) })
	MustBeError(t, c.SendMsg(m), mangos.ErrClosed)

	MustSucceed(t, p.Close())
	MustSucceed(t, s.Close())
}

func TestRespondentContextClosed(t *testing.T) {
	s := GetSocket(t, NewSocket)
	c, e := s.OpenContext()
	MustSucceed(t, e)
	MustNotBeNil(t, c)
	MustSucceed(t, c.Close())
	MustBeError(t, c.Close(), mangos.ErrClosed)

	c, e = s.OpenContext()
	MustSucceed(t, e)
	MustNotBeNil(t, c)

	MustSucceed(t, s.Close())

	MustBeError(t, c.Close(), mangos.ErrClosed)

	_, e = s.OpenContext()
	MustBeError(t, e, mangos.ErrClosed)
}

// This sets up a bunch of contexts to run in parallel, and verifies that
// they all seem to run with no mis-deliveries.
func TestRespondentMultiContexts(t *testing.T) {
	count := 30
	repeat := 20

	s := GetSocket(t, NewSocket)
	p := GetSocket(t, surveyor.NewSocket)
	MustSucceed(t, s.SetOption(mangos.OptionReadQLen, count*repeat))
	MustSucceed(t, p.SetOption(mangos.OptionReadQLen, count*repeat))
	MustSucceed(t, p.SetOption(mangos.OptionWriteQLen, count*repeat))
	MustSucceed(t, s.SetOption(mangos.OptionWriteQLen, count*repeat))

	ConnectPair(t, p, s)

	recv := make([]int, count)
	send := make([]int, count)

	var wg1 sync.WaitGroup
	fn := func(c1, c2 mangos.Context, index int) {
		defer wg1.Done()

		topic := make([]byte, 4)
		binary.BigEndian.PutUint32(topic, uint32(index))

		for i := 0; i < repeat; i++ {
			MustSucceed(t, c2.Send(topic))
			m, e := c1.RecvMsg()
			MustSucceed(t, e)
			MustBeTrue(t, len(m.Body) == 4)
			peer := binary.BigEndian.Uint32(m.Body)
			recv[int(peer)]++
			MustSucceed(t, c1.Send([]byte("answer")))
			m, e = c2.RecvMsg()
			MustSucceed(t, e)
			MustBeTrue(t, string(m.Body) == "answer")
			send[index]++
		}
	}

	wg1.Add(count)

	for i := 0; i < count; i++ {
		c1, e := s.OpenContext()
		MustSucceed(t, e)
		MustNotBeNil(t, c1)

		c2, e := p.OpenContext()
		MustSucceed(t, e)
		MustNotBeNil(t, c2)

		MustSucceed(t, c1.SetOption(mangos.OptionRecvDeadline, time.Minute/4))
		MustSucceed(t, c1.SetOption(mangos.OptionSendDeadline, time.Minute/4))
		MustSucceed(t, c2.SetOption(mangos.OptionRecvDeadline, time.Minute/4))
		MustSucceed(t, c2.SetOption(mangos.OptionSurveyTime, time.Minute/2))

		go fn(c1, c2, i)
	}

	// Give time for everything to be delivered.
	wg1.Wait()
	MustSucceed(t, p.Close())
	MustSucceed(t, s.Close())

	for i := 0; i < count; i++ {
		MustBeTrue(t, recv[i] == repeat)
		MustBeTrue(t, send[i] == repeat)
	}
}
