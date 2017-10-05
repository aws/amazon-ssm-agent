// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package file contains file gatherer.
package file

import (
	"errors"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/stretchr/testify/assert"
)

var testCmdOutPut = "[{\"CompanyName\": \"\", \"ProductName\": \"\", \"ProductVersion\": \"1.11.161\", \"ProductLanguage\": \"English (United States)\", \"Name\":\"aws.exe\", \"Size\":\"25088\",\"Description\":\"<startea53e72c>Universal Command Line Environment for AWS.<end71ebc6c5>\" ,\"FileVersion\":\"1.11.161\",\"InstalledDate\":\"2017-09-27T15:05:32Z\",\"LastAccessTime\":\"2017-09-29T19:23:25Z\",\"InstalledDir\":\"<startea53e72c>C:\\Program Files\\Amazon\\AWSCLI<end71ebc6c5>\",\"ModificationTime\":\"\"}]"

var fileData = []model.FileData{
	{
		Name:             "aws.exe",
		Size:             "25088",
		Description:      "Universal Command Line Environment for AWS.",
		FileVersion:      "1.11.161",
		InstalledDate:    "2017-09-27T15:05:32Z",
		ModificationTime: "",
		LastAccessTime:   "2017-09-29T19:23:25Z",
		ProductName:      "",
		InstalledDir:     "C:\\Program Files\\Amazon\\AWSCLI",
		CompanyName:      "",
		ProductVersion:   "1.11.161",
		ProductLanguage:  "English (United States)",
	},
}

var fileData2 = []model.FileData{
	{
		Name:             "aws.exe",
		Size:             "25088",
		Description:      "Universal Command Line Environment for AWS.",
		FileVersion:      "1.11.161",
		InstalledDate:    "2017-09-27T15:05:32Z",
		ModificationTime: "",
		LastAccessTime:   "2017-09-29T19:23:25Z",
		ProductName:      "",
		InstalledDir:     "C:\\Program Files\\Amazon\\AWSCLI",
		CompanyName:      "",
		ProductVersion:   "1.11.161",
		ProductLanguage:  "English (United States)",
	},
	{
		Name:             "aws.exe",
		Size:             "25088",
		Description:      "Universal Command Line Environment for AWS.",
		FileVersion:      "1.11.161",
		InstalledDate:    "2017-09-27T15:05:32Z",
		ModificationTime: "",
		LastAccessTime:   "2017-09-29T19:23:25Z",
		ProductName:      "",
		InstalledDir:     "C:\\Program Files\\Amazon\\AWSCLI",
		CompanyName:      "",
		ProductVersion:   "1.11.161",
		ProductLanguage:  "English (United States)",
	},
}

func MockWriteFile(path string, commands string) (err error) {
	return nil
}

func MockWriteFileError(path string, commands string) (err error) {
	return errors.New("Error")
}

func MockExecuteCommand(command string, args ...string) ([]byte, error) {
	return []byte(testCmdOutPut), nil
}

func MockExecuteCommandErr(command string, args ...string) ([]byte, error) {
	return nil, errors.New("Error")
}

func TestGetMetaUsingScript(t *testing.T) {
	mockContext := context.NewMockDefault()
	mockLog := mockContext.Log()
	writeFileText = MockWriteFile
	cmdExecutor = MockExecuteCommand
	startMarker = "<startea53e72c>"
	endMarker = "<end71ebc6c5>"

	path := []string{
		"C:\\Windows\\Program Files",
	}
	data, err := getMetaData(mockLog, path)

	assert.Nil(t, err)
	assert.Equal(t, fileData, data)
}

func TestGetMetaCmdError(t *testing.T) {
	mockContext := context.NewMockDefault()
	mockLog := mockContext.Log()
	writeFileText = MockWriteFileError
	cmdExecutor = MockExecuteCommandErr
	startMarker = "<startea53e72c>"
	endMarker = "<end71ebc6c5>"
	FileInfoBatchSize = 1

	path := []string{
		"C:\\Windows\\Program Files", "C:\\Windows\\Application",
	}
	data, err := getMetaData(mockLog, path)

	assert.NotNil(t, err)
	assert.Nil(t, data)
}
