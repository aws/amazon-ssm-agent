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

// Package fileutil_mock has mock functions for filemanager package
package fileutil_mock

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

func (fileMock FileSystemMock) WriteFile(filename string, content string) error {
	args := fileMock.Called(filename, content)
	return args.Error(0)
}

func (fileMock FileSystemMock) ReadFile(filename string) (string, error) {
	args := fileMock.Called(filename)
	return args.Get(0).(string), args.Error(1)
}

func (fileMock FileSystemMock) MoveAndRenameFile(sourcePath, sourceName, destPath, destName string) (result bool, err error) {
	args := fileMock.Called(sourcePath, sourceName, destPath, destName)
	return args.Bool(0), args.Error(1)
}

func (fileMock FileSystemMock) DeleteDirectory(fileName string) (err error) {
	args := fileMock.Called(fileName)
	return args.Error(0)
}

func (fileMock FileSystemMock) DeleteFile(fileName string) (err error) {
	args := fileMock.Called(fileName)
	return args.Error(0)
}

func (fileMock FileSystemMock) Exists(root string) bool {
	args := fileMock.Called(root)
	return args.Bool(0)
}

func (fileMock FileSystemMock) IsDirectory(root string) bool {
	args := fileMock.Called(root)
	return args.Bool(0)
}

func (fileMock FileSystemMock) AppendToFile(fileDirectory string, filename string, content string) (filePath string, err error) {
	args := fileMock.Called(fileDirectory, filename, content)
	return args.Get(0).(string), args.Error(1)
}
