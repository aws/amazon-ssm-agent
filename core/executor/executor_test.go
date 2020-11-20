// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package executor

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {

}

func TestIsPidRunning(t *testing.T) {
	exec := NewProcessExecutor(log.NewMockLog())

	oldGetProcess := getProcess
	getProcess = func() ([]OsProcess, error) {
		return []OsProcess{
			{1, 2, "exe", "R"},
			{3, 4, "exe", "R"},
			{5, 6, "exe", "Z"},
		}, nil
	}
	defer func() { getProcess = oldGetProcess }()

	isRunning, err := exec.IsPidRunning(1)
	assert.True(t, isRunning)
	assert.Nil(t, err)

	isRunning, err = exec.IsPidRunning(2)
	assert.False(t, isRunning)
	assert.Nil(t, err)

	isRunning, err = exec.IsPidRunning(3)
	assert.True(t, isRunning)
	assert.Nil(t, err)

	isRunning, err = exec.IsPidRunning(5)
	assert.False(t, isRunning)
	assert.Nil(t, err)
}

func TestIsPidRunningButError(t *testing.T) {
	exec := NewProcessExecutor(log.NewMockLog())

	errMsg := "Some Error"

	oldGetProcess := getProcess
	getProcess = func() ([]OsProcess, error) {
		return []OsProcess{
			{1, 2, "exe", "R"},
			{3, 4, "exe", "R"},
			{5, 6, "exe", "Z"},
		}, fmt.Errorf(errMsg)
	}
	defer func() { getProcess = oldGetProcess }()

	isRunning, err := exec.IsPidRunning(1)
	assert.False(t, isRunning)
	assert.NotNil(t, err)
	assert.Equal(t, errMsg, fmt.Sprint(err))
}
