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
// WITHOUT WARRANTIES O R CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package test

import (
	"testing"
)

func TestNewKeys(t *testing.T) {
	keys, err := newKeys()
	MustSucceed(t, err)
	MustNotBeNil(t, keys)

	MustSucceed(t, keys.root.cert.CheckSignatureFrom(keys.root.cert))
	MustSucceed(t, keys.server.cert.CheckSignatureFrom(keys.root.cert))
	MustSucceed(t, keys.client.cert.CheckSignatureFrom(keys.root.cert))
	MustFail(t, keys.root.cert.CheckSignatureFrom(keys.client.cert))
}

func TestNewTLSConfig(t *testing.T) {

	cfg, err := NewTLSConfig(true)
	MustSucceed(t, err)
	MustNotBeNil(t, cfg)
}
