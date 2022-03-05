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

package xpub

import (
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3"
	. "go.nanomsg.org/mangos/v3/internal/test"
	_ "go.nanomsg.org/mangos/v3/transport/inproc"
)

func TestXPubRaw(t *testing.T) {
	VerifyRaw(t, NewSocket)
}

func TestXPubNoRecv(t *testing.T) {
	CannotRecv(t, NewSocket)
}

func TestXPubClosed(t *testing.T) {
	VerifyClosedSend(t, NewSocket)
	VerifyClosedClose(t, NewSocket)
	VerifyClosedDial(t, NewSocket)
	VerifyClosedListen(t, NewSocket)
	VerifyClosedAddPipe(t, NewSocket)
}

func TestXPubOptions(t *testing.T) {
	VerifyInvalidOption(t, NewSocket)
	VerifyOptionQLen(t, NewSocket, mangos.OptionWriteQLen)
}

func TestXPubNonBlock(t *testing.T) {
	maxqlen := 2

	p, err := NewSocket()
	MustSucceed(t, err)
	MustNotBeNil(t, p)

	MustSucceed(t, p.SetOption(mangos.OptionWriteQLen, maxqlen))
	MustSucceed(t, p.Listen(AddrTestInp()))

	msg := []byte{'A', 'B', 'C'}

	for i := 0; i < maxqlen*10; i++ {
		MustSucceed(t, p.Send(msg))
	}
	MustSucceed(t, p.Close())
}

func TestXPubNonBlock2(t *testing.T) {
	self := GetSocket(t, NewSocket)
	MustSucceed(t, self.SetOption(mangos.OptionWriteQLen, 2))

	_, _ = MockConnect(t, self)

	for i := 0; i < 100; i++ {
		MustSendString(t, self, "yep")
	}
	MustSucceed(t, self.Close())
}

func TestXPubSendClose(t *testing.T) {
	self := GetSocket(t, NewSocket)
	MustSucceed(t, self.SetOption(mangos.OptionWriteQLen, 2))

	_, _ = MockConnect(t, self)

	time.AfterFunc(time.Millisecond*10, func() {
		MustSucceed(t, self.Close())
	})
	for {
		e := self.Send([]byte{})
		if e != nil {
			MustBeError(t, e, mangos.ErrClosed)
			break
		}
	}
	time.Sleep(time.Millisecond * 40)
	// MustSucceed(t, self.Close())
}

func TestXPubSendClose2(t *testing.T) {
	self := GetSocket(t, NewSocket)
	MustSucceed(t, self.SetOption(mangos.OptionWriteQLen, 2))
	_, _ = MockConnect(t, self)
	time.Sleep(time.Millisecond * 10)
	MustSucceed(t, self.Close())
}

func TestXPubRecvDiscard(t *testing.T) {
	self := GetSocket(t, NewSocket)
	MustSucceed(t, self.SetOption(mangos.OptionWriteQLen, 2))
	mock, _ := MockConnect(t, self)
	time.Sleep(time.Millisecond * 10)
	MockMustSendStr(t, mock, "garbage", time.Second)
	time.Sleep(time.Millisecond * 10)
	MustSucceed(t, self.Close())
}
