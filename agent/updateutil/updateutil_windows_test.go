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

//go:build windows
// +build windows

package updateutil

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	logPkg "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	"github.com/stretchr/testify/assert"
)

func TestGetPluginState(t *testing.T) {
	exponentialBackOff, err := backoffConfigExponential(200*time.Millisecond, 1)
	assert.Nil(t, err)
	fileExists = func(filePath string) bool {
		return false
	}
	var updatePluginState UpdatePluginRunState
	unmarshallFile = func(filePath string, dest interface{}) (err error) {
		return fmt.Errorf("err1")
	}
	_, err = getPluginState("test1", exponentialBackOff)
	assert.Nil(t, err)
	assert.Equal(t, "", updatePluginState.CommandId)

	fileExists = func(filePath string) bool {
		return true
	}
	unmarshallFile = func(filePath string, dest interface{}) (err error) {
		updatePluginState.CommandId = "cmd1"
		return fmt.Errorf("err1")
	}

	_, err = getPluginState("test1", exponentialBackOff)
	assert.Equal(t, "cmd1", updatePluginState.CommandId)
	assert.NotNil(t, err)

	fileExists = func(filePath string) bool {
		return true
	}
	unmarshallFile = func(filePath string, dest interface{}) (err error) {
		updatePluginState.CommandId = "cmd2"
		return nil
	}
	_, err = getPluginState("test1", exponentialBackOff)
	assert.Equal(t, "cmd2", updatePluginState.CommandId)
	assert.Nil(t, err)
}

func TestSavePluginState(t *testing.T) {
	exponentialBackOff, err := backoffConfigExponential(200*time.Millisecond, 1)
	assert.Nil(t, err)
	var tempState UpdatePluginRunState
	tempState.CommandId = "Sample"
	tempStateContent, _ := jsonutil.Marshal(tempState)
	outputContent := ""

	// Success Case
	fileWrite = func(absolutePath, content string, perm os.FileMode) (result bool, err error) {
		outputContent = content
		return true, nil
	}
	err = savePluginState("path1", tempState, exponentialBackOff)
	assert.Equal(t, jsonutil.Indent(tempStateContent), outputContent)
	assert.Nil(t, err)

	// Failure case
	fileWrite = func(absolutePath, content string, perm os.FileMode) (result bool, err error) {
		outputContent = ""
		return false, fmt.Errorf("err1")
	}
	err = savePluginState("path1", tempState, exponentialBackOff)
	assert.Equal(t, "", outputContent)
	assert.NotNil(t, err)
}

func TestDeleteFile(t *testing.T) {
	exponentialBackOff, err := backoffConfigExponential(200*time.Millisecond, 1)
	assert.Nil(t, err)
	visited := true

	deleteFile = func(filepath string) (err error) {
		visited = false
		return nil
	}
	fileExists = func(filePath string) bool {
		return true
	}
	logMock := log.NewMockLog()
	removePluginState(logMock, "root1", exponentialBackOff)
	assert.False(t, visited)

	visited = true
	deleteFile = func(filepath string) (err error) {
		visited = false
		return nil
	}
	fileExists = func(filePath string) bool {
		return false
	}
	removePluginState(logMock, "root1", exponentialBackOff)
	assert.True(t, visited)
}

func TestVerifyVersion_Success(t *testing.T) {
	expectedVersion := "3.2.0.0"
	getVersionThroughRegistryKeyRef = func(log logPkg.T) string {
		return expectedVersion
	}
	logMock := log.NewMockLog()
	errCode := verifyVersion(logMock, expectedVersion)
	assert.Equal(t, updateconstants.ErrorCode(""), errCode)
}

func TestVerifyVersion_Failed_Registry(t *testing.T) {
	expectedVersion := "3.2.0.0"
	getVersionThroughRegistryKeyRef = func(log logPkg.T) string {
		return ""
	}
	logMock := log.NewMockLog()
	errCode := verifyVersion(logMock, expectedVersion)
	assert.Equal(t, updateconstants.ErrorInstTargetVersionNotFoundViaReg, errCode)
}

func TestVerifyVersion_Failed_Both(t *testing.T) {
	getVersionThroughRegistryKeyRef = func(log logPkg.T) string {
		return ""
	}
	logMock := log.NewMockLog()
	expectedVersion := "3.2.0.0"
	errCode := verifyVersion(logMock, expectedVersion)
	assert.Equal(t, updateconstants.ErrorInstTargetVersionNotFoundViaReg, errCode)
}
