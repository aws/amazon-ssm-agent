// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package filemanager_mock has mock functions for filemanager package
package filemanager_mock

import (
	"github.com/stretchr/testify/mock"
)

type FileSystemMock struct {
	mock.Mock
}

func (fileMock FileSystemMock) MakeDirs(destinationDir string) (err error) {
	args := fileMock.Called(destinationDir)
	return args.Error(0)
}

// WriteFile writes the content in the file path provided
func (fileMock FileSystemMock) WriteFile(filename string, content string) error {
	args := fileMock.Called(filename, content)
	return args.Error(0)
}

// ReadFile reads the contents of file in path provided
func (fileMock FileSystemMock) ReadFile(filename string) (string, error) {
	args := fileMock.Called(filename)
	return args.Get(0).(string), args.Error(1)
}
