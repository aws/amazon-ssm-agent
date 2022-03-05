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

package wss

import (
	"crypto/tls"
	"testing"

	"go.nanomsg.org/mangos/v3"
	. "go.nanomsg.org/mangos/v3/internal/test"
)

var tran = Transport

var lOpts map[string]interface{}
var dOpts map[string]interface{}

func init() {
	dOpts = make(map[string]interface{})
	lOpts = make(map[string]interface{})
	var t *testing.T
	dOpts[mangos.OptionTLSConfig] = GetTLSConfig(t, false)
	lOpts[mangos.OptionTLSConfig] = GetTLSConfig(t, true)
}

func TestWssOptions(t *testing.T) {
	TranVerifyInvalidOption(t, tran)
	TranVerifyIntOption(t, tran, mangos.OptionMaxRecvSize)
	TranVerifyNoDelayOption(t, tran)
	TranVerifyTLSConfigOption(t, tran)
}

func TestWssScheme(t *testing.T) {
	TranVerifyScheme(t, tran)
}
func TestWssRecvMax(t *testing.T) {
	TranVerifyMaxRecvSize(t, tran, dOpts, lOpts)
}
func TestWssAcceptWithoutListen(t *testing.T) {
	TranVerifyAcceptWithoutListen(t, tran)
}
func TestWssListenAndAccept(t *testing.T) {
	TranVerifyListenAndAccept(t, tran, dOpts, lOpts)
}
func TestWssDuplicateListen(t *testing.T) {
	TranVerifyDuplicateListen(t, tran, lOpts)
}
func TestWssConnectionRefused(t *testing.T) {
	TranVerifyConnectionRefused(t, tran, dOpts)
}
func TestWssSendRecv(t *testing.T) {
	TranVerifySendRecv(t, tran, dOpts, lOpts)
}
func TestWssListenNoCert(t *testing.T) {
	sock := GetMockSocket()
	defer MustClose(t, sock)

	addr := AddrTestWSS()
	MustBeError(t, sock.ListenOptions(addr, nil), mangos.ErrTLSNoConfig)

	cfg := &tls.Config{}
	opts := make(map[string]interface{})
	opts[mangos.OptionTLSConfig] = cfg
	MustBeError(t, sock.ListenOptions(addr, opts), mangos.ErrTLSNoCert)
}

func TestWssDialNoCert(t *testing.T) {
	TranVerifyDialNoCert(t, tran)
}

func TestWssDialInsecure(t *testing.T) {
	TranVerifyDialInsecure(t, tran)
}

func TestWssMessageSize(t *testing.T) {
	TranVerifyMessageSizes(t, tran, dOpts, lOpts)
}

func TestWssMessageHeader(t *testing.T) {
	TranVerifyMessageHeader(t, tran, dOpts, lOpts)
}
