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

package test

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/pair"
	_ "go.nanomsg.org/mangos/v3/transport/inproc"
)

func TestDialerBadScheme(t *testing.T) {
	self := GetMockSocket()
	defer MustClose(t, self)

	d, e := self.NewDialer("bad://nothere", nil)
	MustBeError(t, e, mangos.ErrBadTran)
	MustBeTrue(t, d == nil)

	// Malformed, needs :// bit
	d, e = self.NewDialer("inproc:nothere", nil)
	MustBeError(t, e, mangos.ErrBadTran)
	MustBeTrue(t, d == nil)
}

func TestDialerAddress(t *testing.T) {
	AddMockTransport()
	self := GetMockSocket()
	defer MustClose(t, self)

	d, e := self.NewDialer(AddrMock(), nil)
	MustSucceed(t, e)
	MustBeTrue(t, d.Address() == AddrMock())
}

func TestDialerSocketOptions(t *testing.T) {
	AddMockTransport()

	VerifyOptionDuration(t, NewMockSocket, mangos.OptionReconnectTime)
	VerifyOptionDuration(t, NewMockSocket, mangos.OptionMaxReconnectTime)
	VerifyOptionBool(t, NewMockSocket, mangos.OptionDialAsynch)
	VerifyOptionInt(t, NewMockSocket, mangos.OptionMaxRecvSize)
}

func TestDialerOptions(t *testing.T) {
	AddMockTransport()
	sock := GetMockSocket()
	defer MustClose(t, sock)

	d, e := sock.NewDialer(AddrMock(), nil)
	MustSucceed(t, e)
	MustBeTrue(t, d != nil)

	MustBeError(t, d.SetOption("bogus", nil), mangos.ErrBadOption)
	_, e = d.GetOption("bogus")
	MustBeError(t, e, mangos.ErrBadOption)

	val, e := d.GetOption(mangos.OptionReconnectTime)
	MustSucceed(t, e)
	MustBeTrue(t, reflect.TypeOf(val) == reflect.TypeOf(time.Duration(0)))

	val, e = d.GetOption(mangos.OptionMaxReconnectTime)
	MustSucceed(t, e)
	MustBeTrue(t, reflect.TypeOf(val) == reflect.TypeOf(time.Duration(0)))

	val, e = d.GetOption(mangos.OptionDialAsynch)
	MustSucceed(t, e)
	MustBeTrue(t, reflect.TypeOf(val) == reflect.TypeOf(true))

	val, e = d.GetOption("mockError")
	MustBeError(t, e, mangos.ErrProtoState)
	MustBeTrue(t, val == nil)

	MustBeError(t, d.SetOption(mangos.OptionDialAsynch, 1), mangos.ErrBadValue)
	MustBeError(t, d.SetOption(mangos.OptionReconnectTime, 1), mangos.ErrBadValue)
	MustBeError(t, d.SetOption(mangos.OptionReconnectTime, -time.Second), mangos.ErrBadValue)
	MustBeError(t, d.SetOption(mangos.OptionMaxReconnectTime, 1), mangos.ErrBadValue)
	MustBeError(t, d.SetOption(mangos.OptionMaxReconnectTime, -time.Second), mangos.ErrBadValue)
	MustBeError(t, d.SetOption(mangos.OptionMaxRecvSize, -100), mangos.ErrBadValue)
	MustBeError(t, d.SetOption(mangos.OptionMaxRecvSize, "a"), mangos.ErrBadValue)
	MustBeError(t, d.SetOption("mockError", mangos.ErrCanceled), mangos.ErrCanceled)

	MustSucceed(t, d.SetOption(mangos.OptionDialAsynch, false))
	MustSucceed(t, d.SetOption(mangos.OptionReconnectTime, time.Duration(0)))
	MustSucceed(t, d.SetOption(mangos.OptionReconnectTime, time.Second))
	MustSucceed(t, d.SetOption(mangos.OptionMaxReconnectTime, time.Duration(0)))
	MustSucceed(t, d.SetOption(mangos.OptionMaxReconnectTime, 5*time.Second))
	MustSucceed(t, d.SetOption(mangos.OptionMaxRecvSize, 1024))

	MustSucceed(t, d.SetOption(mangos.OptionMaxRecvSize, 1024))
	val, e = d.GetOption(mangos.OptionMaxRecvSize)
	MustSucceed(t, e)
	sz, ok := val.(int)
	MustBeTrue(t, ok)
	MustBeTrue(t, sz == 1024)
}

func TestDialerOptionsMap(t *testing.T) {
	AddMockTransport()
	sock := GetMockSocket()
	defer MustClose(t, sock)
	addr := AddrMock()

	opts := make(map[string]interface{})
	opts[mangos.OptionMaxRecvSize] = "garbage"
	d, e := sock.NewDialer(addr, opts)
	MustBeError(t, e, mangos.ErrBadValue)
	MustBeTrue(t, d == nil)

	opts = make(map[string]interface{})
	opts[mangos.OptionMaxRecvSize] = -1
	d, e = sock.NewDialer(addr, opts)
	MustBeError(t, e, mangos.ErrBadValue)
	MustBeTrue(t, d == nil)

	opts = make(map[string]interface{})
	opts[mangos.OptionMaxRecvSize] = 1001
	d, e = sock.NewDialer(addr, opts)
	MustBeError(t, e, mangos.ErrBadValue)
	MustBeTrue(t, d == nil)

	opts = make(map[string]interface{})
	opts[mangos.OptionMaxRecvSize] = 1002
	d, e = sock.NewDialer(addr, opts)
	MustBeError(t, e, mangos.ErrBadOption)
	MustBeTrue(t, d == nil)

	opts = make(map[string]interface{})
	opts[mangos.OptionDialAsynch] = -1
	d, e = sock.NewDialer(addr, opts)
	MustBeError(t, e, mangos.ErrBadValue)
	MustBeTrue(t, d == nil)

	opts = make(map[string]interface{})
	opts["JUNKOPT"] = "yes"
	d, e = sock.NewDialer(addr, opts)
	MustBeError(t, e, mangos.ErrBadOption)
	MustBeTrue(t, d == nil)

	opts = make(map[string]interface{})
	opts["mockError"] = mangos.ErrCanceled
	d, e = sock.NewDialer(addr, opts)
	MustBeError(t, e, mangos.ErrCanceled)
	MustBeTrue(t, d == nil)

	// Now good options
	opts = make(map[string]interface{})
	opts[mangos.OptionMaxRecvSize] = 3172
	opts[mangos.OptionReconnectTime] = time.Second
	opts[mangos.OptionMaxReconnectTime] = time.Second * 2
	opts[mangos.OptionDialAsynch] = false
	d, e = sock.NewDialer(addr, opts)
	MustSucceed(t, e)
	MustBeTrue(t, d != nil)
	v, e := d.GetOption(mangos.OptionMaxRecvSize)
	MustSucceed(t, e)
	sz, ok := v.(int)
	MustBeTrue(t, ok)
	MustBeTrue(t, sz == 3172)

}

func TestDialerOptionsInherit(t *testing.T) {
	AddMockTransport()
	sock := GetMockSocket()
	defer MustClose(t, sock)
	addr := AddrMock()

	// This should force listener not to alloc (bad option value)
	MustSucceed(t, sock.SetOption(mangos.OptionMaxRecvSize, 1001))
	d, e := sock.NewDialer(addr, nil)
	MustBeError(t, e, mangos.ErrBadValue)
	MustBeTrue(t, d == nil)
	MustSucceed(t, sock.SetOption(mangos.OptionMaxRecvSize, 1002))
	d, e = sock.NewDialer(addr, nil)
	MustSucceed(t, e)
	MustBeTrue(t, d != nil)

	MustSucceed(t, sock.SetOption(mangos.OptionMaxRecvSize, 500))
	v, e := d.GetOption(mangos.OptionMaxRecvSize)
	MustSucceed(t, e)
	MustBeTrue(t, v.(int) == 500)

}

func TestDialerPipe(t *testing.T) {
	sock1 := GetMockSocket()
	defer MustClose(t, sock1)
	sock2 := GetMockSocket()
	defer MustClose(t, sock2)
	addr := AddrTestInp()

	MustSucceed(t, sock1.SetOption(mangos.OptionRecvDeadline, time.Second))
	MustSucceed(t, sock2.SetOption(mangos.OptionSendDeadline, time.Second))

	d, e := sock1.NewDialer(addr, nil)
	MustSucceed(t, e)
	MustSucceed(t, sock2.Listen(addr))
	MustSucceed(t, d.Dial())

	MustSendString(t, sock2, "junk")
	m := MustRecvMsg(t, sock1)

	MustBeTrue(t, m.Pipe.Dialer() == d)
	MustBeTrue(t, m.Pipe.Listener() == nil)
	MustBeTrue(t, m.Pipe.Address() == addr)
	m.Free()
}

func TestDialerClosed(t *testing.T) {
	AddMockTransport()
	sock := GetMockSocket()
	defer MustClose(t, sock)

	d, e := sock.NewDialer(AddrMock(), nil)
	MustSucceed(t, e)
	MustBeTrue(t, d != nil)

	MustSucceed(t, d.Close())

	MustBeError(t, d.Dial(), mangos.ErrClosed)
	MustBeError(t, d.Close(), mangos.ErrClosed)
}

func TestDialerCloseAbort(t *testing.T) {
	addr := AddrTestInp()
	sock := GetMockSocket()
	defer MustClose(t, sock)

	d, e := sock.NewDialer(addr, nil)
	MustSucceed(t, e)
	MustBeTrue(t, d != nil)
	MustSucceed(t, d.SetOption(mangos.OptionDialAsynch, true))
	MustSucceed(t, d.SetOption(mangos.OptionReconnectTime, time.Millisecond))
	MustSucceed(t, d.SetOption(mangos.OptionMaxReconnectTime, time.Millisecond))

	MustSucceed(t, d.Dial())
	time.Sleep(time.Millisecond * 50)
	MustSucceed(t, d.Close())
}

func TestDialerCloseAbort2(t *testing.T) {
	sock := GetMockSocket()
	defer MustClose(t, sock)

	d, mc := GetMockDialer(t, sock)
	MustSucceed(t, d.SetOption(mangos.OptionDialAsynch, true))
	MustSucceed(t, d.SetOption(mangos.OptionReconnectTime, time.Millisecond))
	MustSucceed(t, d.SetOption(mangos.OptionMaxReconnectTime, time.Millisecond))

	var wg sync.WaitGroup
	wg.Add(1)

	pass := false
	go func() {
		defer wg.Done()
		time.Sleep(time.Millisecond * 20)
		MustSucceed(t, mc.Close())
		pass = true
	}()

	// We're async, so this is guaranteed to succeed.
	MustSucceed(t, d.Dial())
	time.Sleep(time.Millisecond * 50)
	wg.Wait()
	MustBeTrue(t, pass)
}

func TestDialerReuse(t *testing.T) {
	AddMockTransport()
	sock := GetMockSocket()
	defer MustClose(t, sock)

	d, e := sock.NewDialer(AddrMock(), nil)
	MustSucceed(t, e)
	MustBeTrue(t, d != nil)
	MustSucceed(t, d.SetOption(mangos.OptionDialAsynch, true))

	MustSucceed(t, d.Dial())
	MustBeError(t, d.Dial(), mangos.ErrAddrInUse)

	MustSucceed(t, d.Close())
}

func TestDialerReconnect(t *testing.T) {
	// We have to use real protocol and transport for this.
	addr := AddrTestInp()
	sock := GetSocket(t, pair.NewSocket)
	defer MustClose(t, sock)
	peer1 := GetSocket(t, pair.NewSocket)
	peer2 := GetSocket(t, pair.NewSocket)
	defer MustClose(t, peer2)

	MustSucceed(t, sock.SetOption(mangos.OptionRecvDeadline, time.Second))
	MustSucceed(t, sock.SetOption(mangos.OptionSendDeadline, time.Second))
	MustSucceed(t, peer1.SetOption(mangos.OptionRecvDeadline, time.Second))
	MustSucceed(t, peer1.SetOption(mangos.OptionSendDeadline, time.Second))
	MustSucceed(t, peer2.SetOption(mangos.OptionRecvDeadline, time.Second))
	MustSucceed(t, peer2.SetOption(mangos.OptionSendDeadline, time.Second))

	d, e := sock.NewDialer(addr, nil)
	MustSucceed(t, e)
	MustBeTrue(t, d != nil)
	MustSucceed(t, d.SetOption(mangos.OptionReconnectTime, time.Millisecond))
	MustSucceed(t, d.SetOption(mangos.OptionMaxReconnectTime, time.Millisecond))

	MustSucceed(t, peer1.Listen(addr))
	MustSucceed(t, d.Dial())
	time.Sleep(time.Millisecond * 20)
	MustClose(t, peer1)
	MustSucceed(t, peer2.Listen(addr))
	time.Sleep(time.Millisecond * 20)
	MustSendString(t, sock, "test")
	MustRecvString(t, peer2, "test")

	MustSucceed(t, d.Close())
}

func TestDialerConnectLate(t *testing.T) {
	// We have to use real protocol and transport for this.
	addr := AddrTestInp()
	sock := GetSocket(t, pair.NewSocket)
	defer MustClose(t, sock)
	peer := GetSocket(t, pair.NewSocket)
	defer MustClose(t, peer)

	d, e := sock.NewDialer(addr, nil)
	MustSucceed(t, e)
	MustBeTrue(t, d != nil)
	MustSucceed(t, d.SetOption(mangos.OptionReconnectTime, time.Millisecond))
	MustSucceed(t, d.SetOption(mangos.OptionMaxReconnectTime, time.Millisecond))
	MustSucceed(t, d.SetOption(mangos.OptionDialAsynch, true))

	lock := &sync.Mutex{}
	cond := sync.NewCond(lock)
	done := false

	hook := func(ev mangos.PipeEvent, p mangos.Pipe) {
		if ev == mangos.PipeEventAttached {
			lock.Lock()
			done = true
			cond.Broadcast()
			lock.Unlock()
		}
	}
	_ = sock.SetPipeEventHook(hook)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		time.Sleep(time.Millisecond * 50)
		MustSucceed(t, peer.Listen(addr))
	}()

	MustSucceed(t, d.Dial())

	wg.Wait()
	lock.Lock()
	cond.Wait()
	lock.Unlock()

	MustBeTrue(t, done)
}

func TestDialerConnectRefused(t *testing.T) {
	// We have to use real protocol and transport for this.
	addr := AddrTestInp()
	sock := GetSocket(t, pair.NewSocket)
	defer MustClose(t, sock)
	peer := GetSocket(t, pair.NewSocket)
	defer MustClose(t, peer)

	d, e := sock.NewDialer(addr, nil)
	MustSucceed(t, e)
	MustBeTrue(t, d != nil)
	MustSucceed(t, d.SetOption(mangos.OptionReconnectTime, time.Millisecond))
	MustSucceed(t, d.SetOption(mangos.OptionMaxReconnectTime, time.Millisecond))

	MustBeError(t, d.Dial(), mangos.ErrConnRefused)

}
