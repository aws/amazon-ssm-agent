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

package main

import (
	"fmt"
	"log"
	"time"

	"go.nanomsg.org/mangos/v3/protocol/pair"
	"go.nanomsg.org/mangos/v3/transport/all"
)

// ThroughputServer is the server side -- very much equivalent to local_thr in
// nanomsg/perf.  It does the measurement by counting packets received.
func ThroughputServer(addr string, msgSize int, count int) {
	s, err := pair.NewSocket()
	if err != nil {
		log.Fatalf("Failed to make new pair socket: %v", err)
	}
	defer func() {
		_ = s.Close()
	}()

	all.AddTransports(s)
	l, err := s.NewListener(addr, nil)
	if err != nil {
		log.Fatalf("Failed to make new listener: %v", err)
	}

	if err = l.Listen(); err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	msg, err := s.RecvMsg()
	if err != nil {
		log.Fatalf("Failed to receive start message: %v", err)
	}
	msg.Free()

	start := time.Now()

	for i := 0; i != count; i++ {
		msg, err := s.RecvMsg()
		if err != nil {
			log.Fatalf("Failed to recv: %v", err)
		}
		if len(msg.Body) != msgSize {
			log.Fatalf("Received wrong message size: %d != %d", len(msg.Body), msgSize)
		}
		// return to cache to avoid GC
		msg.Free()
	}

	finish := time.Now()

	delta := finish.Sub(start)
	deltasec := float64(delta) / float64(time.Second)
	msgpersec := float64(count) / deltasec
	mbps := (float64((count)*8*msgSize) / deltasec) / 1000000.0
	fmt.Printf("message size: %d [B]\n", msgSize)
	fmt.Printf("message count: %d\n", count)
	fmt.Printf("throughput: %d [msg/s]\n", uint64(msgpersec))
	fmt.Printf("throughput: %.3f [Mb/s]\n", mbps)
}

// ThroughputClient is the client side of the latency test.  It simply sends
// the requested number of packets of given size to the server.  It corresponds
// to remote_thr.
func ThroughputClient(addr string, msgSize int, count int) {
	s, err := pair.NewSocket()
	if err != nil {
		log.Fatalf("Failed to make new pair socket: %v", err)
	}
	defer func() {
		_ = s.Close()
	}()

	all.AddTransports(s)
	d, err := s.NewDialer(addr, nil)
	if err != nil {
		log.Fatalf("Failed to make new dialer: %v", err)
	}

	err = d.Dial()
	if err != nil {
		log.Fatalf("Failed to dial: %v", err)
	}

	// 100 milliseconds to give TCP a chance to establish
	time.Sleep(time.Millisecond * 100)

	body := make([]byte, msgSize)
	for i := 0; i < msgSize; i++ {
		body[i] = 111
	}

	// send the start message
	_ = s.Send([]byte{})

	for i := 0; i < count; i++ {
		if err = s.Send(body); err != nil {
			log.Fatalf("Failed SendMsg: %v", err)
		}
	}
}
