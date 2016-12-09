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

// Package application contains a application gatherer.
package application

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/stretchr/testify/assert"
)

const (
	sampleData = `{"Name":"amazon-ssm-agent","Version":"1.2.0.0-1","Publisher":"Amazon.com, Inc. <ec2-ssm-feedback@amazon.com>","ApplicationType":"admin","Architecture":"amd64","Url":""},{"Name":"adduser","Version":"3.113+nmu3ubuntu3","Publisher":"Ubuntu Core Developers <ubuntu-devel-discuss@lists.ubuntu.com>","ApplicationType":"admin","Architecture":"all","Url":"http://alioth.debian.org/projects/adduser/"},`
)

func MockTestExecutorWithError(command string, args ...string) ([]byte, error) {
	var result []byte
	return result, fmt.Errorf("Random Error")
}

func MockTestExecutorWithoutError(command string, args ...string) ([]byte, error) {
	return []byte(sampleData), nil
}

var i = 0

// cmdExecutor returns error first (dpkg) and returns some valid result (rpm)
func MockTestExecutorWithAndWithoutError(command string, args ...string) ([]byte, error) {
	if i == 0 {
		i++
		return MockTestExecutorWithError(command, args...)
	} else {
		return MockTestExecutorWithoutError(command, args...)
	}
}

func TestConvertToApplicationData(t *testing.T) {
	data, err := ConvertToApplicationData(sampleData)

	assert.Nil(t, err, "Check conversion logic - since sample data in unit test is tied to implementation")
	assert.Equal(t, 2, len(data), "Given sample data must return 2 entries of application data")
}

func TestGetApplicationData(t *testing.T) {

	var data []model.ApplicationData
	var err error

	//setup
	mockContext := context.NewMockDefault()
	mockCommand := "RandomCommand"
	mockArgs := []string{
		"RandomArgument-1",
		"RandomArgument-2",
	}

	//testing with error
	cmdExecutor = MockTestExecutorWithError

	data, err = GetApplicationData(mockContext, mockCommand, mockArgs)

	assert.NotNil(t, err, "Error must be thrown when command execution fails")
	assert.Equal(t, 0, len(data), "When command execution fails - application dataset must be empty")

	//testing without error
	cmdExecutor = MockTestExecutorWithoutError

	data, err = GetApplicationData(mockContext, mockCommand, mockArgs)

	assert.Nil(t, err, "Error must not be thrown with MockTestExecutorWithoutError")
	assert.Equal(t, 2, len(data), "Given sample data must return 2 entries of application data")
}

func TestCollectApplicationData(t *testing.T) {
	mockContext := context.NewMockDefault()

	// both dpkg and rpm return result without error
	cmdExecutor = MockTestExecutorWithoutError
	data := CollectApplicationData(mockContext)
	assert.Equal(t, 2, len(data), "Given sample data must return 2 entries of application data")

	// both dpkg and rpm return errors
	cmdExecutor = MockTestExecutorWithError
	data = CollectApplicationData(mockContext)
	assert.Equal(t, 0, len(data), "When command execution fails - application dataset must be empty")

	// dpkg returns error and rpm return some result
	cmdExecutor = MockTestExecutorWithAndWithoutError
	data = CollectApplicationData(mockContext)
	assert.Equal(t, 2, len(data), "Given sample data must return 2 entries of application data")
}
