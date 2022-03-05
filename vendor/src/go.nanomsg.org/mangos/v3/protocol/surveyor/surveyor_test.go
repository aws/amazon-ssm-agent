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

package surveyor

import (
	"bytes"
	"encoding/binary"
	"sync"
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3"
	. "go.nanomsg.org/mangos/v3/internal/test"
	"go.nanomsg.org/mangos/v3/protocol/respondent"
	_ "go.nanomsg.org/mangos/v3/transport/inproc"
)

func TestSurveyorIdentity(t *testing.T) {
	s, err := NewSocket()
	MustSucceed(t, err)
	id := s.Info()
	MustBeTrue(t, id.Self == mangos.ProtoSurveyor)
	MustBeTrue(t, id.SelfName == "surveyor")
	MustBeTrue(t, id.Peer == mangos.ProtoRespondent)
	MustBeTrue(t, id.PeerName == "respondent")
	MustSucceed(t, s.Close())
}

func TestSurveyorCooked(t *testing.T) {
	VerifyCooked(t, NewSocket)
}

func TestSurveyorNonBlock(t *testing.T) {
	maxqlen := 2

	rp, err := NewSocket()
	MustSucceed(t, err)
	MustNotBeNil(t, rp)

	MustSucceed(t, rp.SetOption(mangos.OptionWriteQLen, maxqlen))
	MustSucceed(t, rp.Listen(AddrTestInp()))

	msg := []byte{'A', 'B', 'C'}
	start := time.Now()
	for i := 0; i < maxqlen*10; i++ {
		MustSucceed(t, rp.Send(msg))
	}
	end := time.Now()
	MustBeTrue(t, end.Sub(start) < time.Second/10)
	MustSucceed(t, rp.Close())
}

func TestSurveyorOptions(t *testing.T) {
	VerifyInvalidOption(t, NewSocket)
	VerifyOptionDuration(t, NewSocket, mangos.OptionRecvDeadline)
	VerifyOptionDuration(t, NewSocket, mangos.OptionSurveyTime)
	VerifyOptionQLen(t, NewSocket, mangos.OptionReadQLen)
	VerifyOptionQLen(t, NewSocket, mangos.OptionWriteQLen)
}

func TestSurveyorClosed(t *testing.T) {
	VerifyClosedRecv(t, NewSocket)
	VerifyClosedSend(t, NewSocket)
	VerifyClosedClose(t, NewSocket)
	VerifyClosedDial(t, NewSocket)
	VerifyClosedListen(t, NewSocket)
	VerifyClosedAddPipe(t, NewSocket)
}

func TestSurveyorRecvState(t *testing.T) {
	s, e := NewSocket()
	MustSucceed(t, e)
	v, e := s.Recv()
	MustBeError(t, e, mangos.ErrProtoState)
	MustBeNil(t, v)
	MustSucceed(t, s.Close())
}

// This test demonstrates that sending a second survey cancels any Rx on the
// earlier outstanding ones.
func TestSurveyorCancel(t *testing.T) {
	self := GetSocket(t, NewSocket)
	MustSucceed(t, self.SetOption(mangos.OptionSurveyTime, time.Second))
	MustSucceed(t, self.Send([]byte("first")))
	var wg sync.WaitGroup
	wg.Add(1)
	pass := false
	go func() {
		MustNotRecv(t, self, mangos.ErrCanceled)
		pass = true
		wg.Done()
	}()
	time.Sleep(time.Millisecond * 50) // to allow go routine to run
	MustSucceed(t, self.Send([]byte("second")))
	wg.Wait()
	MustSucceed(t, self.Close())
	MustBeTrue(t, pass)
}

// This test demonstrates that sending a second survey discards any received
// messages from the earlier survey.
func TestSurveyorCancelDiscard(t *testing.T) {
	s := GetSocket(t, NewSocket)
	// r1's message will get into the queue, but r2's will be sent
	// after we have canceled -- neither should get through
	r1 := GetSocket(t, respondent.NewSocket)
	r2 := GetSocket(t, respondent.NewSocket)
	a := AddrTestInp()
	MustSucceed(t, s.Listen(a))
	MustSucceed(t, r1.Dial(a))
	MustSucceed(t, r2.Dial(a))
	time.Sleep(time.Millisecond * 20)
	MustSucceed(t, s.SetOption(mangos.OptionSurveyTime, time.Second))
	MustSucceed(t, s.SetOption(mangos.OptionRecvDeadline, time.Millisecond*10))
	MustSucceed(t, s.Send([]byte("first")))
	v1, e := r1.Recv()
	MustSucceed(t, e)
	MustBeTrue(t, string(v1) == "first")
	MustSucceed(t, r1.Send([]byte("reply1")))
	v2, e := r2.Recv()
	MustSucceed(t, e)
	MustBeTrue(t, string(v2) == "first")

	time.Sleep(time.Millisecond * 10)
	MustSucceed(t, s.Send([]byte("second")))
	time.Sleep(time.Millisecond * 10) // to allow async cancel to finish
	MustSucceed(t, r2.Send([]byte("reply2")))
	v, e := s.Recv()
	MustBeError(t, e, mangos.ErrRecvTimeout)
	MustBeNil(t, v)
	MustSucceed(t, s.Close())
	MustSucceed(t, r1.Close())
	MustSucceed(t, r2.Close())
}

// This test demonstrates that closing the socket aborts outstanding receives.
func TestSurveyorCloseRx(t *testing.T) {
	s := GetSocket(t, NewSocket)
	MustSucceed(t, s.SetOption(mangos.OptionSurveyTime, time.Second))
	MustSucceed(t, s.Send([]byte("first")))
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
	time.Sleep(time.Millisecond * 50) // to allow go routine to run
	MustSucceed(t, s.Close())
	wg.Wait()
	MustBeTrue(t, pass)
}

// This test demonstrates that surveys expire on their own.
func TestSurveyExpire(t *testing.T) {
	s := GetSocket(t, NewSocket)
	MustSucceed(t, s.SetOption(mangos.OptionSurveyTime, time.Millisecond*10))
	MustSucceed(t, s.Send([]byte("first")))
	v, e := s.Recv()
	MustBeError(t, e, mangos.ErrProtoState)
	MustBeNil(t, v)
	MustSucceed(t, s.Close())
}

// This test demonstrates that we can keep sending even if the pipes are full.
func TestSurveyorBestEffortSend(t *testing.T) {
	s := GetSocket(t, NewSocket)
	r := GetSocket(t, respondent.NewSocket)
	a := AddrTestInp()
	MustSucceed(t, s.SetOption(mangos.OptionWriteQLen, 1))
	MustSucceed(t, s.Listen(a))
	MustSucceed(t, r.Dial(a))

	MustSucceed(t, s.SetOption(mangos.OptionSurveyTime, time.Second))

	time.Sleep(time.Millisecond * 10)
	for i := 0; i < 100; i++ {
		MustSucceed(t, s.Send([]byte{byte(i)}))
	}
	MustSucceed(t, s.Close())
}

// This test demonstrates that if too many responses are received they will
// be dropped.
func TestSurveyorRxDiscard(t *testing.T) {
	a := AddrTestInp()
	s := GetSocket(t, NewSocket)
	MustSucceed(t, s.SetOption(mangos.OptionReadQLen, 2))
	MustSucceed(t, s.SetOption(mangos.OptionRecvDeadline, time.Millisecond*10))
	MustSucceed(t, s.SetOption(mangos.OptionSurveyTime, time.Second))
	MustSucceed(t, s.Listen(a))

	nresp := 100
	var rs []mangos.Socket
	var wg sync.WaitGroup
	wg.Add(nresp)
	for i := 0; i < nresp; i++ {
		r := GetSocket(t, respondent.NewSocket)
		MustSucceed(t, r.Dial(a))
		rs = append(rs, r)
		go func() {
			defer wg.Done()
			m, e := r.RecvMsg()
			MustSucceed(t, e)
			MustSucceed(t, r.SendMsg(m))
		}()
	}
	time.Sleep(time.Millisecond * 10)
	MustSucceed(t, s.Send([]byte{'a'}))
	wg.Wait()
	time.Sleep(time.Millisecond * 10)
	nrecv := 0
	for {
		m, e := s.RecvMsg()
		if e != nil {
			MustBeError(t, e, mangos.ErrRecvTimeout)
			break
		}
		m.Free()
		nrecv++
	}
	MustBeTrue(t, nrecv == 2) // must match the queue length
	for _, r := range rs {
		MustSucceed(t, r.Close())
	}
	MustSucceed(t, s.Close())
}

// This test demonstrates that we can send surveys to a bunch of peers,
// and collect responses from them all.
func TestSurveyorBroadcast(t *testing.T) {
	a := AddrTestInp()

	s := GetSocket(t, NewSocket)
	MustSucceed(t, s.Listen(a))

	// note the total number of messages exchanged will be nresp * repeat
	nresp := 100
	repeat := 300

	MustSucceed(t, s.SetOption(mangos.OptionReadQLen, nresp))
	r := make([]mangos.Socket, 0, nresp)
	var wg sync.WaitGroup
	wg.Add(nresp)

	for i := 0; i < nresp; i++ {
		x := GetSocket(t, respondent.NewSocket)
		MustSucceed(t, x.Dial(a))
		r = append(r, x)

		go func(idx int) {
			defer wg.Done()

			for num := 0; num < repeat; num++ {
				v, e := x.Recv()
				MustSucceed(t, e)
				MustBeTrue(t, string(v) == "survey")
				body := make([]byte, 8)
				binary.BigEndian.PutUint32(body, uint32(idx))
				binary.BigEndian.PutUint32(body[4:], uint32(num))
				MustSucceed(t, x.Send(body))
			}
		}(i)
	}

	time.Sleep(time.Millisecond * 100)

	MustSucceed(t, s.SetOption(mangos.OptionSurveyTime, time.Millisecond*200))
	MustSucceed(t, s.SetOption(mangos.OptionReadQLen, nresp))
	MustSucceed(t, s.SetOption(mangos.OptionRecvDeadline, time.Millisecond*500))

	recv := make([]int, nresp)

	for num := 0; num < repeat; num++ {
		MustSucceed(t, s.Send([]byte("survey")))
		for i := 0; i < nresp; i++ {
			v, e := s.Recv()
			MustSucceed(t, e)
			MustBeTrue(t, len(v) == 8)
			peer := binary.BigEndian.Uint32(v)
			MustBeTrue(t, peer < uint32(nresp))
			MustBeTrue(t, binary.BigEndian.Uint32(v[4:]) == uint32(num))
			MustBeTrue(t, recv[peer] == num)
			recv[peer]++
		}

		// We could wait for the survey to timeout, but we don't.
	}
	wg.Wait()

	MustSucceed(t, s.Close())
	for i := 0; i < nresp; i++ {
		MustSucceed(t, r[i].Close())
	}
}

func TestSurveyorContextClosed(t *testing.T) {
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

func TestSurveyorContextCloseAbort(t *testing.T) {
	s := GetSocket(t, NewSocket)
	defer MustClose(t, s)

	c, e := s.OpenContext()
	MustSucceed(t, e)
	MustNotBeNil(t, c)
	MustSucceed(t, c.SetOption(mangos.OptionRecvDeadline, time.Second))

	// To get us an id.
	MustSucceed(t, c.Send([]byte{}))
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(time.Millisecond * 50)
		MustSucceed(t, c.Close())
	}()

	_, e = c.Recv()
	MustBeError(t, e, mangos.ErrClosed)
}

// This sets up a bunch of contexts to run in parallel, and verifies that
// they all seem to run with no mis-deliveries.
func TestSurveyorMultiContexts(t *testing.T) {
	count := 30
	repeat := 20

	s := GetSocket(t, NewSocket)
	r := GetSocket(t, respondent.NewSocket)
	MustSucceed(t, s.SetOption(mangos.OptionReadQLen, count*repeat))
	MustSucceed(t, r.SetOption(mangos.OptionReadQLen, count*repeat))
	MustSucceed(t, r.SetOption(mangos.OptionWriteQLen, count*repeat))
	MustSucceed(t, s.SetOption(mangos.OptionWriteQLen, count*repeat))

	a := AddrTestInp()

	MustSucceed(t, r.Listen(a))
	MustSucceed(t, s.Dial(a))

	// Make sure we have dialed properly
	time.Sleep(time.Millisecond * 20)

	recv := make([]int, count)
	ctxs := make([]mangos.Context, 0, count)
	var wg1 sync.WaitGroup
	var wg2 sync.WaitGroup
	wg1.Add(count)
	fn := func(index int) {
		defer wg1.Done()
		c, e := s.OpenContext()
		MustSucceed(t, e)
		MustNotBeNil(t, c)

		ctxs = append(ctxs, c)
		topic := make([]byte, 4)
		binary.BigEndian.PutUint32(topic, uint32(index))

		MustSucceed(t, c.SetOption(mangos.OptionRecvDeadline, time.Minute/4))
		MustSucceed(t, c.SetOption(mangos.OptionSurveyTime, time.Minute))

		for i := 0; i < repeat; i++ {
			MustSucceed(t, c.Send(topic))
			m, e := c.RecvMsg()
			MustSucceed(t, e)
			MustBeTrue(t, len(m.Body) == 4)
			MustBeTrue(t, bytes.Equal(m.Body, topic))
			recv[index]++
		}
	}

	for i := 0; i < count; i++ {
		go fn(i)
	}

	wg2.Add(1)
	go func() {
		defer wg2.Done()
		var e error
		var m *mangos.Message
		for {
			if m, e = r.RecvMsg(); e != nil {
				break
			}
			if e = r.SendMsg(m); e != nil {
				break
			}
		}
		MustBeError(t, e, mangos.ErrClosed)
	}()

	// Give time for everything to be delivered.
	wg1.Wait()
	MustSucceed(t, r.Close())
	wg2.Wait()
	MustSucceed(t, s.Close())

	for i := 0; i < count; i++ {
		MustBeTrue(t, recv[i] == repeat)
	}
}

func TestSurveyorRecvGarbage(t *testing.T) {
	self := GetSocket(t, NewSocket)
	mock, _ := MockConnect(t, self)
	expire := time.Millisecond * 20

	MustSucceed(t, self.SetOption(mangos.OptionRecvDeadline, expire))

	MustSendString(t, self, "")
	MockMustSendStr(t, mock, "abc", expire)
	MustNotRecv(t, self, mangos.ErrRecvTimeout)

	var msg []byte
	// No header
	MustSendString(t, self, "")
	MockMustSend(t, mock, msg, expire)
	MustNotRecv(t, self, mangos.ErrRecvTimeout)

	// No request ID
	MustSendString(t, self, "")
	msg = append(msg, 0, 1, 2, 3)
	MockMustSend(t, mock, msg, expire)
	MustNotRecv(t, self, mangos.ErrRecvTimeout)

	// Also send a bogus header -- no request ID
	MustSendString(t, self, "")
	MockMustSendStr(t, mock, "\001\002\003\004", time.Second)
	MustNotRecv(t, self, mangos.ErrRecvTimeout)

	MustSucceed(t, self.Close())
}
