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

package star

import (
	"testing"

	"go.nanomsg.org/mangos/v3"
	. "go.nanomsg.org/mangos/v3/internal/test"
	. "go.nanomsg.org/mangos/v3/protocol"
	"go.nanomsg.org/mangos/v3/protocol/xstar"
	_ "go.nanomsg.org/mangos/v3/transport/inproc"
)

func TestStarIdentity(t *testing.T) {
	id := GetSocket(t, NewSocket).Info()
	MustBeTrue(t, id.Self == ProtoStar)
	MustBeTrue(t, id.Peer == ProtoStar)
	MustBeTrue(t, id.SelfName == "star")
	MustBeTrue(t, id.PeerName == "star")
}

func TestStarCooked(t *testing.T) {
	VerifyCooked(t, NewSocket)
}

func TestStarClosed(t *testing.T) {
	VerifyClosedRecv(t, NewSocket)
	VerifyClosedSend(t, NewSocket)
	VerifyClosedClose(t, NewSocket)
	VerifyClosedDial(t, NewSocket)
	VerifyClosedListen(t, NewSocket)
	VerifyClosedAddPipe(t, NewSocket)
}

func TestStarDiscardHeader(t *testing.T) {
	s1 := GetSocket(t, NewSocket)
	s2 := GetSocket(t, NewSocket)
	ConnectPair(t, s1, s2)

	m := mangos.NewMessage(0)
	m.Header = append(m.Header, 0, 1, 2, 3)
	m.Body = append(m.Body, 'a', 'b', 'c')

	MustSendMsg(t, s1, m)
	m = MustRecvMsg(t, s2)
	MustBeTrue(t, len(m.Header) == 0)
	MustBeTrue(t, string(m.Body) == "abc")
}

func TestStarTTL(t *testing.T) {
	SetTTLZero(t, NewSocket)
	SetTTLNegative(t, NewSocket)
	SetTTLTooBig(t, NewSocket)
	SetTTLNotInt(t, NewSocket)
	SetTTL(t, NewSocket)
	TTLDropTest(t, NewSocket, NewSocket, xstar.NewSocket, xstar.NewSocket)
}
