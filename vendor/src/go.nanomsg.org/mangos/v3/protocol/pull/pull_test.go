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

package pull

import (
	"testing"

	"go.nanomsg.org/mangos/v3"
	. "go.nanomsg.org/mangos/v3/internal/test"
)

func TestPullIdentity(t *testing.T) {
	s := GetSocket(t, NewSocket)
	id := s.Info()
	MustBeTrue(t, id.Self == mangos.ProtoPull)
	MustBeTrue(t, id.SelfName == "pull")
	MustBeTrue(t, id.Peer == mangos.ProtoPush)
	MustBeTrue(t, id.PeerName == "push")
	MustSucceed(t, s.Close())
}

func TestPullCooked(t *testing.T) {
	VerifyCooked(t, NewSocket)
}

func TestPullNoContext(t *testing.T) {
	s := GetSocket(t, NewSocket)
	_, e := s.OpenContext()
	MustBeError(t, e, mangos.ErrProtoOp)
	MustSucceed(t, s.Close())
}

func TestPullOptions(t *testing.T) {
	VerifyInvalidOption(t, NewSocket)
}
