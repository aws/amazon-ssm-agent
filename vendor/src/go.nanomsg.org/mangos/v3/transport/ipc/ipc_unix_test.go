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

// +build !plan9,!windows,!js

package ipc

import (
	"errors"
	"net"
	"os"
	"syscall"
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3"
	. "go.nanomsg.org/mangos/v3/internal/test"
)

func TestIsSyscallError(t *testing.T) {

	MustBeFalse(t, isSyscallError(errors.New("nope"), syscall.ENOENT))
	MustBeFalse(t, isSyscallError(&net.OpError{
		Op:     "test",
		Net:    "none",
		Source: nil,
		Addr:   nil,
		Err:    mangos.ErrClosed,
	}, syscall.ENOENT))
	MustBeFalse(t, isSyscallError(&net.OpError{
		Op:     "test",
		Net:    "none",
		Source: nil,
		Addr:   nil,
		Err: &os.SyscallError{
			Syscall: "none",
			Err:     syscall.EINVAL,
		},
	}, syscall.ENOENT))
	MustBeFalse(t, isSyscallError(&net.OpError{
		Op:     "test",
		Net:    "none",
		Source: nil,
		Addr:   nil,
		Err: &os.SyscallError{
			Syscall: "none",
			Err:     mangos.ErrNotRaw,
		},
	}, syscall.ENOENT))
	MustBeTrue(t, isSyscallError(&net.OpError{
		Op:     "test",
		Net:    "none",
		Source: nil,
		Addr:   nil,
		Err: &os.SyscallError{
			Syscall: "none",
			Err:     syscall.ENOENT,
		},
	}, syscall.ENOENT))
}

func TestIpcStaleListen(t *testing.T) {
	addr1 := AddrTestIPC()
	name := addr1[len("ipc://"):]
	defer func() {
		_ = os.Remove(name)
		_ = os.Remove(name + ".hold")
	}()

	uaddr, _ := net.ResolveUnixAddr("unix", name)
	sock, err := net.ListenUnix("unix", uaddr)

	MustSucceed(t, err)

	// We rename it so that closing won't unlink the socket.
	// This lets us leave a stale socket behind.
	MustSucceed(t, os.Rename(name, name+".hold"))
	MustSucceed(t, sock.Close())
	MustSucceed(t, os.Rename(name+".hold", name))

	// Clean up the stale link.
	self := GetMockSocket()

	MustSucceed(t, self.Listen(addr1))
	defer MustClose(t, self)
}

func TestIpcBusyListen(t *testing.T) {
	addr1 := AddrTestIPC()
	name := addr1[len("ipc://"):]

	uaddr, _ := net.ResolveUnixAddr("unix", name)
	sock, err := net.ListenUnix("unix", uaddr)
	defer func() {
		_ = sock.Close()
	}()

	MustSucceed(t, err)

	self := GetMockSocket()

	MustBeError(t, self.Listen(addr1), mangos.ErrAddrInUse)
	defer MustClose(t, self)
}

func TestIpcFileConflictListen(t *testing.T) {
	addr1 := AddrTestIPC()
	name := addr1[len("ipc://"):]

	file, err := os.Create(name)
	MustSucceed(t, err)
	_, _ = file.WriteString("abc")
	_ = file.Close()
	defer func() {
		MustSucceed(t, os.Remove(name))
	}()

	self := GetMockSocket()

	MustBeError(t, self.Listen(addr1), mangos.ErrAddrInUse)
	defer MustClose(t, self)
}

type testAddr string

func (a testAddr) testDial() (net.Conn, error) {
	return net.Dial("unix", string(a)[len("ipc://"):])
}

func TestIpcAbortHandshake(t *testing.T) {
	sock := GetMockSocket()
	defer MustClose(t, sock)
	addr := AddrTestIPC()
	l, e := sock.NewListener(addr, nil)
	MustSucceed(t, e)
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
	MustSucceed(t, l.Listen())
	c, e := testAddr(addr).testDial()
	MustSucceed(t, e)
	TranConnHandshake(t, c, sock.Info().Peer)
	MustSend(t, sock, make([]byte, 1024*1024)) // TCP window size is 64k
	time.Sleep(time.Millisecond * 100)
	MustSucceed(t, c.Close())
}
