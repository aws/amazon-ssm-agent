// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package fingerprint

import (
	"fmt"

	"github.com/stretchr/testify/mock"
)

type fpFsVaultMock struct {
	mock.Mock
}

func ByteArrayArg(args mock.Arguments, index int) []byte {
	var s []byte
	var ok bool
	var rawArg = args.Get(index)
	if rawArg == nil {
		return nil
	}

	if s, ok = rawArg.([]byte); !ok {
		panic(fmt.Sprintf("assert: arguments: ByteArrayArg(%d) failed because object wasn't correct type: %v", index, args.Get(index)))
	}

	return s
}

func (m *fpFsVaultMock) Retrieve(manifestFileNamePrefix string, key string) ([]byte, error) {
	args := m.Called(key)
	return ByteArrayArg(args, 0), args.Error(1)
}

func (m *fpFsVaultMock) Store(manifestFileNamePrefix string, key string, data []byte) error {
	args := m.Called(key, data)
	return args.Error(0)
}
