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
	"sync"
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol"
)

// VerifyClosedSend verifies that Send on the socket created returns protocol.ErrClosed if it is closed.
func VerifyClosedSend(t *testing.T, f func() (mangos.Socket, error)) {
	s, err := f()
	MustSucceed(t, err)
	MustSucceed(t, s.Close())
	err = s.Send([]byte{})
	MustFail(t, err)
	MustBeTrue(t, err == protocol.ErrClosed)
}

// VerifyClosedRecv verifies that Recv on the socket created returns protocol.ErrClosed if it is closed.
func VerifyClosedRecv(t *testing.T, f func() (mangos.Socket, error)) {
	s, err := f()
	MustSucceed(t, err)
	MustSucceed(t, s.Close())
	_, err = s.Recv()
	MustFail(t, err)
	MustBeTrue(t, err == protocol.ErrClosed)
}

// VerifyClosedClose verifies that Close on an already closed socket returns protocol.ErrClosed.
func VerifyClosedClose(t *testing.T, f func() (mangos.Socket, error)) {
	s, err := f()
	MustSucceed(t, err)
	MustSucceed(t, s.Close())
	err = s.Close()
	MustFail(t, err)
	MustBeTrue(t, err == protocol.ErrClosed)
}

// VerifyClosedListen verifies that Listen returns protocol.ErrClosed on a closed socket.
func VerifyClosedListen(t *testing.T, f func() (mangos.Socket, error)) {
	s := GetSocket(t, f)
	AddMockTransport()
	MustClose(t, s)
	MustBeError(t, s.Listen(AddrMock()), protocol.ErrClosed)
}

// VerifyClosedDial verifies that Dial returns protocol.ErrClosed on a closed socket.
func VerifyClosedDial(t *testing.T, f func() (mangos.Socket, error)) {
	s := GetSocket(t, f)
	AddMockTransport()
	MustClose(t, s)
	err := s.DialOptions(AddrMock(), map[string]interface{}{
		mangos.OptionDialAsynch: true,
	})
	MustBeError(t, err, protocol.ErrClosed)
}

func VerifyClosedAddPipe(t *testing.T, f func() (mangos.Socket, error)) {
	AddMockTransport()
	s := GetSocket(t, f)
	peer := s.Info().Peer
	d, mc := GetMockDialer(t, s)
	var wg sync.WaitGroup
	wg.Add(1)
	pass := false
	go func() {
		defer wg.Done()
		// The pipe is connected, but then immediately detached.
		MustSucceed(t, d.Dial())
		pass = true
	}()

	time.Sleep(time.Millisecond * 10)
	mc.DeferClose(true)
	MustSucceed(t, s.Close())
	time.Sleep(time.Millisecond * 20)
	mp := mc.NewPipe(peer)
	MustSucceed(t, mc.AddPipe(mp))
	time.Sleep(time.Millisecond * 10)
	mc.DeferClose(false)
	wg.Wait()
	MustBeTrue(t, pass)
}

func VerifyClosedContext(t *testing.T, f func() (mangos.Socket, error)) {
	s := GetSocket(t, f)
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
