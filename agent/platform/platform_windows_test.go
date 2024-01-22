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

//go:build windows
// +build windows

// Package platform contains platform specific utilities.
package platform

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	logger "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/stretchr/testify/assert"
)

func TestVersion_Positive(t *testing.T) {
	logMock := logger.NewMockLog()
	getPlatformVersionRef = func(log log.T) (value string, err error) {
		return "6.2323.23", nil
	}

	isWin2012, err := isPlatformWindowsServer2012OrEarlier(logMock)
	assert.True(t, isWin2012, "Should return true")
	assert.Nil(t, err)

	getPlatformVersionRef = func(log log.T) (value string, err error) {
		return "20.2323.23", nil
	}
	isWin2012, err = isPlatformWindowsServer2012OrEarlier(logMock)
	assert.False(t, isWin2012, "Should return false")
	assert.Nil(t, err)
}

func TestVersion_Negative(t *testing.T) {
	logMock := logger.NewMockLog()
	getPlatformVersionRef = func(log log.T) (value string, err error) {
		return "0.022", nil
	}

	isWin2012, err := isPlatformWindowsServer2012OrEarlier(logMock)
	assert.True(t, isWin2012, "Should return true")
	assert.Nil(t, err)

	getPlatformVersionRef = func(log log.T) (value string, err error) {
		return "dsdsds23323", nil
	}
	isWin2012, err = isPlatformWindowsServer2012OrEarlier(logMock)
	assert.False(t, isWin2012, "Should return false")
	assert.NotNil(t, err)

	getPlatformVersionRef = func(log log.T) (value string, err error) {
		return "", nil
	}
	isWin2012, err = isPlatformWindowsServer2012OrEarlier(logMock)
	assert.False(t, isWin2012, "Should return false")
	assert.NotNil(t, err)

	getPlatformVersionRef = func(log log.T) (value string, err error) {
		return "", fmt.Errorf("test1")
	}
	isWin2012, err = isPlatformWindowsServer2012OrEarlier(logMock)
	assert.False(t, isWin2012, "Should return false")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "test1")
}
