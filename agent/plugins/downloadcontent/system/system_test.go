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

// Package system have all the file related dependencies used by the copy package
package system

import (
	filemock "github.com/aws/amazon-ssm-agent/agent/fileutil/filemanager/mock"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"

	"errors"
	"fmt"
	"testing"
)

var logMock = log.NewMockLog()

func TestSaveFileContent_MakeDirFail(t *testing.T) {
	var fileMock filemock.FileSystemMock
	destination := "destinationDir/file.ps"
	contents := "contents"
	//resourcePath := "resourcePath"

	fileMock.On("MakeDirs", "destinationDir").Return(fmt.Errorf("failed to create directory "))

	err := SaveFileContent(logMock, fileMock, destination, contents)

	assert.Error(t, err, "Must return error")
}

func TestSaveFileContent_WriteFileFail(t *testing.T) {
	var fileMock filemock.FileSystemMock

	destinationDir := "destinationDir/filepath.ps"
	contents := "contents"
	//resourcePath := "resourcePath"

	fileMock.On("MakeDirs", "destinationDir").Return(nil).Once()
	fileMock.On("WriteFile", destinationDir, contents).Return(fmt.Errorf("failed to create directory "))

	err := SaveFileContent(logMock, fileMock, destinationDir, contents)

	assert.Error(t, err, "Must return error")
}

func TestSaveFileContent_Pass(t *testing.T) {
	fileMock := filemock.FileSystemMock{}
	destination := "destinationDir/filename.py"
	contents := "contents"
	//resourcePath := "resourcePath"

	fileMock.On("MakeDirs", "destinationDir").Return(nil).Once()
	fileMock.On("WriteFile", destination, contents).Return(nil).Once()

	err := SaveFileContent(logMock, fileMock, destination, contents)

	assert.NoError(t, err)
}

func TestRenameFile(t *testing.T) {
	fileMock := filemock.FileSystemMock{}
	sourceName := "destination/oldFileName.ext"
	newFileName := "newFileName.ext"

	fileMock.On("MoveAndRenameFile", "destination", "oldFileName.ext", "destination", "newFileName.ext").Return(true, nil)

	err := RenameFile(logMock, fileMock, sourceName, newFileName)

	assert.NoError(t, err)
	fileMock.AssertExpectations(t)
}

func TestRenameFile_Error(t *testing.T) {
	fileMock := filemock.FileSystemMock{}
	sourceName := "destination/oldFileName.ext"
	newFileName := "newFileName.ext"

	fileMock.On("MoveAndRenameFile", "destination", "oldFileName.ext", "destination", "newFileName.ext").Return(true, errors.New("There was an error"))

	err := RenameFile(logMock, fileMock, sourceName, newFileName)

	assert.Error(t, err)
	assert.Equal(t, "There was an error", err.Error())
	fileMock.AssertExpectations(t)

}
