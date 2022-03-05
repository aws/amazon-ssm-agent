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

// pair implements a pair example.  node0 is a listening
// pair socket, and node1 is a dialing pair socket.
//
// To use:
//
//   $ go build .
//   $ url=tcp://127.0.0.1:40899
//   $ ./pair node0 $url & node0=$!
//   $ ./pair node1 $url & node1=$!
//   $ sleep 3
//   $ kill $node0 $node1
//
package main

import (
	"fmt"
	"os"
	"time"

	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/pair"

	// register transports
	_ "go.nanomsg.org/mangos/v3/transport/all"
)

func die(format string, v ...interface{}) {
	fmt.Fprintln(os.Stderr, fmt.Sprintf(format, v...))
	os.Exit(1)
}

func sendName(sock mangos.Socket, name string) {
	fmt.Printf("%s: SENDING \"%s\"\n", name, name)
	if err := sock.Send([]byte(name)); err != nil {
		die("failed sending: %s", err)
	}
}

func recvName(sock mangos.Socket, name string) {
	var msg []byte
	var err error
	if msg, err = sock.Recv(); err == nil {
		fmt.Printf("%s: RECEIVED: \"%s\"\n", name, string(msg))
	}
}

func sendRecv(sock mangos.Socket, name string) {
	for {
		sock.SetOption(mangos.OptionRecvDeadline, 100*time.Millisecond)
		recvName(sock, name)
		time.Sleep(time.Second)
		sendName(sock, name)
	}
}

func node0(url string) {
	var sock mangos.Socket
	var err error
	if sock, err = pair.NewSocket(); err != nil {
		die("can't get new pair socket: %s", err)
	}
	if err = sock.Listen(url); err != nil {
		die("can't listen on pair socket: %s", err.Error())
	}
	sendRecv(sock, "node0")
}

func node1(url string) {
	var sock mangos.Socket
	var err error

	if sock, err = pair.NewSocket(); err != nil {
		die("can't get new pair socket: %s", err.Error())
	}
	if err = sock.Dial(url); err != nil {
		die("can't dial on pair socket: %s", err.Error())
	}
	sendRecv(sock, "node1")
}

func main() {
	if len(os.Args) > 2 && os.Args[1] == "node0" {
		node0(os.Args[2])
		os.Exit(0)
	}
	if len(os.Args) > 2 && os.Args[1] == "node1" {
		node1(os.Args[2])
		os.Exit(0)
	}
	fmt.Fprintf(os.Stderr, "Usage: pair node0|node1 <URL>\n")
	os.Exit(1)
}
