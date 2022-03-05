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

	MustSucceed(t, keys.Root.cert.CheckSignatureFrom(keys.Root.cert))
	MustSucceed(t, keys.Server.cert.CheckSignatureFrom(keys.Root.cert))
	MustSucceed(t, keys.Client.cert.CheckSignatureFrom(keys.Root.cert))
	MustFail(t, keys.Root.cert.CheckSignatureFrom(keys.Client.cert))
}

func TestNewTLSConfig(t *testing.T) {
	s, c, k, err := NewTLSConfig()
	MustSucceed(t, err)
	MustNotBeNil(t, s)
	MustNotBeNil(t, c)
	MustNotBeNil(t, k)
	MustBeTrue(t, len(c.RootCAs.Subjects()) != 0)
	MustBeFalse(t, c.InsecureSkipVerify)
	MustBeFalse(t, s.InsecureSkipVerify)
}
