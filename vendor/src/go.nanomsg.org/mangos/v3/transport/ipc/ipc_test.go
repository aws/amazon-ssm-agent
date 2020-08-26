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

// +build !nacl,!plan9,!wasm

package ipc

import (
	"io/ioutil"
	"os"
	"testing"

	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/internal/test"
)

var tran = Transport

func TestMain(m *testing.M) {
	cwd, err := os.Getwd()
	if err != nil {
		panic("Failed to determine working directory")
	}

	dir, err := ioutil.TempDir("", "ipctest")
	if err != nil {
		panic("Failed to create directory")
	}
	if err = os.Chdir(dir); err != nil {
		panic("Failed to chdir: " + err.Error())
	}
	v := m.Run()
	if err = os.Chdir(cwd); err != nil {
		panic("Failed to chdir: " + err.Error())
	}
	if err = os.RemoveAll(dir); err != nil {
		panic("Failed to clean up directory: " + err.Error())
	}
	os.Exit(v)
}

func TestIpcRecvMax(t *testing.T) {
	test.TranVerifyMaxRecvSize(t, tran, nil, nil)
}

func TestIpcOptions(t *testing.T) {
	test.TranVerifyInvalidOption(t, tran)
	test.TranVerifyIntOption(t, tran, mangos.OptionMaxRecvSize)
}

func TestIpcScheme(t *testing.T) {
	test.TranVerifyScheme(t, tran)
}
func TestIpcAcceptWithoutListen(t *testing.T) {
	test.TranVerifyAcceptWithoutListen(t, tran)
}
func TestIpcListenAndAccept(t *testing.T) {
	test.TranVerifyListenAndAccept(t, tran, nil, nil)
}
func TestIpcDuplicateListen(t *testing.T) {
	test.TranVerifyDuplicateListen(t, tran, nil)
}
func TestIpcConnectionRefused(t *testing.T) {
	test.TranVerifyConnectionRefused(t, tran, nil)
}
func TestIpcHandshake(t *testing.T) {
	test.TranVerifyHandshakeFail(t, tran, nil, nil)
}
func TestIpcSendRecv(t *testing.T) {
	test.TranVerifySendRecv(t, tran, nil, nil)
}
func TestIpcListenerClosed(t *testing.T) {
	test.TranVerifyListenerClosed(t, tran, nil)
}
func TestIpcMessageSize(t *testing.T) {
	test.TranVerifyMessageSizes(t, tran, nil, nil)
}
func TestIpcMessageHeader(t *testing.T) {
	test.TranVerifyMessageHeader(t, tran, nil, nil)
}
func TestIpcVerifyPipeAddresses(t *testing.T) {
	test.TranVerifyPipeAddresses(t, tran, nil, nil)
}
func TestIpcVerifyPipeOptions(t *testing.T) {
	test.TranVerifyPipeOptions2(t, tran, nil, nil)
}
