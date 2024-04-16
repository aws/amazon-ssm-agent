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

//go:build windows
// +build windows

package utility

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_IsRunningElevatedPermissions_Success(t *testing.T) {
	expectedValue := defaultCommandTimeOut
	executePowershellCommandWithTimeoutFunc = func(timeout time.Duration, command string) (string, error) {
		assert.Equal(t, timeout.Seconds(), expectedValue.Seconds())
		return "True", nil
	}
	err := IsRunningElevatedPermissions()
	assert.Nil(t, err)
}

func Test_IsRunningElevatedPermissions_Failure(t *testing.T) {
	expectedValue := defaultCommandTimeOut
	executePowershellCommandWithTimeoutFunc = func(timeout time.Duration, command string) (string, error) {
		assert.Equal(t, timeout.Seconds(), expectedValue.Seconds())
		return "False", nil
	}
	err := IsRunningElevatedPermissions()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "needs to be executed by administrator")
}

func Test_IsRunningElevatedPermissions_Failure_OtherErrors(t *testing.T) {
	expectedValue := defaultCommandTimeOut
	executePowershellCommandWithTimeoutFunc = func(timeout time.Duration, command string) (string, error) {
		assert.Equal(t, timeout.Seconds(), expectedValue.Seconds())
		return "True", fmt.Errorf("dss")
	}
	err := IsRunningElevatedPermissions()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "failed to check permissions")
}

func Test_IsRunningElevatedPermissions_Failure_OtherOutput(t *testing.T) {
	expectedValue := defaultCommandTimeOut
	executePowershellCommandWithTimeoutFunc = func(timeout time.Duration, command string) (string, error) {
		assert.Equal(t, timeout.Seconds(), expectedValue.Seconds())
		return "DummyReturnValue", nil
	}
	err := IsRunningElevatedPermissions()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "DummyReturnValue")
}
