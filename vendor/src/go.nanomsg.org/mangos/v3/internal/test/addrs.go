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
	"fmt"
	"sync/atomic"
	"time"
)

var currPort uint32

func init() {
	currPort = uint32(time.Now().UnixNano()%20000 + 20000)
}

// NextPort returns the next port, incrementing by one.
func NextPort() uint32 {
	return atomic.AddUint32(&currPort, 1)
}

// AddrTestIPC returns a test IPC address.  It will be in the current
// directory.
func AddrTestIPC() string {
	return (fmt.Sprintf("ipc://mangostest%d", NextPort()))
}

// AddrTestWSS returns a websocket over TLS address.
func AddrTestWSS() string {
	return (fmt.Sprintf("wss://127.0.0.1:%d/", NextPort()))
}

// AddrTestWS returns a websocket address.
func AddrTestWS() string {
	return (fmt.Sprintf("ws://127.0.0.1:%d/", NextPort()))
}

// AddrTestTCP returns a TCP address.
func AddrTestTCP() string {
	return (fmt.Sprintf("tcp://127.0.0.1:%d", NextPort()))
}

// AddrTestTLS returns a TLS over TCP address.
func AddrTestTLS() string {
	return (fmt.Sprintf("tls+tcp://127.0.0.1:%d", NextPort()))
}

// AddrTestInp returns an inproc address.
func AddrTestInp() string {
	return (fmt.Sprintf("inproc://test_%d", NextPort()))
}
