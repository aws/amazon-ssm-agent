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

package inproc

import (
	"testing"

	. "go.nanomsg.org/mangos/v3/internal/test"
)

var tran = Transport

func TestInpOptions(t *testing.T) {
	TranVerifyInvalidOption(t, tran)
}

func TestInpScheme(t *testing.T) {
	TranVerifyScheme(t, tran)
}
func TestInpAcceptWithoutListen(t *testing.T) {
	TranVerifyAcceptWithoutListen(t, tran)
}
func TestInpListenAndAccept(t *testing.T) {
	TranVerifyListenAndAccept(t, tran, nil, nil)
}
func TestInpDuplicateListen(t *testing.T) {
	TranVerifyDuplicateListen(t, tran, nil)
}
func TestInpConnectionRefused(t *testing.T) {
	TranVerifyConnectionRefused(t, tran, nil)
}
func TestInpHandshake(t *testing.T) {
	TranVerifyHandshakeFail(t, tran, nil, nil)
}
func TestInpSendRecv(t *testing.T) {
	TranVerifySendRecv(t, tran, nil, nil)
}
func TestInpListenerClosed(t *testing.T) {
	TranVerifyListenerClosed(t, tran, nil)
}
func TestInpPipeOptions(t *testing.T) {
	TranVerifyPipeOptions(t, tran, nil, nil)
}
func TestInpMessageSize(t *testing.T) {
	TranVerifyMessageSizes(t, tran, nil, nil)
}
func TestInpMessageHeader(t *testing.T) {
	TranVerifyMessageHeader(t, tran, nil, nil)
}
