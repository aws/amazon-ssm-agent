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

package transport

// Handshaker is used to support dealing with asynchronous
// handshaking used for some transports.  This allows the
// initial handshaking to be done in the background, without
// stalling the server's accept queue.  This is important to
// ensure that a slow remote peer cannot bog down the server
// or effect a denial-of-service for new connections.
type Handshaker interface {
	// Start injects a pipe into the handshaker.  The
	// handshaking is done asynchronously on a Go routine.
	Start(Pipe)

	// Waits for until a pipe has completely finished the
	// handshaking and returns it.
	Wait() (Pipe, error)

	// Close is used to close the handshaker.  Any existing
	// negotiations will be canceled, and the underlying
	// transport sockets will be closed.  Any new attempts
	// to start will return mangos.ErrClosed.
	Close()
}
