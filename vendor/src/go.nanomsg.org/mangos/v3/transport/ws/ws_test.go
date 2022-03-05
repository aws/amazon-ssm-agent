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

package ws

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"go.nanomsg.org/mangos/v3"
	. "go.nanomsg.org/mangos/v3/internal/test"
)

var tran = Transport

func TestWsOptions(t *testing.T) {
	TranVerifyInvalidOption(t, tran)
	TranVerifyIntOption(t, tran, mangos.OptionMaxRecvSize)
	TranVerifyNoDelayOption(t, tran)
	TranVerifyBoolOption(t, tran, OptionWebSocketCheckOrigin)
}

func TestWsScheme(t *testing.T) {
	TranVerifyScheme(t, tran)
}
func TestWsRecvMax(t *testing.T) {
	TranVerifyMaxRecvSize(t, tran, nil, nil)
}
func TestWsAcceptWithoutListen(t *testing.T) {
	TranVerifyAcceptWithoutListen(t, tran)
}
func TestWsListenAndAccept(t *testing.T) {
	TranVerifyListenAndAccept(t, tran, nil, nil)
}
func TestWsDuplicateListen(t *testing.T) {
	TranVerifyDuplicateListen(t, tran, nil)
}
func TestWsConnectionRefused(t *testing.T) {
	TranVerifyConnectionRefused(t, tran, nil)
}
func TestTcpHandshake(t *testing.T) {
	TranVerifyHandshakeFail(t, tran, nil, nil)
}
func TestWsSendRecv(t *testing.T) {
	TranVerifySendRecv(t, tran, nil, nil)
}
func TestWsAnonymousPort(t *testing.T) {
	TranVerifyAnonymousPort(t, "ws://127.0.0.1:0/", nil, nil)
}
func TestWsInvalidDomain(t *testing.T) {
	TranVerifyBadAddress(t, "ws://invalid.invalid/", nil, nil)
}
func TestWsInvalidURI(t *testing.T) {
	TranVerifyBadAddress(t, "ws://127.0.0.1:80/\x01", nil, nil)
}

func TestWsInvalidLocalIP(t *testing.T) {
	TranVerifyBadLocalAddress(t, "ws://1.1.1.1:80", nil)
}
func TestWsBroadcastIP(t *testing.T) {
	TranVerifyBadAddress(t, "ws://255.255.255.255:80", nil, nil)
}

func TestWsListenerClosed(t *testing.T) {
	TranVerifyListenerClosed(t, tran, nil)
}

func TestWsResolverChange(t *testing.T) {
	sock := GetMockSocket()
	defer MustClose(t, sock)

	addr := AddrTestWS()
	MustSucceed(t, sock.Listen(addr))

	d, e := tran.NewDialer(addr, sock)
	MustSucceed(t, e)
	td := d.(*dialer)
	addr = td.addr
	td.addr = "ws://invalid.invalid:80"
	p, e := d.Dial()
	MustFail(t, e)
	MustBeTrue(t, p == nil)

	td.addr = addr
	p, e = d.Dial()
	MustSucceed(t, e)
	MustSucceed(t, p.Close())
}

func TestWsPipeOptions(t *testing.T) {
	TranVerifyPipeOptions(t, tran, nil, nil)
}

func TestWsMessageSize(t *testing.T) {
	TranVerifyMessageSizes(t, tran, nil, nil)
}

func TestWsMessageHeader(t *testing.T) {
	TranVerifyMessageHeader(t, tran, nil, nil)
}
func TestWsSendAbort(t *testing.T) {
	wd := &websocket.Dialer{}
	sock := GetMockSocket()
	defer MustClose(t, sock)
	addr := AddrTestWS()
	l, e := sock.NewListener(addr, nil)
	MustSucceed(t, e)
	MustSucceed(t, l.Listen())

	wd.Subprotocols = []string{sock.Info().PeerName + ".sp.nanomsg.org"}
	ws, _, e := wd.Dial(addr, nil)
	MustSucceed(t, e)
	MustSend(t, sock, make([]byte, 2*1024*1024)) // TCP window size is 64k
	time.Sleep(time.Millisecond * 100)
	MustSucceed(t, ws.Close())
	MustSend(t, sock, make([]byte, 2*1024*1024)) // TCP window size is 64k
}

func TestWsCheckOriginDefault(t *testing.T) {
	sock := GetMockSocket()
	defer MustClose(t, sock)
	l, e := sock.NewListener(AddrTestWS(), nil)
	MustSucceed(t, e)
	v, e := l.GetOption(OptionWebSocketCheckOrigin)
	MustSucceed(t, e)
	b, ok := v.(bool)
	MustBeTrue(t, ok)
	MustBeTrue(t, b)

	MustSucceed(t, l.SetOption(OptionWebSocketCheckOrigin, false))
	MustSucceed(t, l.SetOption(OptionWebSocketCheckOrigin, true))
}

func TestWsCheckOriginDisabled(t *testing.T) {
	sock := GetMockSocket()
	defer MustClose(t, sock)
	peer := GetMockSocket()
	defer MustClose(t, peer)
	addr := AddrTestWS()
	l, e := sock.NewListener(addr, nil)
	MustSucceed(t, l.SetOption(OptionWebSocketCheckOrigin, false))
	MustSucceed(t, l.Listen())
	MustSucceed(t, peer.Dial(addr))
	MustSucceed(t, e)
}

func TestWsBadWsVersion(t *testing.T) {
	sock := GetMockSocket()
	defer MustClose(t, sock)
	addr := AddrTestWS()
	l, e := sock.NewListener(addr, nil)
	MustSucceed(t, e)
	MustSucceed(t, l.Listen())

	url := "http://" + addr[len("ws://"):]
	req, e := http.NewRequest("GET", url, strings.NewReader(""))
	MustSucceed(t, e)
	req.Header.Set("Sec-WebSocket-Version", "10")
	req.Header.Set("Sec-WebSocket-Protocol", sock.Info().PeerName+".sp.nanomsg.org")

	client := &http.Client{}
	res, e := client.Do(req)
	MustSucceed(t, e)
	MustBeTrue(t, res.StatusCode == http.StatusBadRequest)
}

func TestWsCloseOneOfTwo(t *testing.T) {
	sock1 := GetMockSocket()
	defer MustClose(t, sock1)
	sock2 := GetMockSocket()
	defer MustClose(t, sock2)
	peer := GetMockSocket()
	defer MustClose(t, peer)

	addr := AddrTestWS()
	addr1 := addr + "redline"
	l1, e := sock1.NewListener(addr1, nil)
	MustSucceed(t, e)
	MustSucceed(t, l1.Listen())

	muxi, e := l1.GetOption(OptionWebSocketMux)
	if e != nil {
		t.Errorf("Failed get mux: %v", e)
	}

	addr2 := addr + "blueline"
	l2, e := sock2.NewListener(addr2, nil)
	MustSucceed(t, e)
	i2, e := l2.GetOption(OptionWebSocketHandler)
	MustSucceed(t, e)
	h2, ok := i2.(http.Handler)
	MustBeTrue(t, ok)

	mux := muxi.(*http.ServeMux)
	mux.Handle("/blueline", h2)

	e = l2.Listen()
	MustSucceed(t, e)

	MustSucceed(t, l2.Close())

	MustBeError(t, peer.Dial(addr2), mangos.ErrBadProto)
	MustSucceed(t, peer.Dial(addr1))
	// Nothing here, so that's a 404... which we treat as proto error.
	MustBeError(t, peer.Dial(addr+"nobody"), mangos.ErrBadProto)
}

func TestWsClosePending(t *testing.T) {
	addr := AddrTestWS()
	sock1 := GetMockSocket()
	defer MustClose(t, sock1)
	sock2 := GetMockSocket()
	defer MustClose(t, sock2)
	sock3 := GetMockSocket()
	defer MustClose(t, sock3)

	l, e := Transport.NewListener(addr, sock1)
	MustSucceed(t, e)
	MustSucceed(t, l.Listen())

	// We don't *accept* them.

	MustSucceed(t, sock2.Dial(addr))
	MustSucceed(t, sock3.Dial(addr))

	time.Sleep(time.Millisecond * 100)
	MustSucceed(t, l.Close())
	time.Sleep(time.Millisecond * 100)
}
