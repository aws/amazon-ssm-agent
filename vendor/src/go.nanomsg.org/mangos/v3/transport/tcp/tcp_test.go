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

package tcp

import (
	"net"
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3"
	. "go.nanomsg.org/mangos/v3/internal/test"
)

var tran = Transport

func TestTcpRecvMax(t *testing.T) {
	TranVerifyMaxRecvSize(t, tran, nil, nil)
}

func TestTcpOptions(t *testing.T) {
	TranVerifyInvalidOption(t, tran)
	TranVerifyIntOption(t, tran, mangos.OptionMaxRecvSize)
	TranVerifyNoDelayOption(t, tran)
	TranVerifyKeepAliveOption(t, tran)
}

func TestTcpScheme(t *testing.T) {
	TranVerifyScheme(t, tran)
}
func TestTcpAcceptWithoutListen(t *testing.T) {
	TranVerifyAcceptWithoutListen(t, tran)
}
func TestTcpListenAndAccept(t *testing.T) {
	TranVerifyListenAndAccept(t, tran, nil, nil)
}
func TestTcpDuplicateListen(t *testing.T) {
	TranVerifyDuplicateListen(t, tran, nil)
}
func TestTcpConnectionRefused(t *testing.T) {
	TranVerifyConnectionRefused(t, tran, nil)
}
func TestTcpHandshake(t *testing.T) {
	TranVerifyHandshakeFail(t, tran, nil, nil)
}
func TestTcpSendRecv(t *testing.T) {
	TranVerifySendRecv(t, tran, nil, nil)
}
func TestTcpAnonymousPort(t *testing.T) {
	TranVerifyAnonymousPort(t, "tcp://127.0.0.1:0", nil, nil)
}
func TestTcpInvalidDomain(t *testing.T) {
	TranVerifyBadAddress(t, "tcp://invalid.invalid", nil, nil)
}
func TestTcpInvalidLocalIP(t *testing.T) {
	TranVerifyBadLocalAddress(t, "tcp://1.1.1.1:80", nil)
}
func TestTcpBroadcastIP(t *testing.T) {
	TranVerifyBadAddress(t, "tcp://255.255.255.255:80", nil, nil)
}
func TestTcpListenerClosed(t *testing.T) {
	TranVerifyListenerClosed(t, tran, nil)
}

func TestTcpResolverChange(t *testing.T) {
	sock := GetMockSocket()
	defer MustClose(t, sock)

	addr := AddrTestTCP()
	MustSucceed(t, sock.Listen(addr))

	d, e := tran.NewDialer(addr, sock)
	MustSucceed(t, e)
	td := d.(*dialer)
	addr = td.addr
	td.addr = "tcp://invalid.invalid:80"
	p, e := d.Dial()
	MustFail(t, e)
	MustBeTrue(t, p == nil)

	td.addr = addr
	p, e = d.Dial()
	MustSucceed(t, e)
	MustSucceed(t, p.Close())
}

func TestTcpAcceptAbort(t *testing.T) {
	sock := GetMockSocket()
	defer MustClose(t, sock)

	addr := AddrTestTCP()
	l, e := tran.NewListener(addr, sock)
	MustSucceed(t, e)
	MustSucceed(t, l.Listen())
	_ = l.(*listener).l.Close()
	// This will make the accept loop spin hard, but nothing much
	// we can do about it.
	time.Sleep(time.Millisecond * 50)
}

func TestTcpMessageSize(t *testing.T) {
	TranVerifyMessageSizes(t, tran, nil, nil)
}
func TestTcpMessageHeader(t *testing.T) {
	TranVerifyMessageHeader(t, tran, nil, nil)
}
func TestTcpVerifyPipeAddresses(t *testing.T) {
	TranVerifyPipeAddresses(t, tran, nil, nil)
}
func TestTcpVerifyPipeOptions(t *testing.T) {
	TranVerifyPipeOptions2(t, tran, nil, nil)
}

type testAddr string

func (a testAddr) testDial() (net.Conn, error) {
	return net.Dial("tcp", string(a)[len("tcp://"):])
}

func TestTcpAbortHandshake(t *testing.T) {
	sock := GetMockSocket()
	defer MustClose(t, sock)
	addr := AddrTestTCP()
	l, e := sock.NewListener(addr, nil)
	MustSucceed(t, e)
	MustSucceed(t, l.Listen())
	c, e := testAddr(addr).testDial()
	MustSucceed(t, e)
	MustSucceed(t, c.Close())
}

func TestTcpBadHandshake(t *testing.T) {
	sock := GetMockSocket()
	defer MustClose(t, sock)
	addr := AddrTestTCP()
	l, e := sock.NewListener(addr, nil)
	MustSucceed(t, e)
	MustSucceed(t, l.Listen())
	TranSendConnBadHandshakes(t, testAddr(addr).testDial)
}

func TestTcpBadRecv(t *testing.T) {
	sock := GetMockSocket()
	defer MustClose(t, sock)
	addr := AddrTestTCP()
	l, e := sock.NewListener(addr, nil)
	MustSucceed(t, e)
	MustSucceed(t, l.Listen())
	TranSendBadMessages(t, sock.Info().Peer, false, testAddr(addr).testDial)
}

func TestTcpSendAbort(t *testing.T) {
	sock := GetMockSocket()
	defer MustClose(t, sock)
	addr := AddrTestTCP()
	l, e := sock.NewListener(addr, nil)
	MustSucceed(t, e)
	MustSucceed(t, l.Listen())
	c, e := testAddr(addr).testDial()
	MustSucceed(t, e)
	TranConnHandshake(t, c, sock.Info().Peer)
	MustSend(t, sock, make([]byte, 2*1024*1024)) // TCP window size is 64k
	time.Sleep(time.Millisecond * 100)
	MustSend(t, sock, make([]byte, 2*1024*1024)) // TCP window size is 64k
	MustSucceed(t, c.Close())
}
