// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// +build darwin

// Package platform contains platform specific utilities.
package platform

import (
	"testing"

	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

func TestParsePlatformMap_EmptyString(t *testing.T) {
	platformInfoMap = map[string]string{}

	tmpFunc := execWithTimeout
	execWithTimeout = func(string, ...string) ([]byte, error) {
		return []byte(" \t"), nil
	}
	defer func() { execWithTimeout = tmpFunc }()

	logObj := logger.NewMockLog()
	prodVer, err := getPlatformDetail(logObj, "ProductVersion")
	assert.Equal(t, notAvailableMessage, prodVer)
	assert.NotNil(t, err)
}

func TestParsePlatformMap_QueryTwice(t *testing.T) {
	platformInfoMap = map[string]string{}
	queryCount := 0

	tmpFunc := execWithTimeout
	execWithTimeout = func(string, ...string) ([]byte, error) {
		queryCount += 1
		return []byte("\nProductVersion:\t10.15.8\ntestingsomething\n"), nil
	}
	defer func() { execWithTimeout = tmpFunc }()

	logObj := logger.NewMockLog()
	prodVer, err := getPlatformDetail(logObj, "ProductVersion")
	assert.Equal(t, "10.15.8", prodVer)
	assert.Nil(t, err)

	prodVer, err = getPlatformDetail(logObj, "ProductName")
	assert.Equal(t, notAvailableMessage, prodVer)
	assert.NotNil(t, err)
}

func TestParsePlatformMap(t *testing.T) {
	platformInfoMap = map[string]string{}

	tmpFunc := execWithTimeout
	queryCounter := 0
	execWithTimeout = func(string, ...string) ([]byte, error) {
		queryCounter += 1
		return []byte("ProductName:\tMac OS X\nProductVersion:\t10.15.7\nBuildVersion:\t19H524\n"), nil
	}
	defer func() { execWithTimeout = tmpFunc }()

	logObj := logger.NewMockLog()
	prodVer, err := getPlatformDetail(logObj, "ProductVersion")
	assert.Equal(t, "10.15.7", prodVer)
	assert.Nil(t, err)

	assert.Equal(t, 3, len(platformInfoMap))
	assert.Equal(t, "Mac OS X", platformInfoMap["ProductName"])
	assert.Equal(t, "10.15.7", platformInfoMap["ProductVersion"])
	assert.Equal(t, "19H524", platformInfoMap["BuildVersion"])

	prodName, err := getPlatformDetail(logObj, "ProductName")
	assert.Equal(t, 1, queryCounter)
	assert.Equal(t, "Mac OS X", prodName)
	assert.Nil(t, err)
}
