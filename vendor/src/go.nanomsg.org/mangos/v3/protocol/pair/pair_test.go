/*
 * Copyright  2019 The Mangos Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use file except in compliance with the License.
 *  You may obtain a copy of the license at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package pair

import (
	"testing"

	"go.nanomsg.org/mangos/v3"

	. "go.nanomsg.org/mangos/v3/internal/test"
	_ "go.nanomsg.org/mangos/v3/transport/inproc"
)

func TestPairIdentity(t *testing.T) {
	s, e := NewSocket()
	MustSucceed(t, e)
	id := s.Info()
	MustBeTrue(t, id.Self == mangos.ProtoPair)
	MustBeTrue(t, id.Peer == mangos.ProtoPair)
	MustBeTrue(t, id.SelfName == "pair")
	MustBeTrue(t, id.PeerName == "pair")
	MustSucceed(t, s.Close())
}

func TestPairCooked(t *testing.T) {
	VerifyCooked(t, NewSocket)
}

func TestPairClosed(t *testing.T) {
	VerifyClosedSend(t, NewSocket)
	VerifyClosedRecv(t, NewSocket)
	VerifyClosedClose(t, NewSocket)
	VerifyClosedDial(t, NewSocket)
	VerifyClosedListen(t, NewSocket)
	VerifyClosedAddPipe(t, NewSocket)
}

func TestPairOptions(t *testing.T) {
	VerifyInvalidOption(t, NewSocket)
	VerifyOptionDuration(t, NewSocket, mangos.OptionRecvDeadline)
	VerifyOptionDuration(t, NewSocket, mangos.OptionSendDeadline)
	VerifyOptionInt(t, NewSocket, mangos.OptionReadQLen)
	VerifyOptionInt(t, NewSocket, mangos.OptionWriteQLen)
	VerifyOptionBool(t, NewSocket, mangos.OptionBestEffort)
}
