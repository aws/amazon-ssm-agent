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

package main

import (
	"encoding/binary"
	"fmt"
	"sync"

	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/req"

	// register transports
	_ "go.nanomsg.org/mangos/v3/transport/all"
)

// synchronize our output messaging so we don't overlap
var lock sync.Mutex

func clientWorker(url string, id int) {
	var sock mangos.Socket
	var m *mangos.Message
	var err error

	if sock, err = req.NewSocket(); err != nil {
		die("can't get new req socket: %s", err.Error())
	}

	// Leave this in Cooked mode!

	if err = sock.Dial(url); err != nil {
		die("can't dial on req socket: %s", err.Error())
	}

	// send an empty messsage
	m = mangos.NewMessage(1)
	if err = sock.SendMsg(m); err != nil {
		die("can't send request: %s", err.Error())
	}

	if m, err = sock.RecvMsg(); err != nil {
		die("can't recv reply: %s", err.Error())
	}
	sock.Close()

	if len(m.Body) != 4 {
		die("bad response len: %d", len(m.Body))
	}

	worker := binary.BigEndian.Uint32(m.Body[0:])

	lock.Lock()
	fmt.Printf("Client: %4d   Server: %4d\n", id, worker)
	lock.Unlock()
}

func client(url string, nworkers int) {

	var wg sync.WaitGroup
	for i := 0; i < nworkers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			clientWorker(url, i)
		}(i)
	}
	wg.Wait()

}
