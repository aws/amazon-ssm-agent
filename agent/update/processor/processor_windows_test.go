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

// Package processor contains the methods for update ssm agent.
// It also provides methods for sendReply and updateInstanceInfo
package processor

import (
	"fmt"
	"path/filepath"
	"testing"

	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/stretchr/testify/assert"
)

func TestUpdateTempEnv_Success(t *testing.T) {
	var logger = logmocks.NewMockLog()
	testDir := "test1"
	fileMakeDirs = func(destinationDir string) (err error) {
		return nil
	}
	osEnviron = func() []string {
		return []string{"TMP=testTemp1", "TEMP=testTemp2"}
	}
	output := getCommandEnv(logger, testDir)
	tempPath := filepath.Join(testDir, installationDirectory)
	expectedOutput := []string{"TMP=" + tempPath, "TEMP=" + tempPath}
	assert.Equal(t, expectedOutput[0], output[0])
	assert.Equal(t, expectedOutput[1], output[1])
}

func TestUpdateTempEnv_WithoutDefault_Success(t *testing.T) {
	var logger = logmocks.NewMockLog()
	testDir := "test1"
	fileMakeDirs = func(destinationDir string) (err error) {
		return nil
	}
	osEnviron = func() []string {
		return []string{"TestEnv1=test1"}
	}
	output := getCommandEnv(logger, testDir)
	tempPath := filepath.Join(testDir, installationDirectory)
	expectedOutput := []string{"TMP=" + tempPath, "TEMP=" + tempPath}
	assert.Equal(t, expectedOutput[0], output[1])
	assert.Equal(t, expectedOutput[1], output[2])
}

func TestUpdateTempEnv_Failed(t *testing.T) {
	var logger = logmocks.NewMockLog()
	testDir := "test1"
	fileMakeDirs = func(destinationDir string) (err error) {
		return fmt.Errorf("err1")
	}
	osEnviron = func() []string {
		return []string{"TMP=testTemp1", "TEMP=testTemp2"}
	}
	output := getCommandEnv(logger, testDir)
	assert.Equal(t, len(output), 0)
}
