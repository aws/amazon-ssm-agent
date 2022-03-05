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
// WITHOUT WARRANTIES O R CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package macat

import (
	"bytes"
	"encoding/binary"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/bus"
	"go.nanomsg.org/mangos/v3/protocol/pair"
	"go.nanomsg.org/mangos/v3/protocol/pub"
	"go.nanomsg.org/mangos/v3/protocol/pull"
	"go.nanomsg.org/mangos/v3/protocol/push"
	"go.nanomsg.org/mangos/v3/protocol/rep"
	"go.nanomsg.org/mangos/v3/protocol/req"
	"go.nanomsg.org/mangos/v3/protocol/respondent"
	"go.nanomsg.org/mangos/v3/protocol/star"
	"go.nanomsg.org/mangos/v3/protocol/sub"
	"go.nanomsg.org/mangos/v3/protocol/surveyor"
	"go.nanomsg.org/mangos/v3/protocol/xreq"

	. "go.nanomsg.org/mangos/v3/internal/test"
)

func MustFailLike(t *testing.T, e error, s string) {
	if e != nil && strings.Contains(e.Error(), s) {
		return
	}
	t.Fatalf("Failure expected %s got %v", s, e)
}

func Test_mustSucceed(t *testing.T) {
	defer func() {
		pass := false
		if r := recover(); r != nil {
			pass = true
		}
		MustBeTrue(t, pass)
	}()
	mustSucceed(mangos.ErrBadProto)
}

func TestApp_Initialize(t *testing.T) {
	a := &App{}
	a.Initialize()
}

func TestApp_Help(t *testing.T) {
	a := &App{}
	a.Initialize()
	h := a.Help()
	MustBeTrue(t, len(h) > 20)
}

func TestApp_Run(t *testing.T) {
	a := &App{}
	a.Initialize()
	e := a.Run()
	MustFail(t, e)
}

func TestApp_Run2(t *testing.T) {
	a := &App{}
	a.Initialize()
	e := a.Run("-v", "--verbose", "-v", "-q")
	MustFail(t, e)
	MustBeTrue(t, a.verbose == 2)
}

func TestApp_Run3(t *testing.T) {
	a := &App{}
	a.Initialize()
	MustFail(t, a.Run("--push", "--pull"))
	a.Initialize()
	MustFail(t, a.Run("--pub", "--sub"))
	a.Initialize()
	MustFail(t, a.Run("--req", "--rep"))
	a.Initialize()
	MustFail(t, a.Run("--respondent", "--surveyor"))
	a.Initialize()
	MustFail(t, a.Run("--bus", "--pair"))
	a.Initialize()
	MustFail(t, a.Run("--star", "--star"))

}

func TestApp_Run4(t *testing.T) {
	a := &App{}
	a.Initialize()
	addr := AddrTestInp() // Only works as we are in the same process!
	rx := GetSocket(t, sub.NewSocket)
	defer MustClose(t, rx)
	MustSucceed(t, rx.SetOption(mangos.OptionRecvDeadline, time.Second))
	MustSucceed(t, rx.SetOption(mangos.OptionSubscribe, ""))

	MustSucceed(t, rx.Listen(addr))
	go func() {
		e := a.Run("--pub", "--connect", addr, "--data", "abc", "-d", "20ms")
		MustSucceed(t, e)
	}()
	MustRecvString(t, rx, "abc")
	MustClose(t, a.sock)
}

func TestApp_Run5(t *testing.T) {
	a := &App{}
	a.Initialize()
	port := strconv.Itoa(int(NextPort()))
	addr := "tcp://127.0.0.1:" + port
	rx := GetSocket(t, push.NewSocket)
	defer MustClose(t, rx)
	MustSucceed(t, rx.SetOption(mangos.OptionSendDeadline, time.Second))

	b := &strings.Builder{}
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		a.stdOut = b
		e := a.Run("--pull", "-L", port, "--raw", "--recv-timeout", "2")
		MustFailLike(t, e, mangos.ErrClosed.Error())
	}()
	time.Sleep(time.Millisecond * 20)
	MustSucceed(t, rx.Dial(addr))
	MustSendString(t, rx, "abc")
	MustSendString(t, rx, "def")
	time.Sleep(time.Millisecond * 50)
	MustClose(t, a.sock)
	MustBeTrue(t, b.String() == "abcdef")
	wg.Wait()
}

func TestApp_Run6(t *testing.T) {
	a := &App{}
	a.Initialize()
	port := strconv.Itoa(int(NextPort()))
	addr := "tcp://127.0.0.1:" + port
	rx := GetSocket(t, bus.NewSocket)
	defer MustClose(t, rx)
	MustSucceed(t, rx.Listen(addr))

	b := &strings.Builder{}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		a.stdOut = b
		e := a.Run("--bus", "-l", port, "-A", "--recv-timeout", "2")
		MustFailLike(t, e, mangos.ErrClosed.Error())
	}()
	time.Sleep(time.Millisecond * 20)
	MustSucceed(t, rx.Dial(addr))
	MustSendString(t, rx, "abc")
	MustSendString(t, rx, "def\x01")
	time.Sleep(time.Millisecond * 50)
	MustClose(t, a.sock)
	MustBeTrue(t, b.String() == "abc\ndef.\n")
	wg.Wait()
}

func TestApp_Run7(t *testing.T) {
	a := &App{}
	a.Initialize()
	addr := AddrTestInp()
	rx := GetSocket(t, star.NewSocket)
	defer MustClose(t, rx)
	MustSucceed(t, rx.Listen(addr))

	b := &strings.Builder{}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		a.stdOut = b
		e := a.Run("--star", "--connect", addr, "-Q", "--recv-timeout", "2")
		MustFailLike(t, e, mangos.ErrClosed.Error())
	}()
	time.Sleep(time.Millisecond * 20)
	MustSendString(t, rx, "a\nbc")
	MustSendString(t, rx, "d\ref")
	MustSendString(t, rx, "g\\hi")
	MustSendString(t, rx, "j\"kl")
	MustSendString(t, rx, "\x01\x02")

	time.Sleep(time.Millisecond * 50)
	MustClose(t, a.sock)
	MustBeTrue(t, b.String() == "a\\nbc\nd\\ref\ng\\\\hi\nj\\\"kl\n\\x01\\x02\n")
	wg.Wait()
}

func TestApp_Run8(t *testing.T) {
	a := &App{}
	a.Initialize()
	addr := AddrTestIPC()
	path := strings.TrimPrefix(addr, "ipc://")
	rx := GetSocket(t, req.NewSocket)
	defer MustClose(t, rx)

	b := &strings.Builder{}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		a.stdOut = b
		e := a.Run("--rep", "-X", path, "-A", "--data", "pong", "--send-timeout", "2", "--recv-timeout", "2")
		MustFailLike(t, e, mangos.ErrClosed.Error())
	}()
	time.Sleep(time.Millisecond * 20)
	MustSucceed(t, rx.Dial(addr))
	MustSendString(t, rx, "abc")
	MustRecvString(t, rx, "pong")
	MustSendString(t, rx, "def")
	MustRecvString(t, rx, "pong")

	time.Sleep(time.Millisecond * 50)
	MustClose(t, a.sock)
	MustBeTrue(t, b.String() == "abc\ndef\n")
	wg.Wait()
}

func TestApp_Ipc(t *testing.T) {
	a := &App{}
	a.Initialize()
	rx := GetSocket(t, req.NewSocket)
	defer MustClose(t, rx)
	MustSucceed(t, rx.SetOption(mangos.OptionRecvDeadline, time.Millisecond*40))
	addr := AddrTestIPC()
	path := strings.TrimPrefix(addr, "ipc://")

	b := &strings.Builder{}
	var wg sync.WaitGroup
	wg.Add(1)
	MustSucceed(t, rx.Listen(addr))
	go func() {
		defer wg.Done()
		a.stdOut = b
		_ = a.Run("--rep", "-x", path, "-A", "--recv-timeout", "2")
	}()
	time.Sleep(time.Millisecond * 20)
	MustSendString(t, rx, "query")
	MustSendString(t, rx, "again")
	MustNotRecv(t, rx, mangos.ErrRecvTimeout)
	MustClose(t, a.sock)
	MustBeTrue(t, b.String() == "query\nagain\n")
	wg.Wait()
}

func TestApp_Rep_NoData(t *testing.T) {
	a := &App{}
	a.Initialize()
	addr := AddrTestInp()
	rx := GetSocket(t, req.NewSocket)
	defer MustClose(t, rx)
	MustSucceed(t, rx.SetOption(mangos.OptionRecvDeadline, time.Millisecond*40))

	b := &strings.Builder{}
	var wg sync.WaitGroup
	wg.Add(1)
	MustSucceed(t, rx.Listen(addr))
	go func() {
		defer wg.Done()
		a.stdOut = b
		_ = a.Run("--rep", "--connect", addr, "-A", "--recv-timeout", "2")
	}()
	time.Sleep(time.Millisecond * 20)
	MustSendString(t, rx, "query")
	MustSendString(t, rx, "again")
	MustNotRecv(t, rx, mangos.ErrRecvTimeout)
	MustClose(t, a.sock)
	MustBeTrue(t, b.String() == "query\nagain\n")
	wg.Wait()
}

func TestApp_Rep_RecvTimeout(t *testing.T) {
	a := &App{}
	a.Initialize()
	addr := AddrTestInp()
	rx := GetSocket(t, req.NewSocket)
	defer MustClose(t, rx)
	MustSucceed(t, rx.SetOption(mangos.OptionRecvDeadline, time.Millisecond*40))

	b := &strings.Builder{}
	var wg sync.WaitGroup
	wg.Add(1)
	MustSucceed(t, rx.Listen(addr))
	go func() {
		defer wg.Done()
		a.stdOut = b
		e := a.Run("--rep", "--connect", addr, "-A", "--recv-timeout=100ms", "-DYES")
		MustSucceed(t, e)
	}()
	wg.Wait()
}

func TestApp_Rep_SendTimeout(t *testing.T) {
	a := &App{}
	a.Initialize()
	addr := AddrTestInp()
	rx := GetSocket(t, xreq.NewSocket)
	defer MustClose(t, rx)
	MustSucceed(t, rx.SetOption(mangos.OptionRecvDeadline, time.Millisecond*40))
	MustSucceed(t, rx.SetOption(mangos.OptionReadQLen, 0))
	a.sock = GetSocket(t, rep.NewSocket)
	MustSucceed(t, a.sock.SetOption(mangos.OptionWriteQLen, 0))

	b := &strings.Builder{}
	var wg sync.WaitGroup
	wg.Add(1)
	MustSucceed(t, rx.Listen(addr))
	go func() {
		defer wg.Done()
		a.stdOut = b
		e := a.Run("--connect", addr, "-A", "-i30ms", "--recv-timeout=1s", "--send-timeout=10ms", "-DYES")
		MustFailLike(t, e, "send time out")
	}()
	m := mangos.NewMessage(0)
	m.Body = append(m.Body, 0x80, 0, 0, 1, 'h', 'e', 'l', 'l', 'o')
	MustSendMsg(t, rx, m.Dup())
	time.Sleep(time.Millisecond * 20)
	MustSendMsg(t, rx, m.Dup())
	time.Sleep(time.Millisecond * 20)
	MustSendMsg(t, rx, m.Dup())
	time.Sleep(time.Millisecond * 20)
	MustSendMsg(t, rx, m.Dup())
	wg.Wait()
}

func TestApp_Req_SendTimeout(t *testing.T) {
	a := &App{}
	a.Initialize()
	AddMockTransport()
	addr := AddrMock()
	a.sock = GetSocket(t, req.NewSocket)

	b := &strings.Builder{}
	a.stdOut = b
	e := a.Run("--bind", addr, "-A", "--recv-timeout=10ms", "--send-timeout=10ms", "-i30ms", "-DYES")
	MustFailLike(t, e, "send time out")
}

func TestApp_Pub_NoData(t *testing.T) {
	a := &App{}
	a.Initialize()
	addr := AddrTestInp()
	rx := GetSocket(t, sub.NewSocket)
	defer MustClose(t, rx)

	b := &strings.Builder{}
	var wg sync.WaitGroup
	wg.Add(1)
	MustSucceed(t, rx.Listen(addr))
	go func() {
		defer wg.Done()
		a.stdOut = b
		e := a.Run("--pub",
			"--connect", addr,
			"-d", "20ms")
		MustFailLike(t, e, "no data")
	}()
	wg.Wait()
}

func TestApp_Sub(t *testing.T) {
	a := &App{}
	a.Initialize()
	addr := AddrTestInp()
	rx := GetSocket(t, pub.NewSocket)
	defer MustClose(t, rx)

	b := &strings.Builder{}
	var wg sync.WaitGroup
	wg.Add(1)
	MustSucceed(t, rx.Listen(addr))
	go func() {
		defer wg.Done()
		a.stdOut = b
		_ = a.Run("--sub", "--subscribe=once", "--connect", addr, "-A", "--recv-timeout=2s")
	}()
	time.Sleep(time.Millisecond * 40)
	MustSendString(t, rx, "twice is coincidence")
	MustSendString(t, rx, "once upon a time")
	time.Sleep(time.Millisecond * 40)
	MustClose(t, a.sock)
	wg.Wait()
	MustBeTrue(t, b.String() == "once upon a time\n")
}

func TestApp_Sub_Wild(t *testing.T) {
	a := &App{}
	a.Initialize()
	addr := AddrTestInp()
	rx := GetSocket(t, pub.NewSocket)
	defer MustClose(t, rx)

	b := &strings.Builder{}
	var wg sync.WaitGroup
	wg.Add(1)
	MustSucceed(t, rx.Listen(addr))
	go func() {
		defer wg.Done()
		a.stdOut = b
		_ = a.Run("--sub", "--connect", addr, "-A", "--recv-timeout=2s")
	}()
	time.Sleep(time.Millisecond * 40)
	MustSendString(t, rx, "once upon a time")
	MustSendString(t, rx, "twice is coincidence")
	time.Sleep(time.Millisecond * 40)
	MustClose(t, a.sock)
	wg.Wait()
	MustBeTrue(t, b.String() == "once upon a time\ntwice is coincidence\n")
}

func TestApp_Surveyor(t *testing.T) {
	a := &App{}
	a.Initialize()
	addr := AddrTestInp()
	rx := GetSocket(t, respondent.NewSocket)
	defer MustClose(t, rx)

	b := &strings.Builder{}
	var wg sync.WaitGroup
	wg.Add(1)
	MustSucceed(t, rx.Listen(addr))
	go func() {
		defer wg.Done()
		a.stdOut = b
		_ = a.Run("--surveyor", "-A",
			"--connect", addr,
			"--data", "query",
			"-d", "20ms",
			"--send-timeout", "2",
			"--recv-timeout", "2")
	}()
	time.Sleep(time.Millisecond * 20)
	MustRecvString(t, rx, "query")
	MustSendString(t, rx, "yes")

	time.Sleep(time.Millisecond * 50)
	MustClose(t, a.sock)
	MustBeTrue(t, b.String() == "yes\n")
	wg.Wait()
}

func TestApp_Surveyor_Count(t *testing.T) {
	a := &App{}
	a.Initialize()
	addr := AddrTestInp()
	rx := GetSocket(t, respondent.NewSocket)
	MustSucceed(t, rx.SetOption(mangos.OptionRecvDeadline, time.Second))
	defer MustClose(t, rx)

	b := &strings.Builder{}
	var wg sync.WaitGroup
	wg.Add(1)
	MustSucceed(t, rx.Listen(addr))
	go func() {
		defer wg.Done()
		a.stdOut = b
		e := a.Run("--surveyor", "-A",
			"--connect", addr,
			"--data", "query",
			"-d", "20ms",
			"--count", "2",
			"--send-interval", "20ms",
			"--send-timeout", "2",
			"--recv-timeout", "2")
		MustSucceed(t, e)
	}()
	MustRecvString(t, rx, "query")
	MustSendString(t, rx, "yes")
	MustRecvString(t, rx, "query")
	MustSendString(t, rx, "still")
	wg.Wait()

	MustBeTrue(t, b.String() == "yes\nstill\n")
}

func TestApp_Surveyor_Expire1(t *testing.T) {
	a := &App{}
	a.Initialize()
	a.sock = GetSocket(t, surveyor.NewSocket)
	MustSucceed(t, a.sock.SetOption(mangos.OptionSurveyTime, time.Millisecond))
	addr := AddrTestInp()
	rx := GetSocket(t, respondent.NewSocket)
	MustSucceed(t, rx.SetOption(mangos.OptionRecvDeadline, time.Second))
	defer MustClose(t, rx)

	b := &strings.Builder{}
	var wg sync.WaitGroup
	wg.Add(1)
	MustSucceed(t, rx.Listen(addr))
	go func() {
		defer wg.Done()
		a.stdOut = b
		e := a.Run("-A",
			"--connect", addr,
			"--data", "query",
			"-d", "20ms",
			"--count=1",
			"--send-timeout", "2",
			"--recv-timeout", "2")
		MustSucceed(t, e)
	}()
	MustRecvString(t, rx, "query")
	time.Sleep(time.Millisecond * 20)
	wg.Wait()

	MustBeTrue(t, b.String() == "")
}

func TestApp_Surveyor_Expire2(t *testing.T) {
	a := &App{}
	a.Initialize()
	a.sock = GetSocket(t, surveyor.NewSocket)
	MustSucceed(t, a.sock.SetOption(mangos.OptionSurveyTime, time.Millisecond))
	addr := AddrTestInp()
	rx := GetSocket(t, respondent.NewSocket)
	MustSucceed(t, rx.SetOption(mangos.OptionRecvDeadline, time.Second))
	defer MustClose(t, rx)

	b := &strings.Builder{}
	var wg sync.WaitGroup
	wg.Add(1)
	MustSucceed(t, rx.Listen(addr))
	go func() {
		defer wg.Done()
		a.stdOut = b
		e := a.Run("-A",
			"--connect", addr,
			"--data", "query",
			"-d", "20ms",
			"--count=2",
			"--send-interval", "20ms",
			"--send-timeout", "2",
			"--recv-timeout", "2")
		MustSucceed(t, e)
	}()
	MustRecvString(t, rx, "query")
	time.Sleep(time.Millisecond * 20)
	wg.Wait()

	MustBeTrue(t, b.String() == "")
}

func TestApp_Surveyor_Expire3(t *testing.T) {
	a := &App{}
	a.Initialize()
	addr := AddrTestInp()
	rx := GetSocket(t, respondent.NewSocket)
	MustSucceed(t, rx.SetOption(mangos.OptionRecvDeadline, time.Second))
	defer MustClose(t, rx)

	b := &strings.Builder{}
	var wg sync.WaitGroup
	wg.Add(1)
	MustSucceed(t, rx.Listen(addr))
	go func() {
		defer wg.Done()
		a.stdOut = b
		_ = a.Run("-A", "--surveyor",
			"--connect", addr,
			"--data", "query",
			"-d", "20ms",
			"--send-interval", "20ms",
			"--send-timeout", "2",
			"--recv-timeout", "2ms")
	}()
	MustRecvString(t, rx, "query")
	MustRecvString(t, rx, "query")
	MustRecvString(t, rx, "query")
	MustClose(t, a.sock)

	time.Sleep(time.Millisecond * 20)
	wg.Wait()

	MustBeTrue(t, b.String() == "")
}

func TestApp_Respondent(t *testing.T) {
	a := &App{}
	a.Initialize()
	addr := AddrTestInp()
	rx := GetSocket(t, surveyor.NewSocket)
	defer MustClose(t, rx)

	b := &strings.Builder{}
	var wg sync.WaitGroup
	wg.Add(1)
	MustSucceed(t, rx.Listen(addr))
	go func() {
		defer wg.Done()
		a.stdOut = b
		_ = a.Run("--respondent", "--connect", addr, "-A", "--data", "yes", "--send-timeout", "2", "--recv-timeout", "2")
	}()
	time.Sleep(time.Millisecond * 20)
	MustSendString(t, rx, "query")
	MustRecvString(t, rx, "yes")
	MustSendString(t, rx, "again")
	MustRecvString(t, rx, "yes")
	MustClose(t, a.sock)
	MustBeTrue(t, b.String() == "query\nagain\n")
	wg.Wait()
}

func TestApp_Respondent_NoData(t *testing.T) {
	a := &App{}
	a.Initialize()
	addr := AddrTestInp()
	rx := GetSocket(t, surveyor.NewSocket)
	defer MustClose(t, rx)
	MustSucceed(t, rx.SetOption(mangos.OptionRecvDeadline, time.Millisecond*40))

	b := &strings.Builder{}
	var wg sync.WaitGroup
	wg.Add(1)
	MustSucceed(t, rx.Listen(addr))
	go func() {
		defer wg.Done()
		a.stdOut = b
		_ = a.Run("--respondent", "--connect", addr, "-A", "--recv-timeout", "2")
	}()
	time.Sleep(time.Millisecond * 20)
	MustSendString(t, rx, "query")
	MustSendString(t, rx, "again")
	MustNotRecv(t, rx, mangos.ErrRecvTimeout)
	MustClose(t, a.sock)
	MustBeTrue(t, b.String() == "query\nagain\n")
	wg.Wait()
}

func TestApp_Pair(t *testing.T) {
	a := &App{}
	a.Initialize()
	addr := AddrTestInp()
	rx := GetSocket(t, pair.NewSocket)
	defer MustClose(t, rx)

	b := &strings.Builder{}
	var wg sync.WaitGroup
	wg.Add(1)
	MustSucceed(t, rx.Listen(addr))
	go func() {
		defer wg.Done()
		a.stdOut = b
		_ = a.Run("--pair", "--connect", addr, "-d", "20ms", "-A", "--data", "ping", "--send-timeout", "2", "--recv-timeout", "100ms")
	}()
	MustRecvString(t, rx, "ping")
	MustSendString(t, rx, "pong")
	wg.Wait()
	MustBeTrue(t, b.String() == "pong\n")
}

func TestApp_Pair_NoData(t *testing.T) {
	a := &App{}
	a.Initialize()
	addr := AddrTestInp()
	rx := GetSocket(t, pair.NewSocket)
	defer MustClose(t, rx)

	b := &strings.Builder{}
	var wg sync.WaitGroup
	wg.Add(1)
	MustSucceed(t, rx.Listen(addr))
	go func() {
		defer wg.Done()
		a.stdOut = b
		_ = a.Run("--pair", "--connect", addr, "-d", "20ms", "-A", "--recv-timeout", "50ms")
	}()
	MustSendString(t, rx, "ping")
	wg.Wait()
	MustBeTrue(t, b.String() == "ping\n")
}

func TestApp_Push(t *testing.T) {
	a := &App{}
	a.Initialize()
	addr := AddrTestInp()
	rx := GetSocket(t, pull.NewSocket)
	defer MustClose(t, rx)
	MustSucceed(t, rx.SetOption(mangos.OptionRecvDeadline, time.Second))

	b := &strings.Builder{}
	var wg sync.WaitGroup
	wg.Add(1)
	MustSucceed(t, rx.Listen(addr))
	go func() {
		defer wg.Done()
		a.stdOut = b
		e := a.Run("--push", "--connect", addr, "-d=20ms", "--data=ping", "--send-timeout=2",
			"-i=20ms")
		MustFailLike(t, e, mangos.ErrClosed.Error())
	}()
	MustRecvString(t, rx, "ping")
	MustRecvString(t, rx, "ping")
	MustRecvString(t, rx, "ping")
	MustClose(t, a.sock)
	wg.Wait()
	MustBeTrue(t, b.String() == "")
}

func TestApp_PushNoData(t *testing.T) {
	a := &App{}
	a.Initialize()
	addr := AddrTestInp()
	e := a.Run("--push", "--bind", addr, "--send-timeout", "50ms")
	MustFailLike(t, e, "no data")
}

func TestApp_MockProto(t *testing.T) {
	a := &App{}
	a.Initialize()
	a.sock = GetMockSocket()
	addr := AddrTestInp()
	e := a.Run("--bind", addr, "--recv-timeout", "50ms")
	MustFailLike(t, e, "unknown protocol")
}

func TestApp_SendFile(t *testing.T) {
	f, err := ioutil.TempFile("", "macattest")
	MustSucceed(t, err)
	defer func() { _ = os.Remove(f.Name()) }()

	data := `This is a test file
It should contain test data.`
	_, err = f.Write([]byte(data))
	MustSucceed(t, err)

	a := &App{}
	a.Initialize()
	addr := AddrTestInp()
	rx := GetSocket(t, pull.NewSocket)
	defer MustClose(t, rx)
	MustSucceed(t, rx.SetOption(mangos.OptionRecvDeadline, time.Second))

	b := &strings.Builder{}
	var wg sync.WaitGroup
	wg.Add(1)
	MustSucceed(t, rx.Listen(addr))
	go func() {
		defer wg.Done()
		a.stdOut = b
		_ = a.Run("--push", "--connect", addr, "-d=20ms", "--file", f.Name(), "--send-timeout=2")
	}()
	m := MustRecvMsg(t, rx)
	MustBeTrue(t, string(m.Body) == data)
	MustClose(t, a.sock)
	wg.Wait()
}

func TestApp_SendFile_NotExist(t *testing.T) {
	a := &App{}
	a.Initialize()

	err := a.Run("--push", "--bind=inproc://none", "--file=nosuchfile", "--send-timeout=ABC")
	MustFail(t, err)
	MustBeTrue(t, os.IsNotExist(err))
}

func TestApp_SendFile_DataSet(t *testing.T) {
	a := &App{}
	a.Initialize()

	err := a.Run("--push", "--bind=inproc://none", "--data=1", "--file=nosuchfile", "--send-timeout=ABC")
	MustFailLike(t, err, "data or file already set")
}

func TestApp_Data_DataSet(t *testing.T) {
	a := &App{}
	a.Initialize()

	err := a.Run("--push", "--bind=inproc://none", "--data=1", "--data=2", "--send-timeout=ABC")
	MustFailLike(t, err, "data or file already set")
}

func TestApp_NoOutput(t *testing.T) {
	a := &App{}
	a.Initialize()
	addr := AddrTestInp()
	rx := GetSocket(t, req.NewSocket)
	MustSucceed(t, rx.SetOption(mangos.OptionSendDeadline, time.Second))
	MustSucceed(t, rx.SetOption(mangos.OptionRecvDeadline, time.Second))
	defer MustClose(t, rx)

	b := &strings.Builder{}
	var wg sync.WaitGroup
	wg.Add(1)
	MustSucceed(t, rx.Listen(addr))
	go func() {
		defer wg.Done()
		a.stdOut = b
		_ = a.Run("--rep", "--connect", addr,
			"--format=no", "--count=1",
			"--data", "pong", "--recv-timeout=50ms")
	}()
	MustSendString(t, rx, "ping")
	MustRecvString(t, rx, "pong")
	wg.Wait()
	MustBeTrue(t, b.String() == "")
}

func TestApp_Format_Quoted(t *testing.T) {
	a := &App{}
	a.Initialize()
	addr := AddrTestInp()
	rx := GetSocket(t, push.NewSocket)
	defer MustClose(t, rx)

	b := &strings.Builder{}
	var wg sync.WaitGroup
	wg.Add(1)
	MustSucceed(t, rx.Listen(addr))
	go func() {
		defer wg.Done()
		a.stdOut = b
		_ = a.Run("--pull", "--connect", addr, "-Q", "--recv-timeout=50ms", "-d20ms")
	}()
	MustSendString(t, rx, "p\x01\r\n\\\"ing")
	MustSendString(t, rx, "abc")
	cmp := "p\\x01\\r\\n\\\\\\\"ing\nabc\n"
	wg.Wait()
	MustBeTrue(t, b.String() == cmp)
}

func TestApp_Format_Ascii(t *testing.T) {
	a := &App{}
	a.Initialize()
	addr := AddrTestInp()
	rx := GetSocket(t, push.NewSocket)
	defer MustClose(t, rx)

	b := &strings.Builder{}
	var wg sync.WaitGroup
	wg.Add(1)
	MustSucceed(t, rx.Listen(addr))
	go func() {
		defer wg.Done()
		a.stdOut = b
		_ = a.Run("--pull", "--connect", addr, "-A", "--recv-timeout=50ms", "-d20ms")
	}()
	MustSendString(t, rx, "p\x01ing")
	MustSendString(t, rx, "abc\x03")
	cmp := "p.ing\nabc.\n"
	wg.Wait()
	MustBeTrue(t, b.String() == cmp)
}

func TestApp_Format_MsgPack(t *testing.T) {
	a := &App{}
	a.Initialize()
	addr := AddrTestInp()
	rx := GetSocket(t, push.NewSocket)
	defer MustClose(t, rx)

	smallMsg := "abc"
	medMsg := make([]byte, 300)
	for i := 0; i < len(medMsg); i++ {
		medMsg[i] = 'M'
	}
	bigMsg := make([]byte, 1024*1024)
	for i := 0; i < len(bigMsg); i++ {
		bigMsg[i] = 'B'
	}

	b := &strings.Builder{}
	var wg sync.WaitGroup
	wg.Add(1)
	MustSucceed(t, rx.Listen(addr))
	go func() {
		defer wg.Done()
		a.stdOut = b
		_ = a.Run("--pull", "--connect", addr, "--msgpack", "--recv-timeout=50ms", "-d20ms")
	}()
	time.Sleep(time.Millisecond * 20)
	MustSendString(t, rx, smallMsg)
	MustSendString(t, rx, string(medMsg))
	MustSendString(t, rx, string(bigMsg))

	wg.Wait()
	bs := b.String()
	by := []byte(bs)
	MustBeTrue(t, by[0] == 0xc4)
	by = by[1:]
	MustBeTrue(t, by[0] == 3)
	by = by[1:]
	MustBeTrue(t, bytes.Equal(by[:3], []byte{'a', 'b', 'c'}))
	by = by[3:]

	MustBeTrue(t, by[0] == 0xc5)
	by = by[1:]
	MustBeTrue(t, binary.BigEndian.Uint16(by) == 300)
	by = by[2:]
	MustBeTrue(t, bytes.Equal(by[:300], medMsg))
	by = by[300:]

	MustBeTrue(t, by[0] == 0xc6)
	by = by[1:]
	MustBeTrue(t, binary.BigEndian.Uint32(by) == 1024*1024)
	by = by[4:]
	MustBeTrue(t, bytes.Equal(by, bigMsg))
}

func TestApp_No_Address(t *testing.T) {
	a := &App{}
	a.Initialize()

	err := a.Run("--pull", "-A", "--recv-timeout", "50ms")
	MustFailLike(t, err, "no address specified")
}

func TestApp_Subscribe_WrongProto(t *testing.T) {
	a := &App{}
	a.Initialize()

	err := a.Run("--pull", "--bind=inproc://none", "--recv-timeout=50ms", "--subscribe=yes")
	MustFailLike(t, err, "only valid with SUB protocol")
}

func TestApp_Duration_Type(t *testing.T) {
	a := &App{}
	a.Initialize()

	err := a.Run("--pull", "--bind=inproc://none", "--recv-timeout=ABC")
	MustFailLike(t, err, "failure parsing")
}

func TestApp_Format_Wrong(t *testing.T) {
	a := &App{}
	a.Initialize()

	err := a.Run("--pull", "--bind=inproc://none", "--format=wrong", "--recv-timeout=1s")
	MustFailLike(t, err, "invalid format")
}

func TestApp_Format_Twice(t *testing.T) {
	a := &App{}
	a.Initialize()

	err := a.Run("--pull", "--bind=inproc://none", "-A", "-Q", "--recv-timeout=1s")
	MustFailLike(t, err, "format already set")
}

func TestApp_Extra_Args(t *testing.T) {
	a := &App{}
	a.Initialize()

	err := a.Run("--pull", "--bind=inproc://none", "-A", "--recv-timeout=1s", "extra")
	MustFailLike(t, err, "usage: extra arguments")
}

func TestApp_Dial_Bad_Addr(t *testing.T) {
	a := &App{}
	a.Initialize()

	err := a.Run("--pull", "--connect=junk", "-A", "--recv-timeout=1s")
	MustFailLike(t, err, "invalid address")
}

func TestApp_Dial_Fail(t *testing.T) {
	a := &App{}
	a.Initialize()

	err := a.Run("--pull", "--connect=inproc://nobody", "-A", "--recv-timeout=1s")
	MustFailLike(t, err, "connection refused")
}

func TestApp_Dial_NoConfig(t *testing.T) {
	a := &App{}
	a.Initialize()
	addr := AddrTestTLS()

	err := a.Run("--pull", "--connect", addr, "-A", "--recv-timeout=1s")
	MustFailLike(t, err, "no CA cert")
}

func TestApp_Dial_Missing_CAFile(t *testing.T) {
	a := &App{}
	a.Initialize()
	addr := AddrTestTLS()

	err := a.Run("--pull", "--bind", addr, "--cacert=JUNK", "-A", "--recv-timeout=1s")
	MustFail(t, err)
	MustBeTrue(t, os.IsNotExist(err))
}

func TestApp_Dial_Duplicate_KeyFile(t *testing.T) {
	a := &App{}
	a.Initialize()
	addr := AddrTestTLS()

	err := a.Run("--pull", "--bind", addr, "--key=JUNK", "--key=JUNK2", "-A", "--recv-timeout=1s")
	MustFail(t, err)
	MustFailLike(t, err, "key file already set")

}

func TestApp_Dial_Duplicate_CertFile(t *testing.T) {
	a := &App{}
	a.Initialize()
	addr := AddrTestTLS()

	err := a.Run("--pull", "--bind", addr, "--cert=JUNK", "--cert=JUNK2", "-A", "--recv-timeout=1s")
	MustFail(t, err)
	MustFailLike(t, err, "certificate file already set")
}

func TestApp_Dial_Missing_CertFile(t *testing.T) {
	a := &App{}
	a.Initialize()
	addr := AddrTestTLS()

	err := a.Run("--pull", "--bind", addr, "--cert=JUNK", "-A", "--recv-timeout=1s")
	MustFail(t, err)
}

func TestApp_Dial_Bad_CAFile(t *testing.T) {
	f, err := ioutil.TempFile("", "badcafile")
	MustSucceed(t, err)
	defer func() {
		_ = os.Remove(f.Name())
	}()

	_, err = f.Write([]byte{'a', 'b', 'c'})
	MustSucceed(t, err)
	a := &App{}
	a.Initialize()
	addr := AddrTestTLS()

	err = a.Run("--pull", "--bind", addr, "--cacert", f.Name(), "-A", "--recv-timeout=1s")
	MustFail(t, err)
	MustFailLike(t, err, "unable to load CA certs")
}

func TestApp_TLS_Dup_CA(t *testing.T) {
	_, _, keys := GetTLSConfigKeys(t)
	dir, err := ioutil.TempDir("", "keys")
	MustSucceed(t, err)
	defer func() { _ = os.RemoveAll(dir) }()

	cafile := path.Join(dir, "cacert.pem")
	keyfile := path.Join(dir, "key.pem")
	crtfile := path.Join(dir, "cert.pem")

	MustSucceed(t, ioutil.WriteFile(cafile, keys.Root.CertPEM, 0644))
	MustSucceed(t, ioutil.WriteFile(keyfile, keys.Client.KeyPEM, 0644))
	MustSucceed(t, ioutil.WriteFile(crtfile, keys.Client.CertPEM, 0644))
	addr := AddrTestTLS()

	a := &App{}
	a.Initialize()

	err = a.Run("--push", "--connect", addr, "--cacert", cafile,
		"--cacert", cafile, "--key", keyfile, "--cert", crtfile,
		"-d=20ms", "--data=ping", "--count=1")
	MustFailLike(t, err, "cacert already set")
}

func TestApp_Dial_TLS(t *testing.T) {
	cfg, _, keys := GetTLSConfigKeys(t)
	dir, err := ioutil.TempDir("", "keys")
	MustSucceed(t, err)
	defer func() { _ = os.RemoveAll(dir) }()

	cafile := path.Join(dir, "cacert.pem")
	keyfile := path.Join(dir, "key.pem")
	crtfile := path.Join(dir, "cert.pem")

	MustSucceed(t, ioutil.WriteFile(cafile, keys.Root.CertPEM, 0644))
	MustSucceed(t, ioutil.WriteFile(keyfile, keys.Client.KeyPEM, 0644))
	MustSucceed(t, ioutil.WriteFile(crtfile, keys.Client.CertPEM, 0644))
	addr := AddrTestTLS()
	opts := make(map[string]interface{})
	opts[mangos.OptionTLSConfig] = cfg

	sock := GetSocket(t, pull.NewSocket)
	defer MustClose(t, sock)
	MustSucceed(t, sock.SetOption(mangos.OptionRecvDeadline, time.Second))
	MustSucceed(t, sock.ListenOptions(addr, opts))

	a := &App{}
	a.Initialize()
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		err = a.Run("--push", "--connect", addr, "--cacert", cafile,
			"--key", keyfile, "--cert", crtfile,
			"-d=20ms", "--data=ping", "--count=1")
		MustSucceed(t, err)
	}()
	MustRecvString(t, sock, "ping")
	wg.Wait()
}

func TestApp_Dial_TLS_Insecure(t *testing.T) {
	cfg, _, keys := GetTLSConfigKeys(t)
	dir, err := ioutil.TempDir("", "keys")
	MustSucceed(t, err)
	defer func() { _ = os.RemoveAll(dir) }()

	cafile := path.Join(dir, "cacert.pem")
	keyfile := path.Join(dir, "key.pem")
	crtfile := path.Join(dir, "cert.pem")

	MustSucceed(t, ioutil.WriteFile(cafile, keys.Root.CertPEM, 0644))
	MustSucceed(t, ioutil.WriteFile(keyfile, keys.Client.KeyPEM, 0644))
	MustSucceed(t, ioutil.WriteFile(crtfile, keys.Client.CertPEM, 0644))
	addr := AddrTestTLS()
	opts := make(map[string]interface{})
	opts[mangos.OptionTLSConfig] = cfg

	sock := GetSocket(t, pull.NewSocket)
	defer MustClose(t, sock)
	MustSucceed(t, sock.SetOption(mangos.OptionRecvDeadline, time.Second))
	MustSucceed(t, sock.ListenOptions(addr, opts))

	a := &App{}
	a.Initialize()
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		err = a.Run("--push", "--connect", addr, "-k",
			"--key", keyfile, "--cert", crtfile,
			"-d=20ms", "--data=ping", "--count=1")
		MustSucceed(t, err)
	}()
	MustRecvString(t, sock, "ping")
	wg.Wait()
}

func TestApp_Bind_Bad_Addr(t *testing.T) {
	a := &App{}
	a.Initialize()

	err := a.Run("--pull", "--bind=junk", "-A", "--recv-timeout=1s")
	MustFailLike(t, err, "invalid address")
}

func TestApp_Bind_Fail(t *testing.T) {
	a := &App{}
	a.Initialize()

	err := a.Run("--pull", "--bind=bogus://bogus", "-A", "--recv-timeout=1s")
	MustFailLike(t, err, "unsupported transport")
}

func TestApp_Bind_NoConfig(t *testing.T) {
	a := &App{}
	a.Initialize()
	addr := AddrTestTLS()

	err := a.Run("--pull", "--bind", addr, "-A", "--recv-timeout=1s")
	MustFailLike(t, err, "no server cert")
}

func TestApp_Bind_TLS(t *testing.T) {
	_, cfg, keys := GetTLSConfigKeys(t)
	dir, err := ioutil.TempDir("", "keys")
	MustSucceed(t, err)
	defer func() { _ = os.RemoveAll(dir) }()

	keyfile := path.Join(dir, "key.pem")
	crtfile := path.Join(dir, "cert.pem")

	MustSucceed(t, ioutil.WriteFile(keyfile, keys.Server.KeyPEM, 0644))
	MustSucceed(t, ioutil.WriteFile(crtfile, keys.Server.CertPEM, 0644))
	addr := AddrTestTLS()
	opts := make(map[string]interface{})
	opts[mangos.OptionTLSConfig] = cfg

	sock := GetSocket(t, pull.NewSocket)
	defer MustClose(t, sock)
	MustSucceed(t, sock.SetOption(mangos.OptionRecvDeadline, time.Second))

	a := &App{}
	a.Initialize()
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		err = a.Run("--push", "--bind", addr,
			"--key", keyfile, "--cert", crtfile,
			"-d=20ms", "--data=ping", "--count=1")
		MustSucceed(t, err)
	}()
	time.Sleep(time.Millisecond * 20)
	MustSucceed(t, sock.DialOptions(addr, opts))

	MustRecvString(t, sock, "ping")
	wg.Wait()
}
