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
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3"
	_ "go.nanomsg.org/mangos/v3/transport/inproc"
)

func TestListenerBadScheme(t *testing.T) {
	self := GetMockSocket()
	defer MustClose(t, self)

	l, e := self.NewListener("bad://nothere", nil)
	MustBeError(t, e, mangos.ErrBadTran)
	MustBeTrue(t, l == nil)

	// Malformed, needs :// bit
	l, e = self.NewListener("inproc:nothere", nil)
	MustBeError(t, e, mangos.ErrBadTran)
	MustBeTrue(t, l == nil)
}

func TestListenerAddress(t *testing.T) {
	AddMockTransport()
	self := GetMockSocket()
	defer MustClose(t, self)

	l, e := self.NewListener(AddrMock(), nil)
	MustSucceed(t, e)
	MustBeTrue(t, l.Address() == AddrMock())
}

func TestListenerSocketOptions(t *testing.T) {
	AddMockTransport()

	VerifyOptionInt(t, NewMockSocket, mangos.OptionMaxRecvSize)
}

func TestListenerOptions(t *testing.T) {
	AddMockTransport()
	sock := GetMockSocket()
	defer MustClose(t, sock)

	l, e := sock.NewListener(AddrMock(), nil)
	MustSucceed(t, e)
	MustBeTrue(t, l != nil)

	MustBeError(t, l.SetOption("bogus", nil), mangos.ErrBadOption)
	_, e = l.GetOption("bogus")
	MustBeError(t, e, mangos.ErrBadOption)

	val, e := l.GetOption(mangos.OptionMaxRecvSize)
	MustSucceed(t, e)
	MustBeTrue(t, reflect.TypeOf(val) == reflect.TypeOf(0))

	val, e = l.GetOption("mockError")
	MustBeError(t, e, mangos.ErrProtoState)
	MustBeTrue(t, val == nil)

	MustBeError(t, l.SetOption(mangos.OptionMaxRecvSize, "a"), mangos.ErrBadValue)
	MustBeError(t, l.SetOption(mangos.OptionMaxRecvSize, -100), mangos.ErrBadValue)
	MustBeError(t, l.SetOption("mockError", mangos.ErrCanceled), mangos.ErrCanceled)

	MustSucceed(t, l.SetOption(mangos.OptionMaxRecvSize, 1024))
	val, e = l.GetOption(mangos.OptionMaxRecvSize)
	MustSucceed(t, e)
	sz, ok := val.(int)
	MustBeTrue(t, ok)
	MustBeTrue(t, sz == 1024)
}

func TestListenerOptionsMap(t *testing.T) {
	AddMockTransport()
	sock := GetMockSocket()
	defer MustClose(t, sock)
	addr := AddrMock()

	opts := make(map[string]interface{})
	opts[mangos.OptionMaxRecvSize] = "garbage"
	l, e := sock.NewListener(addr, opts)
	MustBeError(t, e, mangos.ErrBadValue)
	MustBeTrue(t, l == nil)
	opts[mangos.OptionMaxRecvSize] = -1
	l, e = sock.NewListener(addr, opts)
	MustBeError(t, e, mangos.ErrBadValue)
	MustBeTrue(t, l == nil)

	opts = make(map[string]interface{})
	opts["JUNKOPT"] = "yes"
	l, e = sock.NewListener(addr, opts)
	MustBeError(t, e, mangos.ErrBadOption)
	MustBeTrue(t, l == nil)

	opts = make(map[string]interface{})
	opts["mockError"] = mangos.ErrCanceled
	l, e = sock.NewListener(addr, opts)
	MustBeError(t, e, mangos.ErrCanceled)
	MustBeTrue(t, l == nil)

	// Now a good option
	opts = make(map[string]interface{})
	opts[mangos.OptionMaxRecvSize] = 3172
	l, e = sock.NewListener(addr, opts)
	MustSucceed(t, e)
	MustBeTrue(t, l != nil)
	v, e := l.GetOption(mangos.OptionMaxRecvSize)
	MustSucceed(t, e)
	sz, ok := v.(int)
	MustBeTrue(t, ok)
	MustBeTrue(t, sz == 3172)

}

func TestListenerOptionsInherit(t *testing.T) {
	AddMockTransport()
	sock := GetMockSocket()
	defer MustClose(t, sock)
	addr := AddrMock()

	// This should force listener not to alloc (bad option value)
	MustSucceed(t, sock.SetOption(mangos.OptionMaxRecvSize, 1001))
	l, e := sock.NewListener(addr, nil)
	MustBeError(t, e, mangos.ErrBadValue)
	MustBeTrue(t, l == nil)
	MustSucceed(t, sock.SetOption(mangos.OptionMaxRecvSize, 1002))
	l, e = sock.NewListener(addr, nil)
	MustSucceed(t, e)
	MustBeTrue(t, l != nil)

	MustSucceed(t, sock.SetOption(mangos.OptionMaxRecvSize, 500))
	v, e := l.GetOption(mangos.OptionMaxRecvSize)
	MustSucceed(t, e)
	MustBeTrue(t, v.(int) == 500)
}

func TestListenerClosed(t *testing.T) {
	AddMockTransport()
	sock := GetMockSocket()
	defer MustClose(t, sock)

	l, e := sock.NewListener(AddrMock(), nil)
	MustSucceed(t, e)
	MustBeTrue(t, l != nil)

	MustSucceed(t, l.Close())

	MustBeError(t, l.Listen(), mangos.ErrClosed)
	MustBeError(t, l.Close(), mangos.ErrClosed)
}

func TestListenerCloseAbort(t *testing.T) {
	addr := AddrTestInp()
	sock := GetMockSocket()
	defer MustClose(t, sock)

	l, e := sock.NewListener(addr, nil)
	MustSucceed(t, e)
	MustBeTrue(t, l != nil)

	MustSucceed(t, l.Listen())
	time.Sleep(time.Millisecond * 50)
	MustSucceed(t, l.Close())
}

func TestListenerAcceptClose(t *testing.T) {
	AddMockTransport()
	sock := GetMockSocket()
	defer MustClose(t, sock)

	l, ml := GetMockListener(t, sock)

	MustSucceed(t, l.Listen())
	MustSucceed(t, ml.Close())
}

func TestListenerListenFail(t *testing.T) {
	AddMockTransport()
	sock := GetMockSocket()
	defer MustClose(t, sock)

	l, ml := GetMockListener(t, sock)

	ml.InjectError(mangos.ErrCanceled)
	MustBeError(t, l.Listen(), mangos.ErrCanceled)
}

func TestListenerPipe(t *testing.T) {
	sock1 := GetMockSocket()
	defer MustClose(t, sock1)
	sock2 := GetMockSocket()
	defer MustClose(t, sock2)
	addr := AddrTestInp()

	MustSucceed(t, sock1.SetOption(mangos.OptionRecvDeadline, time.Second))
	MustSucceed(t, sock2.SetOption(mangos.OptionSendDeadline, time.Second))

	l, e := sock1.NewListener(addr, nil)
	MustSucceed(t, e)
	MustSucceed(t, l.Listen())
	MustSucceed(t, sock2.Dial(addr))

	MustSendString(t, sock2, "junk")
	m := MustRecvMsg(t, sock1)

	MustBeTrue(t, m.Pipe.Dialer() == nil)
	MustBeTrue(t, m.Pipe.Listener() == l)
	MustBeTrue(t, m.Pipe.Address() == addr)
	m.Free()
}

func TestListenerAcceptOne(t *testing.T) {
	AddMockTransport()
	sock := GetMockSocket()
	defer MustClose(t, sock)

	l, ml := GetMockListener(t, sock)
	MustSucceed(t, l.Listen())

	mp := ml.NewPipe(sock.Info().Peer)
	MustSucceed(t, ml.AddPipe(mp))
	time.Sleep(time.Millisecond * 50)
	MustSucceed(t, ml.Close())
}

func TestListenerAcceptFail(t *testing.T) {
	AddMockTransport()
	sock := GetMockSocket()
	defer MustClose(t, sock)

	l, ml := GetMockListener(t, sock)

	MustSucceed(t, l.Listen())
	ml.InjectError(mangos.ErrCanceled)
	time.Sleep(time.Millisecond * 50)
	MustSucceed(t, ml.Close())
}

func TestListenerReuse(t *testing.T) {
	AddMockTransport()
	sock := GetMockSocket()
	defer MustClose(t, sock)

	l, e := sock.NewListener(AddrMock(), nil)
	MustSucceed(t, e)
	MustBeTrue(t, l != nil)

	MustSucceed(t, l.Listen())
	MustBeError(t, l.Listen(), mangos.ErrAddrInUse)

	MustSucceed(t, l.Close())
}
