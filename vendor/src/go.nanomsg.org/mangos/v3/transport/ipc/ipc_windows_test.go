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

// +build windows

package ipc

import (
	"net"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"

	"go.nanomsg.org/mangos/v3"
	. "go.nanomsg.org/mangos/v3/internal/test"
)

func TestIpcListenerOptions(t *testing.T) {
	sock := GetMockSocket()
	l, e := tran.NewListener(AddrTestIPC(), sock)
	MustSucceed(t, e)

	// Security Descriptor
	sd := "O:AOG:DAD:(A;;RPWPCCDCLCSWRCWDWOGA;;;S-1-0-0)"
	MustBeError(t, l.SetOption(OptionSecurityDescriptor, 0), mangos.ErrBadValue)
	MustBeError(t, l.SetOption(OptionSecurityDescriptor, true), mangos.ErrBadValue)
	MustSucceed(t, l.SetOption(OptionSecurityDescriptor, sd)) // SDDL not validated
	v, e := l.GetOption(OptionSecurityDescriptor)
	MustSucceed(t, e)
	sd2, ok := v.(string)
	MustBeTrue(t, ok)
	MustBeTrue(t, sd2 == sd)

	for _, opt := range []string{OptionInputBufferSize, OptionOutputBufferSize} {
		MustBeError(t, l.SetOption(opt, "string"), mangos.ErrBadValue)
		MustBeError(t, l.SetOption(opt, true), mangos.ErrBadValue)
		MustSucceed(t, l.SetOption(opt, int32(16384)))
		v, e = l.GetOption(opt)
		MustSucceed(t, e)
		v2, ok := v.(int32)
		MustBeTrue(t, ok)
		MustBeTrue(t, v2 == 16384)
	}
}

type testAddr string

func (a testAddr) testDial() (net.Conn, error) {
	path := "\\\\.\\pipe\\" + a[len("ipc://"):]
	return winio.DialPipe(string(path), nil)
}

func TestIpcAbortHandshake(t *testing.T) {
	sock := GetMockSocket()
	defer MustClose(t, sock)
	addr := AddrTestIPC()
	// Small buffer size so we can see the effect of early close
	l, e := sock.NewListener(addr, nil)
	MustSucceed(t, e)
	MustSucceed(t, l.SetOption(OptionOutputBufferSize, int32(2)))
	MustSucceed(t, l.Listen())
	c, e := testAddr(addr).testDial()
	MustSucceed(t, e)
	MustSucceed(t, c.Close())
}

func TestIpcBadHandshake(t *testing.T) {
	sock := GetMockSocket()
	defer MustClose(t, sock)
	addr := AddrTestIPC()
	l, e := sock.NewListener(addr, nil)
	MustSucceed(t, e)
	MustSucceed(t, l.Listen())
	TranSendConnBadHandshakes(t, testAddr(addr).testDial)
}

func TestIpcBadRecv(t *testing.T) {
	sock := GetMockSocket()
	defer MustClose(t, sock)
	addr := AddrTestIPC()
	l, e := sock.NewListener(addr, nil)
	MustSucceed(t, e)
	MustSucceed(t, l.Listen())
	TranSendBadMessages(t, sock.Info().Peer, true, testAddr(addr).testDial)
}

func TestIpcSendAbort(t *testing.T) {
	sock := GetMockSocket()
	defer MustClose(t, sock)
	addr := AddrTestIPC()
	l, e := sock.NewListener(addr, nil)
	MustSucceed(t, e)
	MustSucceed(t, l.SetOption(OptionOutputBufferSize, int32(128)))
	MustSucceed(t, l.Listen())
	c, e := testAddr(addr).testDial()
	MustSucceed(t, e)
	TranConnHandshake(t, c, sock.Info().Peer)
	MustSend(t, sock, make([]byte, 65536))
	time.Sleep(time.Millisecond * 100)
	MustSucceed(t, c.Close())
}
