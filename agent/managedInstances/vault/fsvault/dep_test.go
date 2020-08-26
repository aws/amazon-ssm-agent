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

package fsvault

import (
	"fmt"

	"github.com/stretchr/testify/mock"
)

type fsvFileSystemMock struct {
	mock.Mock
}

func ByteArrayArg(args mock.Arguments, index int) []byte {
	var s []byte
	var ok bool
	if s, ok = args.Get(index).([]byte); !ok {
		panic(fmt.Sprintf("assert: arguments: ByteArrayArg(%d) failed because object wasn't correct type: %v", index, args.Get(index)))
	}
	return s
}

func (m *fsvFileSystemMock) Exists(path string) bool {
	args := m.Called(path)
	return args.Bool(0)
}

func (m *fsvFileSystemMock) MakeDirs(path string) error {
	args := m.Called(path)
	return args.Error(0)
}

func (m *fsvFileSystemMock) RecursivelyHarden(path string) error {
	args := m.Called(path)
	return args.Error(0)
}

func (m *fsvFileSystemMock) ReadFile(path string) ([]byte, error) {
	args := m.Called(path)
	return ByteArrayArg(args, 0), args.Error(1)
}

func (m *fsvFileSystemMock) Remove(path string) error {
	args := m.Called(path)
	return args.Error(0)
}

func (m *fsvFileSystemMock) HardenedWriteFile(path string, data []byte) error {
	args := m.Called(path, data)
	return args.Error(0)
}

type fsvJsonHandlerMock struct {
	mock.Mock
}

func (m *fsvJsonHandlerMock) Marshal(v interface{}) ([]byte, error) {
	args := m.Called(v)
	return ByteArrayArg(args, 0), args.Error(1)
}

func (m *fsvJsonHandlerMock) Unmarshal(data []byte, v interface{}) error {
	args := m.Called(data, v)
	return args.Error(0)
}
