// Copyright 2024 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

//go:build freebsd || linux || netbsd || openbsd
// +build freebsd linux netbsd openbsd

// Package platform contains platform specific utilities.
package platform

import (
	"fmt"
	"os"
	"testing"

	logger "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/stretchr/testify/assert"
)

func TestVersion_PlatformWithBrackets(t *testing.T) {
	logMock := logger.NewMockLog()
	tmpFileExists := fileExists
	fileExists = func(filePath string) bool {
		return filePath == systemReleaseFile
	}
	tmpReadAllText := readAllText
	readAllText = func(filePath string) (text string, err error) {
		return "Red Hat Enterprise Linux Server release 6.10 (Santiago)", nil
	}
	defer func() {
		fileExists = tmpFileExists
		readAllText = tmpReadAllText
	}()
	name, version, err := getPlatformDetails(logMock)
	assert.Equal(t, "Red Hat Enterprise Linux Server", name)
	assert.Equal(t, "6.10", version)
	assert.Nil(t, err)
}

func TestVersion_PlatformWithOutBrackets(t *testing.T) {
	logMock := logger.NewMockLog()
	tmpFileExists := fileExists
	fileExists = func(filePath string) bool {
		return filePath == systemReleaseFile
	}
	tmpReadAllText := readAllText
	readAllText = func(_ string) (text string, err error) {
		return "Red Hat Enterprise Linux Server release 7", nil
	}
	defer func() {
		fileExists = tmpFileExists
		readAllText = tmpReadAllText
	}()
	name, version, err := getPlatformDetails(logMock)
	assert.Equal(t, "Red Hat Enterprise Linux Server", name)
	assert.Equal(t, "7", version)
	assert.Nil(t, err)
}

func TestPlatformNameAndVersion(t *testing.T) {
	ClearCache()
	tmpFileExists := fileExists
	fileExists = func(filePath string) bool {
		return filePath == systemReleaseFile
	}
	tmpReadAllText := readAllText
	readAllText = func(_ string) (text string, err error) {
		return "Red Hat Enterprise Linux Server release 6.78", nil
	}
	defer func() {
		fileExists = tmpFileExists
		readAllText = tmpReadAllText
	}()
	logMock := logger.NewMockLog()
	platformName, err := PlatformName(logMock)
	assert.Equal(t, "Red Hat Enterprise Linux Server", platformName)
	assert.Nil(t, err)
	platformVersion, err := PlatformVersion(logMock)
	assert.Equal(t, "6.78", platformVersion)
	assert.Nil(t, err)
}

func TestPlatformNameAndVersionWithError(t *testing.T) {
	ClearCache()
	tmpFileExists := fileExists
	fileExists = func(filePath string) bool {
		return filePath == systemReleaseFile
	}
	tmpReadAllText := readAllText
	readAllText = func(_ string) (text string, err error) {
		return "", fmt.Errorf("platform name and version error")
	}
	defer func() {
		fileExists = tmpFileExists
		readAllText = tmpReadAllText
	}()
	logMock := logger.NewMockLog()
	platformName, err := PlatformName(logMock)
	assert.Equal(t, notAvailableMessage, platformName)
	assert.NotNil(t, err)
	platformVersion, err := PlatformVersion(logMock)
	assert.Equal(t, notAvailableMessage, platformVersion)
	assert.NotNil(t, err)
}

func TestGetSystemInfo(t *testing.T) {
	ClearCache()
	tmpFileExists := fileExists
	fileExists = func(filePath string) bool {
		return true
	}
	testValue := "test_value"
	tmpReadAllText := readAllText
	readAllText = func(_ string) (text string, err error) {
		return testValue, nil
	}
	defer func() {
		fileExists = tmpFileExists
		readAllText = tmpReadAllText
	}()
	logMock := logger.NewMockLog()

	returnedValue, err := GetSystemInfo(logMock, "")
	assert.Equal(t, testValue, returnedValue)
	assert.Nil(t, err)
}

func TestGetSystemInfoCacheHit(t *testing.T) {
	ClearCache()
	cacheInitCount := 0
	tmpFileExists := fileExists
	fileExists = func(filePath string) bool {
		cacheInitCount++
		return true
	}
	tmpReadAllText := readAllText
	readAllText = func(_ string) (text string, err error) {
		return "test_value", nil
	}
	defer func() {
		fileExists = tmpFileExists
		readAllText = tmpReadAllText
	}()
	logMock := logger.NewMockLog()

	GetSystemInfo(logMock, XenUuidSystemInfoParamKey)
	GetSystemInfo(logMock, XenUuidSystemInfoParamKey)
	assert.Equal(t, 1, cacheInitCount)
}

func TestGetSystemInfoCacheMiss(t *testing.T) {
	ClearCache()
	cacheInitCount := 0
	tmpFileExists := fileExists
	fileExists = func(filePath string) bool {
		cacheInitCount++
		return true
	}
	tmpReadAllText := readAllText
	readAllText = func(_ string) (text string, err error) {
		return "test_value", nil
	}
	defer func() {
		fileExists = tmpFileExists
		readAllText = tmpReadAllText
	}()
	logMock := logger.NewMockLog()

	GetSystemInfo(logMock, XenUuidSystemInfoParamKey)
	GetSystemInfo(logMock, XenVersionSystemInfoParamKey)
	assert.Equal(t, 2, cacheInitCount)
}

func TestGetSystemInfoWithError(t *testing.T) {
	ClearCache()
	cacheInitCount := 0
	tmpFileExists := fileExists
	fileExists = func(filePath string) bool {
		cacheInitCount++
		return true
	}
	tmpReadAllText := readAllText
	readAllText = func(_ string) (text string, err error) {
		return "", fmt.Errorf("system info error")
	}
	defer func() {
		fileExists = tmpFileExists
		readAllText = tmpReadAllText
	}()
	logMock := logger.NewMockLog()

	uuid, err := GetSystemInfo(logMock, XenUuidSystemInfoParamKey)
	assert.Equal(t, "", uuid)
	assert.NotNil(t, err)
	//make sure we don't cache values with errors
	GetSystemInfo(logMock, XenUuidSystemInfoParamKey)
	assert.Equal(t, 2, cacheInitCount)
}

func TestGetSystemInfoWithNonExistingParam(t *testing.T) {
	ClearCache()
	cacheInitCount := 0
	tmpFileExists := fileExists
	fileExists = func(filePath string) bool {
		cacheInitCount++
		return false
	}
	defer func() { fileExists = tmpFileExists }()
	logMock := logger.NewMockLog()

	uuid, err := GetSystemInfo(logMock, XenUuidSystemInfoParamKey)
	assert.Equal(t, "", uuid)
	assert.ErrorIs(t, err, ErrFileNotFound)
	//make sure we don't cache values for non existing params
	GetSystemInfo(logMock, XenUuidSystemInfoParamKey)
	assert.Equal(t, 2, cacheInitCount)
}

func TestSetOOMScoreAdjust(t *testing.T) {
	// Save original functions
	tmpFileExists := fileExists
	tmpWriteFile := writeFile
	defer func() {
		fileExists = tmpFileExists
		writeFile = tmpWriteFile
	}()

	// Mock file exists
	fileExists = func(filePath string) bool {
		return filePath == "/proc/self/oom_score_adj"
	}

	// Mock successful write
	writeCalled := false
	var writtenData []byte
	writeFile = func(name string, data []byte, perm os.FileMode) error {
		writeCalled = true
		writtenData = data
		return nil
	}

	logMock := logger.NewMockLog()
	SetOOMScoreAdjust(logMock)

	assert.True(t, writeCalled, "WriteFile should have been called")
	assert.Equal(t, "-1000", string(writtenData), "Should write -1000")
}

func TestSetOOMScoreAdjustError(t *testing.T) {
	// Save original functions
	tmpFileExists := fileExists
	tmpWriteFile := writeFile
	defer func() {
		fileExists = tmpFileExists
		writeFile = tmpWriteFile
	}()

	fileExists = func(filePath string) bool {
		return filePath == "/proc/self/oom_score_adj"
	}

	writeFile = func(name string, data []byte, perm os.FileMode) error {
		return fmt.Errorf("permission denied")
	}

	logMock := logger.NewMockLog()
	SetOOMScoreAdjust(logMock)

	assert.NotNil(t, logMock)
}

func TestSetOOMScoreAdjustNotSupported(t *testing.T) {
	tmpFileExists := fileExists
	tmpWriteFile := writeFile
	defer func() {
		fileExists = tmpFileExists
		writeFile = tmpWriteFile
	}()

	fileExists = func(filePath string) bool {
		return false
	}

	writeCalled := false
	writeFile = func(name string, data []byte, perm os.FileMode) error {
		writeCalled = true
		return nil
	}

	logMock := logger.NewMockLog()
	SetOOMScoreAdjust(logMock)

	assert.False(t, writeCalled, "WriteFile should not have been called when file doesn't exist")
}
