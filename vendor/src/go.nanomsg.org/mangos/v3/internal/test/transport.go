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
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/transport"
)

type tranOption interface {
	SetOption(string, interface{}) error
	GetOption(string) (interface{}, error)
}

func getTranPair(t *testing.T, tran transport.Transport) (mangos.Dialer, mangos.Listener, mangos.Socket, mangos.Socket) {
	addr := getScratchAddr(tran)
	s1 := GetMockSocket()
	s2 := GetMockSocket()
	d, e := s1.NewDialer(addr, nil)
	MustSucceed(t, e)
	MustNotBeNil(t, d)
	l, e := s2.NewListener(addr, nil)
	MustSucceed(t, e)
	MustNotBeNil(t, l)
	return d, l, s1, s2
}

func getScratchAddr(tran transport.Transport) string {
	switch tran.Scheme() {
	case "mock":
		return "mock://mock"
	case "inproc":
		return AddrTestInp()
	case "tcp":
		return AddrTestTCP()
	case "ipc":
		return AddrTestIPC()
	case "tls+tcp":
		return AddrTestTLS()
	case "ws":
		return AddrTestWS()
	case "wss":
		return AddrTestWSS()
	}
	return "unknown://"
}

// TranVerifyBoolOption verifies that a bool option behaves properly.
func TranVerifyBoolOption(t *testing.T, tran transport.Transport, name string) {
	d, l, s1, s2 := getTranPair(t, tran)
	defer MustClose(t, s1)
	defer MustClose(t, s2)

	for _, o := range []tranOption{d, l} {
		MustBeError(t, o.SetOption(name, "nope"), mangos.ErrBadValue)
		MustSucceed(t, o.SetOption(name, true))
		MustSucceed(t, o.SetOption(name, false))
		v, e := o.GetOption(name)
		MustSucceed(t, e)
		_, ok := v.(bool)
		MustBeTrue(t, ok)
	}
}

// TranVerifyIntOption verifies that an int option behaves properly.
func TranVerifyIntOption(t *testing.T, tran transport.Transport, name string) {
	d, l, s1, s2 := getTranPair(t, tran)
	defer MustClose(t, s1)
	defer MustClose(t, s2)

	for _, o := range []tranOption{d, l} {

		MustBeError(t, o.SetOption(name, "nope"), mangos.ErrBadValue)
		MustBeError(t, o.SetOption(name, false), mangos.ErrBadValue)
		MustSucceed(t, o.SetOption(name, 2))
		MustSucceed(t, o.SetOption(name, 42))
		v, e := o.GetOption(name)
		MustSucceed(t, e)
		_, ok := v.(int)
		MustBeTrue(t, ok)
	}
}

// TranVerifyDurationOption verifies that a time.Duration option behaves properly.
func TranVerifyDurationOption(t *testing.T, tran transport.Transport, name string) {
	d, l, s1, s2 := getTranPair(t, tran)
	defer MustClose(t, s1)
	defer MustClose(t, s2)

	MustBeError(t, d.SetOption(name, "nope"), mangos.ErrBadValue)
	MustSucceed(t, d.SetOption(name, time.Second))
	v, e := d.GetOption(name)
	MustSucceed(t, e)
	_, ok := v.(time.Duration)
	MustBeTrue(t, ok)

	MustBeError(t, l.SetOption(name, false), mangos.ErrBadValue)
	MustSucceed(t, l.SetOption(name, time.Hour))
	v, e = l.GetOption(name)
	MustSucceed(t, e)
	_, ok = v.(time.Duration)
	MustBeTrue(t, ok)
}

// TranVerifyNoDelayOption verifies that NoDelay is always true.
func TranVerifyNoDelayOption(t *testing.T, tran transport.Transport) {
	d, l, s1, s2 := getTranPair(t, tran)
	defer MustClose(t, s1)
	defer MustClose(t, s2)

	name := mangos.OptionNoDelay
	for _, o := range []tranOption{d, l} {
		MustBeError(t, o.SetOption(name, "nope"), mangos.ErrBadValue)
		MustSucceed(t, o.SetOption(name, true))
		MustSucceed(t, o.SetOption(name, false)) // But it must not work
		v, e := o.GetOption(name)
		MustSucceed(t, e)
		b, ok := v.(bool)
		MustBeTrue(t, ok)
		MustBeTrue(t, b)
	}
}

// TranVerifyKeepAliveOption verifies that keep alive options work.
func TranVerifyKeepAliveOption(t *testing.T, tran transport.Transport) {
	d, l, s1, s2 := getTranPair(t, tran)
	defer MustClose(t, s1)
	defer MustClose(t, s2)

	// First verify that the base types work
	TranVerifyBoolOption(t, tran, mangos.OptionKeepAlive)
	TranVerifyDurationOption(t, tran, mangos.OptionKeepAliveTime)

	// Now try setting various things.
	for _, o := range []tranOption{d, l} {

		// Setting the legacy option to true
		MustSucceed(t, o.SetOption(mangos.OptionKeepAlive, true))
		b, e := o.GetOption(mangos.OptionKeepAlive)
		MustSucceed(t, e)
		MustBeTrue(t, b.(bool))
		x, e := o.GetOption(mangos.OptionKeepAliveTime)
		MustSucceed(t, e)
		MustBeTrue(t, x.(time.Duration) >= 0)

		// Setting the legacy option to false
		MustSucceed(t, o.SetOption(mangos.OptionKeepAlive, false))
		b, e = o.GetOption(mangos.OptionKeepAlive)
		MustSucceed(t, e)
		MustBeFalse(t, b.(bool))
		x, e = o.GetOption(mangos.OptionKeepAliveTime)
		MustSucceed(t, e)
		MustBeTrue(t, x.(time.Duration) < 0)

		// Setting the duration to zero (on)
		MustSucceed(t, o.SetOption(mangos.OptionKeepAliveTime, time.Duration(0)))
		b, e = o.GetOption(mangos.OptionKeepAlive)
		MustSucceed(t, e)
		MustBeTrue(t, b.(bool))

		MustSucceed(t, o.SetOption(mangos.OptionKeepAliveTime, time.Duration(-1)))
		b, e = o.GetOption(mangos.OptionKeepAlive)
		MustSucceed(t, e)
		MustBeFalse(t, b.(bool))

		MustSucceed(t, o.SetOption(mangos.OptionKeepAliveTime, time.Second))
		b, e = o.GetOption(mangos.OptionKeepAlive)
		MustSucceed(t, e)
		MustBeTrue(t, b.(bool))
	}

}

// TranVerifyTLSConfigOption verifies that OptionTLSConfig works properly.
func TranVerifyTLSConfigOption(t *testing.T, tran transport.Transport) {
	d, l, s1, s2 := getTranPair(t, tran)
	defer MustClose(t, s1)
	defer MustClose(t, s2)

	name := mangos.OptionTLSConfig

	MustBeError(t, d.SetOption(name, "nope"), mangos.ErrBadValue)
	MustSucceed(t, d.SetOption(name, GetTLSConfig(t, false)))
	v, e := d.GetOption(name)
	MustSucceed(t, e)
	_, ok := v.(*tls.Config)
	MustBeTrue(t, ok)

	MustBeError(t, l.SetOption(name, false), mangos.ErrBadValue)
	MustSucceed(t, l.SetOption(name, GetTLSConfig(t, true)))
	v, e = l.GetOption(name)
	MustSucceed(t, e)
	_, ok = v.(*tls.Config)
	MustBeTrue(t, ok)
}

// TranVerifyInvalidOption verifies that an invalid option behaves properly.
func TranVerifyInvalidOption(t *testing.T, tran transport.Transport) {
	d, l, s1, s2 := getTranPair(t, tran)
	defer MustClose(t, s1)
	defer MustClose(t, s2)

	// Dialer first.
	MustBeError(t, d.SetOption("NoSuchOption", 0), mangos.ErrBadOption)
	_, e := d.GetOption("NoSuchOption")
	MustBeError(t, e, mangos.ErrBadOption)

	MustBeError(t, l.SetOption("NoSuchOption", 0), mangos.ErrBadOption)
	_, e = l.GetOption("NoSuchOption")
	MustBeError(t, e, mangos.ErrBadOption)
}

// TranVerifyScheme verifies that we get the right scheme.  It also tries
// an invalid scheme.
func TranVerifyScheme(t *testing.T, tran transport.Transport) {
	sock := GetMockSocket()
	defer MustClose(t, sock)
	d, e := tran.NewDialer("wrong://", sock)
	MustBeError(t, e, mangos.ErrBadTran)
	MustBeTrue(t, d == nil)
	l, e := tran.NewListener("wrong://", sock)
	MustBeError(t, e, mangos.ErrBadTran)
	MustBeTrue(t, l == nil)

	addr := getScratchAddr(tran)
	d, e = tran.NewDialer(addr, sock)
	MustSucceed(t, e)
	MustNotBeNil(t, d)

	l, e = tran.NewListener(addr, sock)
	MustSucceed(t, e)
	MustNotBeNil(t, l)
	addr2 := l.Address()
	MustBeTrue(t, strings.HasPrefix(addr2, tran.Scheme()+"://"))
}

// TranVerifyConnectionRefused verifies that connection is refused if no listener.
func TranVerifyConnectionRefused(t *testing.T, tran transport.Transport, opts map[string]interface{}) {
	sock := GetMockSocket()
	defer MustClose(t, sock)
	d, _ := sock.NewDialer(getScratchAddr(tran), opts)
	MustFail(t, d.Dial()) // Windows won't let us validate properly
}

// TranVerifyDuplicateListen verifies that we can't bind to the same address twice.
func TranVerifyDuplicateListen(t *testing.T, tran transport.Transport, opts map[string]interface{}) {
	sock1 := GetMockSocket()
	defer MustClose(t, sock1)
	sock2 := GetMockSocket()
	defer MustClose(t, sock2)
	addr := getScratchAddr(tran)
	l1, _ := sock1.NewListener(addr, opts)
	l2, _ := sock2.NewListener(addr, opts)
	MustSucceed(t, l1.Listen())
	MustFail(t, l2.Listen()) // Cannot validate ErrAddrInUse because Windows
}

// TranVerifyListenAndAccept verifies that we can establish the connection.
func TranVerifyListenAndAccept(t *testing.T, tran transport.Transport, dOpts, lOpts map[string]interface{}) {
	s1 := GetMockSocket()
	s2 := GetMockSocket()
	defer MustClose(t, s1)
	defer MustClose(t, s2)
	addr := getScratchAddr(tran)
	d, e := s1.NewDialer(addr, dOpts)
	MustSucceed(t, e)
	l, e := s2.NewListener(addr, lOpts)
	MustSucceed(t, e)
	MustSucceed(t, l.Listen())

	var wg sync.WaitGroup
	wg.Add(1)
	pass := false
	go func() {
		defer wg.Done()
		MustSucceed(t, d.Dial())
		pass = true
	}()

	wg.Wait()
	MustBeTrue(t, pass)
}

// TranVerifyAcceptWithoutListen verifies that we can't call accept if we
// did not first call listen.
func TranVerifyAcceptWithoutListen(t *testing.T, tran transport.Transport) {
	sock := GetMockSocket()
	defer MustClose(t, sock)
	l, e := tran.NewListener(getScratchAddr(tran), sock)
	MustSucceed(t, e)
	_, e = l.Accept()
	MustBeError(t, e, mangos.ErrClosed)
}

// TranVerifyMaxRecvSize verifies the transport handles maximum receive size properly.
func TranVerifyMaxRecvSize(t *testing.T, tran transport.Transport, dOpts, lOpts map[string]interface{}) {
	VerifyOptionMaxRecvSize(t, NewMockSocket)

	addr := getScratchAddr(tran)
	tx := GetMockSocket()
	rx := GetMockSocket()
	defer MustClose(t, tx)
	defer MustClose(t, rx)
	maxRx := 100

	// Now try setting the option
	MustSucceed(t, rx.SetOption(mangos.OptionMaxRecvSize, maxRx))
	// At this point, we can issue requests on rq, and read them from rp.
	MustSucceed(t, rx.SetOption(mangos.OptionRecvDeadline, time.Millisecond*50))
	MustSucceed(t, tx.SetOption(mangos.OptionSendDeadline, time.Second))

	ConnectPairVia(t, addr, rx, tx, lOpts, dOpts)

	for i := maxRx - 2; i < maxRx+2; i++ {
		m := mangos.NewMessage(i)
		m.Body = append(m.Body, make([]byte, i)...)
		MustSendMsg(t, tx, m)
		if i <= maxRx {
			m = MustRecvMsg(t, rx)
			m.Free()
		} else {
			MustNotRecv(t, rx, mangos.ErrRecvTimeout)
		}
	}
}

// TranVerifyHandshakeFail verifies that we fail if the protocols mismatch.
func TranVerifyHandshakeFail(t *testing.T, tran transport.Transport, dOpts, lOpts map[string]interface{}) {
	s1 := GetMockSocketEx(1, "mock1")
	s2 := GetMockSocketEx(2, "mock2")
	defer MustClose(t, s1)
	defer MustClose(t, s2)
	addr := getScratchAddr(tran)
	d, e := s1.NewDialer(addr, dOpts)
	MustSucceed(t, e)
	l, e := s2.NewListener(addr, lOpts)
	MustSucceed(t, e)
	MustSucceed(t, l.Listen())

	var wg sync.WaitGroup
	wg.Add(1)
	pass := false
	go func() {
		defer wg.Done()
		e = d.Dial()
		MustBeError(t, e, mangos.ErrBadProto)
		pass = true
	}()

	wg.Wait()
	MustBeTrue(t, pass)
}

// TranVerifySendRecv just verifies basic send and receive.
func TranVerifySendRecv(t *testing.T, tran transport.Transport, dOpts, lOpts map[string]interface{}) {
	tx := GetMockSocket()
	rx := GetMockSocket()
	defer MustClose(t, tx)
	defer MustClose(t, rx)

	MustSucceed(t, rx.SetOption(mangos.OptionRecvDeadline, time.Second))
	MustSucceed(t, tx.SetOption(mangos.OptionRecvDeadline, time.Second))
	MustSucceed(t, rx.SetOption(mangos.OptionSendDeadline, time.Second))
	MustSucceed(t, tx.SetOption(mangos.OptionSendDeadline, time.Second))

	addr := getScratchAddr(tran)
	d, e := tx.NewDialer(addr, dOpts)
	MustSucceed(t, e)
	l, e := rx.NewListener(addr, lOpts)
	MustSucceed(t, e)

	MustSucceed(t, l.Listen())
	MustSucceed(t, d.Dial())

	for i := 0; i < 10; i++ {
		send := fmt.Sprintf("SEND%d", i)
		repl := fmt.Sprintf("REPL%d", i)
		MustSendString(t, tx, send)
		MustRecvString(t, rx, send)
		MustSendString(t, rx, repl)
		MustRecvString(t, tx, repl)
	}
}

// TranVerifyAnonymousPort is used by TCP based transports to verify that using
// a wild card port address works. The addr is an address using a wild card
// port (usually port 0).
func TranVerifyAnonymousPort(t *testing.T, addr string, dOpts, lOpts map[string]interface{}) {
	tx := GetMockSocket()
	rx := GetMockSocket()
	defer MustClose(t, tx)
	defer MustClose(t, rx)

	MustSucceed(t, rx.SetOption(mangos.OptionRecvDeadline, time.Second))
	MustSucceed(t, tx.SetOption(mangos.OptionRecvDeadline, time.Second))
	MustSucceed(t, rx.SetOption(mangos.OptionSendDeadline, time.Second))
	MustSucceed(t, tx.SetOption(mangos.OptionSendDeadline, time.Second))

	// First get the listener.
	l, e := rx.NewListener(addr, lOpts)
	MustSucceed(t, e)
	MustBeTrue(t, l.Address() == addr)
	MustSucceed(t, l.Listen())
	MustBeTrue(t, l.Address() != addr)

	d, e := tx.NewDialer(l.Address(), dOpts)
	MustSucceed(t, e)
	MustSucceed(t, d.Dial())

	MustSendString(t, tx, "hello")
	MustRecvString(t, rx, "hello")

	// Impossible to dial to a wildcard address
	d2, e := tx.NewDialer(addr, dOpts)
	if e == nil {
		MustFail(t, d2.Dial())
	}
}

// TranVerifyPipeOptions verifies that the LocalAddr, RemoteAddr and invalid
// options all behave as we expect.
func TranVerifyPipeOptions(t *testing.T, tran transport.Transport, dOpts, lOpts map[string]interface{}) {
	tx := GetMockSocket()
	rx := GetMockSocket()
	defer MustClose(t, tx)
	defer MustClose(t, rx)

	addr := getScratchAddr(tran)
	MustSucceed(t, rx.SetOption(mangos.OptionRecvDeadline, time.Second))
	MustSucceed(t, tx.SetOption(mangos.OptionRecvDeadline, time.Second))
	MustSucceed(t, rx.SetOption(mangos.OptionSendDeadline, time.Second))
	MustSucceed(t, tx.SetOption(mangos.OptionSendDeadline, time.Second))

	MustSucceed(t, rx.ListenOptions(addr, lOpts))
	MustSucceed(t, tx.DialOptions(addr, dOpts))

	MustSendString(t, tx, "hello")
	m := MustRecvMsg(t, rx)
	p1 := m.Pipe

	MustSendString(t, rx, "there")
	m = MustRecvMsg(t, tx)
	p2 := m.Pipe

	remaddr := []net.Addr{}
	locaddr := []net.Addr{}

	for _, p := range []mangos.Pipe{p1, p2} {
		a, e := p.GetOption(mangos.OptionLocalAddr)
		addr1 := a.(net.Addr)
		MustSucceed(t, e)
		MustBeTrue(t, len(addr1.Network()) > 0)
		MustBeTrue(t, len(addr1.String()) > 0)

		a, e = p.GetOption(mangos.OptionRemoteAddr)
		addr2 := a.(net.Addr)
		MustSucceed(t, e)
		MustBeTrue(t, len(addr2.Network()) > 0)
		MustBeTrue(t, len(addr2.String()) > 0)

		MustBeTrue(t, addr2.Network() == addr1.Network())

		locaddr = append(locaddr, addr1)
		remaddr = append(remaddr, addr2)

		_, e = p.GetOption("NO-SUCH-OPTION")
		MustFail(t, e)
	}
	MustBeTrue(t, remaddr[0].String() == locaddr[1].String())
	MustBeTrue(t, remaddr[1].String() == locaddr[0].String())

}

// TranVerifyBadLocalAddress is used to verify that a given address cannot be
// listened to.  This could be for an address that we cannot resolve a name
// for, or an address that we do not have an IP address for.  The failure can
// occur at either listener allocation time, or when trying to bind.
func TranVerifyBadLocalAddress(t *testing.T, addr string, opts map[string]interface{}) {
	sock := GetMockSocket()
	defer MustClose(t, sock)

	if l, e := sock.NewListener(addr, opts); e == nil {
		MustFail(t, l.Listen())
	}
}

// TranVerifyBadRemoteAddress is used to verify that a given address cannot be
// dialed to.  This could be for an address that we cannot resolve a name
// for, or an address is known to be otherwise impossible or invalid.
func TranVerifyBadRemoteAddress(t *testing.T, addr string, opts map[string]interface{}) {
	sock := GetMockSocket()
	defer MustClose(t, sock)

	if d, e := sock.NewDialer(addr, opts); e == nil {
		MustFail(t, d.Dial())
	}
}

// TranVerifyBadAddress is used to verify that certain addresses are invalid
// and cannot be used for dialing or listening.  This is useful, for example,
// when checking that DNS failures are handled properly.
func TranVerifyBadAddress(t *testing.T, addr string, dOpts, lOpts map[string]interface{}) {
	TranVerifyBadLocalAddress(t, addr, lOpts)
	TranVerifyBadRemoteAddress(t, addr, dOpts)
}

// TranVerifyListenerClosed verifies that the listener behaves after closed.
func TranVerifyListenerClosed(t *testing.T, tran transport.Transport, opts map[string]interface{}) {
	sock := GetMockSocket()
	defer MustClose(t, sock)

	l, e := tran.NewListener(getScratchAddr(tran), sock)
	MustSucceed(t, e)
	for key, val := range opts {
		MustSucceed(t, l.SetOption(key, val))
	}
	MustSucceed(t, l.Close())
	MustBeError(t, l.Listen(), mangos.ErrClosed)
	_, e = l.Accept()
	MustBeError(t, e, mangos.ErrClosed)
	_ = l.Close() // this might succeed or fail, we don't care.

	l, e = tran.NewListener(getScratchAddr(tran), sock)
	MustSucceed(t, e)
	for key, val := range opts {
		MustSucceed(t, l.SetOption(key, val))
	}
	MustSucceed(t, l.Listen())
	MustSucceed(t, l.Close())
	_, e = l.Accept()
	MustBeError(t, e, mangos.ErrClosed)

	// Now async
	l, e = tran.NewListener(getScratchAddr(tran), sock)
	MustSucceed(t, e)
	for key, val := range opts {
		MustSucceed(t, l.SetOption(key, val))
	}
	MustSucceed(t, l.Listen())
	time.AfterFunc(time.Millisecond*50, func() {
		MustSucceed(t, l.Close())
	})
	_, e = l.Accept()
	MustBeError(t, e, mangos.ErrClosed)
}

// TranVerifyDialNoCert verifies that we fail to dial if we lack a Server cert.
func TranVerifyDialNoCert(t *testing.T, tran transport.Transport) {
	sock := GetMockSocket()
	defer MustClose(t, sock)

	addr := getScratchAddr(tran)
	opts := make(map[string]interface{})
	opts[mangos.OptionTLSConfig] = GetTLSConfig(t, true)
	MustSucceed(t, sock.ListenOptions(addr, opts))

	// Unfortunately the tls package doesn't allow us to distinguish
	// the various errors.
	MustFail(t, sock.Dial(addr))
}

// TranVerifyDialInsecure verifies InsecureSkipVerify.
func TranVerifyDialInsecure(t *testing.T, tran transport.Transport) {
	sock := GetMockSocket()
	defer MustClose(t, sock)

	addr := getScratchAddr(tran)
	opts := make(map[string]interface{})
	opts[mangos.OptionTLSConfig] = GetTLSConfig(t, true)
	MustSucceed(t, sock.ListenOptions(addr, opts))

	opts = make(map[string]interface{})
	opts[mangos.OptionTLSConfig] = &tls.Config{}
	MustFail(t, sock.DialOptions(addr, opts))

	opts[mangos.OptionTLSConfig] = &tls.Config{
		InsecureSkipVerify: true,
	}
	MustSucceed(t, sock.DialOptions(addr, opts))
}

// TranVerifyMessageSizes verifies we can transport a variety of message sizes
func TranVerifyMessageSizes(t *testing.T, tran transport.Transport, dOpts, lOpts map[string]interface{}) {
	sock1 := GetMockSocket()
	sock2 := GetMockSocket()
	addr := getScratchAddr(tran)
	defer MustClose(t, sock1)
	defer MustClose(t, sock2)

	MustSucceed(t, sock1.SetOption(mangos.OptionRecvDeadline, time.Second))
	MustSucceed(t, sock1.SetOption(mangos.OptionSendDeadline, time.Second))
	MustSucceed(t, sock2.SetOption(mangos.OptionRecvDeadline, time.Second))
	MustSucceed(t, sock2.SetOption(mangos.OptionSendDeadline, time.Second))
	MustSucceed(t, sock1.ListenOptions(addr, lOpts))
	MustSucceed(t, sock2.DialOptions(addr, dOpts))

	for i := 0; i < 20; i++ {
		sz := 1 << i
		m := mangos.NewMessage(sz)
		for j := 0; j < sz; j++ {
			m.Body = append(m.Body, byte(i))
		}
		MustSendMsg(t, sock1, m)
		m = MustRecvMsg(t, sock2)
		MustBeTrue(t, len(m.Body) == sz)
		for j := 0; j < sz; j++ {
			MustBeTrue(t, m.Body[j] == byte(i))
		}
		m.Free()
	}
	// And back down:
	for i := 20; i >= 0; i-- {
		sz := 1 << i
		m := mangos.NewMessage(sz)
		for j := 0; j < sz; j++ {
			m.Body = append(m.Body, byte(i))
		}
		MustSendMsg(t, sock1, m)
		m = MustRecvMsg(t, sock2)
		MustBeTrue(t, len(m.Body) == sz)
		for j := 0; j < sz; j++ {
			MustBeTrue(t, m.Body[j] == byte(i))
		}
		m.Free()
	}
}

// TranVerifyMessageHeader verifies that message headers are transmitted.
func TranVerifyMessageHeader(t *testing.T, tran transport.Transport, dOpts, lOpts map[string]interface{}) {
	sock1 := GetMockSocket()
	sock2 := GetMockSocket()
	addr := getScratchAddr(tran)
	defer MustClose(t, sock1)
	defer MustClose(t, sock2)

	MustSucceed(t, sock1.SetOption(mangos.OptionRecvDeadline, time.Second))
	MustSucceed(t, sock1.SetOption(mangos.OptionSendDeadline, time.Second))
	MustSucceed(t, sock2.SetOption(mangos.OptionRecvDeadline, time.Second))
	MustSucceed(t, sock2.SetOption(mangos.OptionSendDeadline, time.Second))
	MustSucceed(t, sock1.ListenOptions(addr, lOpts))
	MustSucceed(t, sock2.DialOptions(addr, dOpts))

	m := mangos.NewMessage(0)
	m.Header = append(m.Header, 'h', 'e', 'l', 'l', 'o', ',', ' ')
	m.Body = append(m.Body, 'w', 'o', 'r', 'l', 'd')
	MustSendMsg(t, sock1, m)
	MustRecvString(t, sock2, "hello, world")
}

// TranVerifyPipeAddresses performs basic validation of pipe address options.
func TranVerifyPipeAddresses(t *testing.T, tran transport.Transport, dOpts, lOpts map[string]interface{}) {
	sock1 := GetMockSocket()
	sock2 := GetMockSocket()
	addr := getScratchAddr(tran)
	defer MustClose(t, sock1)
	defer MustClose(t, sock2)

	MustSucceed(t, sock1.SetOption(mangos.OptionRecvDeadline, time.Second))
	MustSucceed(t, sock1.SetOption(mangos.OptionSendDeadline, time.Second))
	MustSucceed(t, sock2.SetOption(mangos.OptionRecvDeadline, time.Second))
	MustSucceed(t, sock2.SetOption(mangos.OptionSendDeadline, time.Second))
	MustSucceed(t, sock1.ListenOptions(addr, lOpts))
	MustSucceed(t, sock2.DialOptions(addr, dOpts))

	m := mangos.NewMessage(0)
	MustSendMsg(t, sock1, m)
	m = MustRecvMsg(t, sock2)
	p1 := m.Pipe

	// Now Send it back the other way.
	MustSendMsg(t, sock2, m)
	m = MustRecvMsg(t, sock1)
	p2 := m.Pipe

	v, e := m.Pipe.GetOption("JUNK")
	MustBeError(t, e, mangos.ErrBadProperty)
	MustBeTrue(t, v == nil)

	v, e = p1.GetOption(mangos.OptionLocalAddr)
	MustSucceed(t, e)
	la1, ok := v.(net.Addr)
	MustBeTrue(t, ok)
	v, e = p1.GetOption(mangos.OptionRemoteAddr)
	MustSucceed(t, e)
	ra1, ok := v.(net.Addr)
	MustBeTrue(t, ok)

	v, e = p2.GetOption(mangos.OptionLocalAddr)
	MustSucceed(t, e)
	la2, ok := v.(net.Addr)
	MustBeTrue(t, ok)
	v, e = p2.GetOption(mangos.OptionRemoteAddr)
	MustSucceed(t, e)
	ra2, ok := v.(net.Addr)
	MustBeTrue(t, ok)

	MustBeTrue(t, la1.Network() == la2.Network())
	MustBeTrue(t, ra1.Network() == ra2.Network())
	MustBeTrue(t, ra2.Network() == ra1.Network())

	MustBeTrue(t, la1.String() == ra2.String())
	MustBeTrue(t, la2.String() == ra1.String())

}

// TranVerifyPipeOptions2 verifies standard pipe Options.
func TranVerifyPipeOptions2(t *testing.T, tran transport.Transport, dOpts, lOpts map[string]interface{}) {
	sock1 := GetMockSocket()
	sock2 := GetMockSocket()
	addr := getScratchAddr(tran)
	defer MustClose(t, sock1)
	defer MustClose(t, sock2)

	MustSucceed(t, sock1.SetOption(mangos.OptionRecvDeadline, time.Second))
	MustSucceed(t, sock1.SetOption(mangos.OptionSendDeadline, time.Second))
	MustSucceed(t, sock2.SetOption(mangos.OptionRecvDeadline, time.Second))
	MustSucceed(t, sock2.SetOption(mangos.OptionSendDeadline, time.Second))
	MustSucceed(t, sock1.ListenOptions(addr, lOpts))
	MustSucceed(t, sock2.DialOptions(addr, dOpts))

	m := mangos.NewMessage(0)
	MustSendMsg(t, sock1, m)
	m = MustRecvMsg(t, sock2)
	v, e := m.Pipe.GetOption("JUNK")
	MustBeError(t, e, mangos.ErrBadProperty)
	MustBeTrue(t, v == nil)

	v, e = m.Pipe.GetOption(mangos.OptionMaxRecvSize)
	MustSucceed(t, e)
	_, ok := v.(int)
	MustBeTrue(t, ok)
}

// type connHeader struct {
// 	Zero     byte // must be zero
// 	S        byte // 'S'
// 	P        byte // 'P'
// 	Version  byte // only zero at present
// 	Proto    uint16
// 	Reserved uint16 // always zero at present
// }

// TranSendConnBadHandshakes just sends garbage handshakes.
func TranSendConnBadHandshakes(t *testing.T, dial func() (net.Conn, error)) {

	tries := [][]byte{
		{},
		{0},
		{0, 'S'},
		{0, 'S', 'P'},
		{0, 'S', 'P', 0},
		{0, 'S', 'P', 0, 0},
		{0, 'S', 'P', 0, 0, 0},
		{0, 'S', 'P', 0, 0, 1, 0},
		{0, 'S', 'P', 0, 0, 1, 0, 0},
		{1, 'S', 'P', 0, 0, 0, 0, 0},
		{0, 'x', 'P', 0, 0, 0, 0, 0},
		{0, 'S', 'x', 0, 0, 0, 0, 0},
		{0, 'S', 'P', 2, 0, 0, 0, 0},
		{0, 'S', 'P', 0, 0, 0, 0, 1},
	}

	for _, b := range tries {
		c, e := dial()
		MustSucceed(t, e)
		_, e = c.Write(b)
		MustSucceed(t, e)
		time.Sleep(time.Millisecond * 10)
		MustSucceed(t, c.Close())
	}
}

// TranConnHandshake just performs the handshake, the conn should be connected.
func TranConnHandshake(t *testing.T, c net.Conn, proto uint16) {
	hs := []byte{
		0,                 // Zero
		'S',               // S
		'P',               // P
		0,                 // Version
		byte(proto / 256), // Proto Hi byte
		byte(proto % 256), // Proto Lo byte
		0,                 // Reserved Hi
		0,                 // Reserved Lo
	}
	peer := make([]byte, 8)
	_, e := c.Write(hs)
	MustSucceed(t, e)
	_, e = io.ReadFull(c, peer)
	MustSucceed(t, e)
}

// TranSendBadMessages opens new connections and sends garbage to them.
func TranSendBadMessages(t *testing.T, proto uint16, isipc bool, dial func() (net.Conn, error)) {

	bad := [][]byte{
		{},
		{0x80},
		{0, 0, 0, 1},
		{0, 0, 0, 0, 0, 0, 0, 10, 1, 2},
		{0x80, 0, 0, 0, 1, 1, 2, 3, 4, 5, 6},
	}
	for _, b := range bad {
		c, e := dial()
		MustSucceed(t, e)
		TranConnHandshake(t, c, proto)
		var x []byte
		if isipc {
			x = append([]byte{1}, b...)
		} else {
			x = b
		}
		_, e = c.Write(x)
		MustSucceed(t, e)
		time.Sleep(time.Millisecond * 10)
		MustSucceed(t, c.Close())
	}
}
