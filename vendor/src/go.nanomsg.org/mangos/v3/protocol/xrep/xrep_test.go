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

package xrep

import (
	"encoding/binary"
	"testing"
	"time"

	"go.nanomsg.org/mangos/v3"
	. "go.nanomsg.org/mangos/v3/internal/test"
	. "go.nanomsg.org/mangos/v3/protocol"
	"go.nanomsg.org/mangos/v3/protocol/req"
	"go.nanomsg.org/mangos/v3/protocol/xreq"
	_ "go.nanomsg.org/mangos/v3/transport/inproc"
)

func TestXRepRaw(t *testing.T) {
	VerifyRaw(t, NewSocket)
}

func TestXRepIdentity(t *testing.T) {
	id := MustGetInfo(t, NewSocket)
	MustBeTrue(t, id.Self == ProtoRep)
	MustBeTrue(t, id.SelfName == "rep")
	MustBeTrue(t, id.Peer == ProtoReq)
	MustBeTrue(t, id.PeerName == "req")
}

func TestXRepClosed(t *testing.T) {
	VerifyClosedRecv(t, NewSocket)
	VerifyClosedSend(t, NewSocket)
	VerifyClosedClose(t, NewSocket)
	VerifyClosedDial(t, NewSocket)
	VerifyClosedListen(t, NewSocket)
	VerifyClosedAddPipe(t, NewSocket)
}

func TestXRepOptions(t *testing.T) {
	VerifyInvalidOption(t, NewSocket)
	VerifyOptionDuration(t, NewSocket, OptionRecvDeadline)
	VerifyOptionDuration(t, NewSocket, OptionSendDeadline)
	VerifyOptionInt(t, NewSocket, OptionReadQLen)
	VerifyOptionInt(t, NewSocket, OptionWriteQLen)
	VerifyOptionBool(t, NewSocket, OptionBestEffort)
	VerifyOptionTTL(t, NewSocket)
}

func TestXRepNoHeader(t *testing.T) {
	self := GetSocket(t, NewSocket)
	MustSendString(t, self, "")
	MustClose(t, self)
}

func TestXRepMismatchHeader(t *testing.T) {
	self := GetSocket(t, NewSocket)

	m := mangos.NewMessage(0)
	m.Header = append(m.Header, []byte{1, 1, 1, 1, 0x80, 0, 0, 1}...)

	MustSendMsg(t, self, m)
	MustClose(t, self)
}

func TestXRepRecvDeadline(t *testing.T) {
	self := GetSocket(t, NewSocket)
	MustSucceed(t, self.SetOption(OptionRecvDeadline, time.Millisecond))
	MustNotRecv(t, self, ErrRecvTimeout)
	MustClose(t, self)
}

func TestXRepTTLDrop(t *testing.T) {
	TTLDropTest(t, req.NewSocket, NewSocket, xreq.NewSocket, NewSocket)
}

func newRequest(id uint32, content string) *mangos.Message {
	m := mangos.NewMessage(len(content) + 8)
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b[:4], id|0x80000000)
	// Requests (coming in) will be entirely on the body.
	m.Body = append(m.Body, b...)
	m.Body = append(m.Body, []byte(content)...)
	return m
}

func newReply(id uint32, p mangos.Pipe, content string) *mangos.Message {
	m := mangos.NewMessage(len(content))
	b := make([]byte, 8)
	binary.BigEndian.PutUint32(b, p.ID())            // outgoing pipe ID
	binary.BigEndian.PutUint32(b[4:], id|0x80000000) // request ID
	m.Header = append(m.Header, b...)
	m.Body = append(m.Body, []byte(content)...)
	return m
}

func TestXRepSendTimeout(t *testing.T) {
	timeout := time.Millisecond * 10

	self := GetSocket(t, NewSocket)

	MustSucceed(t, self.SetOption(OptionWriteQLen, 0))
	MustSucceed(t, self.SetOption(OptionSendDeadline, timeout))

	_, p := MockConnect(t, self)
	MustSendMsg(t, self, newReply(0, p, "zero"))
	MustBeError(t, self.SendMsg(newReply(1, p, "one")), ErrSendTimeout)
	MustClose(t, self)
}

func TestXRepSendBestEffort(t *testing.T) {
	timeout := time.Millisecond * 10

	self := GetSocket(t, NewSocket)

	MustSucceed(t, self.SetOption(OptionWriteQLen, 0))
	MustSucceed(t, self.SetOption(OptionSendDeadline, timeout))
	MustSucceed(t, self.SetOption(OptionBestEffort, true))

	_, p := MockConnect(t, self)
	for i := 0; i < 100; i++ {
		MustSendMsg(t, self, newReply(0, p, ""))
	}
	MustClose(t, self)
}

func TestXRepPipeCloseAbort(t *testing.T) {
	self := GetSocket(t, NewSocket)

	MustSucceed(t, self.SetOption(OptionWriteQLen, 0))
	MustSucceed(t, self.SetOption(OptionSendDeadline, time.Second))

	_, p := MockConnect(t, self)
	time.AfterFunc(time.Millisecond*20, func() {
		MustSucceed(t, p.Close())
	})
	MustSendMsg(t, self, newReply(0, p, "good"))
	MustBeError(t, self.SendMsg(newReply(1, p, "bad")), ErrClosed)
	MustClose(t, self)
}

func TestXRepRecvCloseAbort(t *testing.T) {
	self := GetSocket(t, NewSocket)

	MustSucceed(t, self.SetOption(OptionReadQLen, 1))
	MustSucceed(t, self.SetOption(OptionRecvDeadline, time.Millisecond*10))

	mp, p := MockConnect(t, self)
	MockMustSendMsg(t, mp, newRequest(1, "one"), time.Second)
	MockMustSendMsg(t, mp, newRequest(2, "two"), time.Second)

	time.Sleep(time.Millisecond * 10)
	MustSucceed(t, p.Close())
	MustClose(t, self)
}

func TestXRepResizeRecv1(t *testing.T) {
	self := GetSocket(t, NewSocket)
	mp, _ := MockConnect(t, self)
	MustSucceed(t, self.SetOption(OptionReadQLen, 0))
	MustSucceed(t, self.SetOption(OptionRecvDeadline, time.Millisecond))
	MockMustSendMsg(t, mp, newRequest(1, "hello"), time.Second)

	time.Sleep(time.Millisecond * 50)
	MustSucceed(t, self.SetOption(OptionReadQLen, 2))
	MustNotRecv(t, self, ErrRecvTimeout)
	MustClose(t, self)
}

func TestXRepResizeRecv2(t *testing.T) {
	self := GetSocket(t, NewSocket)
	mp, _ := MockConnect(t, self)
	MustSucceed(t, self.SetOption(OptionReadQLen, 1))
	MustSucceed(t, self.SetOption(OptionRecvDeadline, time.Second))

	time.AfterFunc(time.Millisecond*50, func() {
		MustSucceed(t, self.SetOption(OptionReadQLen, 2))
		MockMustSendMsg(t, mp, newRequest(1, "hello"), time.Second)
	})
	MustRecvString(t, self, "hello")
	MustClose(t, self)
}

func TestXRepRecvJunk(t *testing.T) {
	self := GetSocket(t, NewSocket)
	MustSucceed(t, self.SetOption(OptionReadQLen, 20))
	MustSucceed(t, self.SetOption(OptionRecvDeadline, time.Millisecond*10))

	mp, _ := MockConnect(t, self)
	MockMustSend(t, mp, []byte{}, time.Second)
	MockMustSend(t, mp, []byte{0, 1}, time.Second)
	MockMustSend(t, mp, []byte{0, 1, 2, 3}, time.Second)
	MockMustSend(t, mp, []byte{0, 1, 2, 3, 0x80}, time.Second)

	MustNotRecv(t, self, ErrRecvTimeout)
	MustClose(t, self)
}
