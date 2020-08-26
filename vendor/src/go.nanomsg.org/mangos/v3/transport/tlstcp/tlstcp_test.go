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

package tlstcp

import (
	"crypto/tls"
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/internal/test"
)

var tran = Transport
var lOpts map[string]interface{}
var dOpts map[string]interface{}

func init() {
	dOpts = make(map[string]interface{})
	lOpts = make(map[string]interface{})
	var t *testing.T
	dOpts[mangos.OptionTLSConfig] = test.GetTLSConfig(t, false)
	lOpts[mangos.OptionTLSConfig] = test.GetTLSConfig(t, true)
}

func TestTlsTcpRecvMax(t *testing.T) {
	test.TranVerifyMaxRecvSize(t, tran, dOpts, lOpts)
}

func TestTlsTcpOptions(t *testing.T) {
	test.TranVerifyInvalidOption(t, tran)
	test.TranVerifyBoolOption(t, tran, mangos.OptionKeepAlive)
	test.TranVerifyIntOption(t, tran, mangos.OptionMaxRecvSize)
	test.TranVerifyDurationOption(t, tran, mangos.OptionKeepAliveTime)
	test.TranVerifyNoDelayOption(t, tran)
	test.TranVerifyKeepAliveOption(t, tran)
	test.TranVerifyTLSConfigOption(t, tran)
}

func TestTlsTcpScheme(t *testing.T) {
	test.TranVerifyScheme(t, tran)
}
func TestTlsTcpAcceptWithoutListen(t *testing.T) {
	test.TranVerifyAcceptWithoutListen(t, tran)
}
func TestTcpListenAndAccept(t *testing.T) {
	test.TranVerifyListenAndAccept(t, tran, dOpts, lOpts)
}
func TestTlsTcpDuplicateListen(t *testing.T) {
	test.TranVerifyDuplicateListen(t, tran, lOpts)
}
func TestTlsTcpConnectionRefused(t *testing.T) {
	test.TranVerifyConnectionRefused(t, tran, dOpts)
}
func TestTlsTcpHandshake(t *testing.T) {
	test.TranVerifyHandshakeFail(t, tran, dOpts, lOpts)
}
func TestTlsTcpSendRecv(t *testing.T) {
	test.TranVerifySendRecv(t, tran, dOpts, lOpts)
}
func TestTlsTcpAnonymousPort(t *testing.T) {
	test.TranVerifyAnonymousPort(t, "tls+tcp://127.0.0.1:0", dOpts, lOpts)
}
func TestTlsTcpInvalidDomain(t *testing.T) {
	test.TranVerifyBadAddress(t, "tls+tcp://invalid.invalid", dOpts, lOpts)
}
func TestTlsTcpInvalidLocalIP(t *testing.T) {
	test.TranVerifyBadLocalAddress(t, "tls+tcp://1.1.1.1:80", lOpts)
}
func TestTlsTcpBroadcastIP(t *testing.T) {
	test.TranVerifyBadAddress(t, "tls+tcp://255.255.255.255:80", dOpts, lOpts)
}

func TestTlsTcpListenerClosed(t *testing.T) {
	test.TranVerifyListenerClosed(t, tran, lOpts)
}

func TestTlsTcpResolverChange(t *testing.T) {
	sock := test.GetMockSocket()
	defer test.MustClose(t, sock)

	addr := test.AddrTestTLS()
	test.MustSucceed(t, sock.ListenOptions(addr, lOpts))

	d, e := tran.NewDialer(addr, sock)
	test.MustSucceed(t, e)
	for key, val := range dOpts {
		test.MustSucceed(t, d.SetOption(key, val))
	}

	td := d.(*dialer)
	addr = td.addr
	td.addr = "tls+tcp://invalid.invalid:80"
	p, e := d.Dial()
	test.MustFail(t, e)
	test.MustBeTrue(t, p == nil)

	td.addr = addr
	p, e = d.Dial()
	test.MustSucceed(t, e)
	test.MustSucceed(t, p.Close())
}

func TestTlsTcpAcceptAbort(t *testing.T) {
	sock := test.GetMockSocket()
	defer test.MustClose(t, sock)

	addr := test.AddrTestTLS()
	l, e := tran.NewListener(addr, sock)
	test.MustSucceed(t, e)

	for key, val := range lOpts {
		test.MustSucceed(t, l.SetOption(key, val))
	}
	test.MustSucceed(t, l.Listen())
	_ = l.(*listener).l.Close()
	// This will make the accept loop spin hard, but nothing much
	// we can do about it.
	time.Sleep(time.Millisecond * 50)
}

func TestTlsListenNoCert(t *testing.T) {
	sock := test.GetMockSocket()
	defer test.MustClose(t, sock)

	addr := test.AddrTestTLS()
	test.MustBeError(t, sock.ListenOptions(addr, nil), mangos.ErrTLSNoConfig)

	cfg := &tls.Config{}
	opts := make(map[string]interface{})
	opts[mangos.OptionTLSConfig] = cfg
	test.MustBeError(t, sock.ListenOptions(addr, opts), mangos.ErrTLSNoCert)
}

func TestTlsDialNoCert(t *testing.T) {
	test.TranVerifyDialNoCert(t, tran)
}

func TestTlsDialInsecure(t *testing.T) {
	test.TranVerifyDialInsecure(t, tran)
}

func TestTlsMessageSize(t *testing.T) {
	test.TranVerifyMessageSizes(t, tran, dOpts, lOpts)
}

func TestTlsMessageHeader(t *testing.T) {
	test.TranVerifyMessageHeader(t, tran, dOpts, lOpts)
}
