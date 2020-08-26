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
)

// CannotSend verifies that the socket cannot send.
func CannotSend(t *testing.T, f func() (mangos.Socket, error)) {
	s := GetSocket(t, f)

	// Not all protocols support this option, but try.
	_ = s.SetOption(mangos.OptionSendDeadline, time.Millisecond)

	MustBeError(t, s.Send([]byte{0, 1, 2, 3}), mangos.ErrProtoOp)
	MustSucceed(t, s.Close())
}

// CannotRecv verifies that the socket cannot recv.
func CannotRecv(t *testing.T, f func() (mangos.Socket, error)) {
	s := GetSocket(t, f)
	_ = s.SetOption(mangos.OptionRecvDeadline, time.Millisecond)

	v, err := s.Recv()
	MustBeError(t, err, mangos.ErrProtoOp)
	MustBeNil(t, v)
	MustSucceed(t, s.Close())
}

// GetSocket returns a socket using the constructor function.
func GetSocket(t *testing.T, f func() (mangos.Socket, error)) mangos.Socket {
	s, err := f()
	MustSucceed(t, err)
	MustNotBeNil(t, s)
	return s
}

// MustClose closes the socket.
func MustClose(t *testing.T, s mangos.Socket) {
	MustSucceed(t, s.Close())
}

// MustGetInfo returns the Info for the socket.
func MustGetInfo(t *testing.T, f func() (mangos.Socket, error)) mangos.ProtocolInfo {
	s := GetSocket(t, f)
	id := s.Info()
	MustClose(t, s)
	return id
}

// ConnectPairVia connects two sockets using the given address.  The pipe event
// hook is used for this operation, and the function does not return until both
// sockets have seen the connection.
func ConnectPairVia(t *testing.T, addr string, s1, s2 mangos.Socket, o1, o2 map[string]interface{}) {
	wg1 := sync.WaitGroup{}
	wg2 := sync.WaitGroup{}
	wg1.Add(1)
	wg2.Add(1)
	h1 := s1.SetPipeEventHook(func(ev mangos.PipeEvent, p mangos.Pipe) {
		if ev == mangos.PipeEventAttached {
			wg1.Done()
		}
	})
	h2 := s2.SetPipeEventHook(func(ev mangos.PipeEvent, p mangos.Pipe) {
		if ev == mangos.PipeEventAttached {
			wg2.Done()
		}
	})
	MustSucceed(t, s1.ListenOptions(addr, o1))
	MustSucceed(t, s2.SetOption(mangos.OptionDialAsynch, true))
	MustSucceed(t, s2.DialOptions(addr, o2))

	wg1.Wait()
	wg2.Wait()
	s1.SetPipeEventHook(h1)
	s2.SetPipeEventHook(h2)
}

// ConnectPair is like ConnectPairVia but uses inproc.
func ConnectPair(t *testing.T, s1 mangos.Socket, s2 mangos.Socket) {
	ConnectPairVia(t, AddrTestInp(), s1, s2, nil, nil)
}

func MustRecv(t *testing.T, s mangos.Socket) []byte {
	m, e := s.Recv()
	MustSucceed(t, e)
	MustNotBeNil(t, m)
	return m
}

func MustNotRecv(t *testing.T, s mangos.Socket, err error) {
	m, e := s.Recv()
	MustBeError(t, e, err)
	MustBeNil(t, m)
}

func MustRecvMsg(t *testing.T, s mangos.Socket) *mangos.Message {
	m, e := s.RecvMsg()
	MustSucceed(t, e)
	MustNotBeNil(t, m)
	return m
}

func MustSendMsg(t *testing.T, s mangos.Socket, m *mangos.Message) {
	MustSucceed(t, s.SendMsg(m))
}

func MustSend(t *testing.T, s mangos.Socket, b []byte) {
	MustSucceed(t, s.Send(b))
}

func MustSendString(t *testing.T, s mangos.Socket, m string) {
	MustSend(t, s, []byte(m))
}

func MustRecvString(t *testing.T, s mangos.Socket, m string) {
	b := MustRecv(t, s)
	MustBeTrue(t, string(b) == m)
}
