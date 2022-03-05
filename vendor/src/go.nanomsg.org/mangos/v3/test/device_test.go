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
	"strings"
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/pair"
	"go.nanomsg.org/mangos/v3/protocol/xpair"
	_ "go.nanomsg.org/mangos/v3/transport/all"
)

type devTest struct {
	T
}

func (dt *devTest) Init(t *testing.T, addr string) bool {
	var err error
	if dt.Sock, err = pair.NewSocket(); err != nil {
		t.Fatalf("pair.NewSocket(): %v", err)
	}
	return dt.T.Init(t, addr)
}

func (dt *devTest) SendHook(m *mangos.Message) bool {
	m.Body = append(m.Body, byte(dt.GetSend()))
	return dt.T.SendHook(m)
}

func (dt *devTest) RecvHook(m *mangos.Message) bool {
	if len(m.Body) != 1 {
		dt.Errorf("Recv message length %d != 1", len(m.Body))
		return false
	}
	if m.Body[0] != byte(dt.GetRecv()) {
		dt.Errorf("Wrong message: %d != %d", m.Body[0], byte(dt.GetRecv()))
		return false
	}
	return dt.T.RecvHook(m)
}

func deviceCaseClient() []TestCase {
	dev := &devTest{}
	dev.ID = 0
	dev.MsgSize = 4
	dev.WantTx = 50
	dev.WantRx = 50
	cases := []TestCase{dev}
	return cases
}

func testDevLoop(t *testing.T, addr string) {
	s1, err := xpair.NewSocket()
	if err != nil {
		t.Errorf("Failed to open S1: %v", err)
		return
	}
	defer s1.Close()

	options := make(map[string]interface{})
	if strings.HasPrefix(addr, "wss://") || strings.HasPrefix(addr, "tls+tcp://") {
		options[mangos.OptionTLSConfig] = srvCfg
	}

	if err := s1.ListenOptions(addr, options); err != nil {
		t.Errorf("Failed listening to %s: %v", addr, err)
		return
	}

	if err := mangos.Device(s1, s1); err != nil {
		t.Errorf("Device failed: %v", err)
		return
	}

	RunTests(t, addr, deviceCaseClient())
}

func testDevChain(t *testing.T, addr1 string, addr2 string, addr3 string) {
	// This tests using multiple devices across a few transports.
	// It looks like this:  addr1->addr2->addr3 <==> addr3->addr2->addr1
	var err error
	s := make([]mangos.Socket, 5)
	for i := 0; i < 5; i++ {
		if s[i], err = xpair.NewSocket(); err != nil {
			t.Errorf("Failed to open S1_1: %v", err)
			return
		}
		defer s[i].Close()
	}

	if err = s[0].Listen(addr1); err != nil {
		t.Errorf("s[0] Listen: %v", err)
		return
	}
	if err = s[2].Listen(addr2); err != nil {
		t.Errorf("s[2] Listen: %v", err)
		return
	}
	if err = s[4].Listen(addr3); err != nil {
		t.Errorf("s[4] Listen: %v", err)
		return
	}
	if err = s[1].Dial(addr2); err != nil {
		t.Errorf("s[1] Dial: %v", err)
		return
	}
	if err = s[3].Dial(addr3); err != nil {
		t.Errorf("s[3] Dial: %v", err)
		return
	}
	if err = mangos.Device(s[0], s[1]); err != nil {
		t.Errorf("s[0],s[1] Device: %v", err)
		return
	}
	if err = mangos.Device(s[2], s[3]); err != nil {
		t.Errorf("s[2],s[3] Device: %v", err)
		return
	}
	if err = mangos.Device(s[4], nil); err != nil {
		t.Errorf("s[4] Device: %v", err)
		return
	}
	RunTests(t, addr1, deviceCaseClient())
}

func TestDeviceChain(t *testing.T) {
	testDevChain(t, AddrTestTCP(), AddrTestWS(), AddrTestInp())
	// Some platforms (windows) need a little time to wind up the close
	time.Sleep(100 * time.Millisecond)
}

func TestDeviceLoopTCP(t *testing.T) {
	testDevLoop(t, AddrTestTCP())
}

func TestDeviceLoopInp(t *testing.T) {
	testDevLoop(t, AddrTestInp())
}

func TestDeviceLoopIPC(t *testing.T) {
	testDevLoop(t, AddrTestIPC())
}

func TestDeviceLoopTLS(t *testing.T) {
	testDevLoop(t, AddrTestTLS())
}

func TestDeviceLoopWS(t *testing.T) {
	testDevLoop(t, AddrTestWS())
}

func TestDeviceLoopWSS(t *testing.T) {
	testDevLoop(t, AddrTestWSS())
}
