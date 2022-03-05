// Copyright 2018 The Mangos Authors
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
	"go.nanomsg.org/mangos/v3/protocol/pair"
	"go.nanomsg.org/mangos/v3/protocol/xpair"
	_ "go.nanomsg.org/mangos/v3/transport/all"
)

func TestDeviceBadPair(t *testing.T) {
	s1 := GetMockSocketRaw(1, 1, "mock1", "mock1", true, nil)
	defer MustClose(t, s1)
	s2 := GetMockSocketRaw(2, 2, "mock2", "mock2", true, nil)
	defer MustClose(t, s2)

	MustBeError(t, mangos.Device(s1, s2), mangos.ErrBadProto)
}

func TestDeviceBadSingle(t *testing.T) {
	s1 := GetMockSocketRaw(1, 2, "mock1", "mock2", true, nil)
	defer MustClose(t, s1)
	MustBeError(t, mangos.Device(s1, s1), mangos.ErrBadProto)
}

func TestDeviceFirstNil(t *testing.T) {
	s1 := GetMockSocketRaw(1, 1, "m", "m", true, nil)
	defer MustClose(t, s1)
	MustSucceed(t, mangos.Device(nil, s1))
}

func TestDeviceSecondNil(t *testing.T) {
	s1 := GetMockSocketRaw(1, 1, "m", "m", true, nil)
	defer MustClose(t, s1)
	MustSucceed(t, mangos.Device(s1, nil))
}

func TestDeviceBothNil(t *testing.T) {
	MustBeError(t, mangos.Device(nil, nil), mangos.ErrClosed)
}

func TestDeviceRawError(t *testing.T) {
	s0 := GetMockSocketRaw(1, 1, "m", "m", true, nil)
	defer MustClose(t, s0)
	s1 := GetMockSocketRaw(1, 1, "m", "m", nil, mangos.ErrCanceled)
	defer MustClose(t, s1)
	MustBeError(t, mangos.Device(s1, s0), mangos.ErrCanceled)
	MustBeError(t, mangos.Device(s0, s1), mangos.ErrCanceled)
	s2 := GetMockSocketRaw(1, 1, "m", "m", 5, nil)
	defer MustClose(t, s2)
	MustBeError(t, mangos.Device(s2, s0), mangos.ErrNotRaw)
	MustBeError(t, mangos.Device(s0, s2), mangos.ErrNotRaw)
}

func TestDeviceCookedFirst(t *testing.T) {
	s1 := GetMockSocketRaw(1, 2, "m1", "m2", false, nil)
	defer MustClose(t, s1)
	s2 := GetMockSocketRaw(2, 1, "m2", "m1", true, nil)
	defer MustClose(t, s2)

	MustBeError(t, mangos.Device(s1, s2), mangos.ErrNotRaw)
}
func TestDeviceCookedSecond(t *testing.T) {
	s1 := GetMockSocketRaw(1, 2, "m1", "m2", true, nil)
	defer MustClose(t, s1)
	s2 := GetMockSocketRaw(2, 1, "m2", "m1", false, nil)
	defer MustClose(t, s2)

	MustBeError(t, mangos.Device(s1, s2), mangos.ErrNotRaw)
}

func TestDeviceCookedBoth(t *testing.T) {
	s1 := GetMockSocketRaw(1, 2, "m1", "m2", false, nil)
	defer MustClose(t, s1)
	s2 := GetMockSocketRaw(2, 1, "m2", "m1", false, nil)
	defer MustClose(t, s2)

	MustBeError(t, mangos.Device(s1, s2), mangos.ErrNotRaw)
}
func TestDeviceCookedNeither(t *testing.T) {
	s1 := GetMockSocketRaw(1, 2, "m1", "m2", true, nil)
	defer MustClose(t, s1)
	s2 := GetMockSocketRaw(2, 1, "m2", "m1", true, nil)
	defer MustClose(t, s2)

	MustSucceed(t, mangos.Device(s1, s2))
}

func TestDevicePass(t *testing.T) {
	s1 := GetSocket(t, xpair.NewSocket)
	defer MustClose(t, s1)
	s2 := GetSocket(t, xpair.NewSocket)
	defer MustClose(t, s2)
	tx := GetSocket(t, pair.NewSocket)
	defer MustClose(t, tx)
	rx := GetSocket(t, pair.NewSocket)
	defer MustClose(t, rx)

	ConnectPair(t, tx, s1)
	ConnectPair(t, rx, s2)

	MustSucceed(t, mangos.Device(s1, s2))
	var wg sync.WaitGroup
	wg.Add(1)

	pass := false
	go func() {
		defer wg.Done()
		MustRecvString(t, rx, "ping")
		pass = true
	}()

	MustSendString(t, tx, "ping")
	wg.Wait()
	MustBeTrue(t, pass)
}

func TestDeviceCloseSend(t *testing.T) {
	s1 := GetSocket(t, xpair.NewSocket)
	defer MustClose(t, s1)
	s2 := GetSocket(t, xpair.NewSocket)
	defer MustClose(t, s2)
	tx := GetSocket(t, pair.NewSocket)
	rx := GetSocket(t, pair.NewSocket)

	ConnectPair(t, tx, s1)
	ConnectPair(t, rx, s2)
	var wg sync.WaitGroup

	MustSucceed(t, mangos.Device(s1, s2))

	pass := false
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			e := tx.Send([]byte{})
			if e != nil {
				MustBeError(t, e, mangos.ErrClosed)
				pass = true
				break
			}
		}
	}()

	time.Sleep(time.Millisecond * 100)
	MustClose(t, rx)
	time.Sleep(time.Millisecond * 20)
	MustClose(t, tx)
	wg.Wait()
	MustBeTrue(t, pass)
}
