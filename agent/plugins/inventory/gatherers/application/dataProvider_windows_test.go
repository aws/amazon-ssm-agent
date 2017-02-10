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

// +build windows

package application

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/stretchr/testify/assert"
)

const (
	sampleData   = `[{"Name":"Notepad++","Version":"6.9.2","Publisher":"Notepad++ Team","InstalledTime":null},{"Name":"AWS Tools for Windows","Version":"3.9.344.0","Publisher":"Amazon Web Services Developer Relations","InstalledTime":"20160512"},{"Name":"EC2ConfigService","Version":"3.16.930.0","Publisher":"Amazon Web Services","InstalledTime":null}]`
	mockArch     = "randomArch"
	randomString = "blahblah"
)

func MockTestExecutorWithError(command string, args ...string) ([]byte, error) {
	var result []byte
	return result, fmt.Errorf("Random Error")
}

func MockTestExecutorWithoutError(command string, args ...string) ([]byte, error) {
	return []byte(sampleData), nil
}

func MockTestExecutorWithConvertToApplicationDataReturningRandomString(command string, args ...string) ([]byte, error) {
	return []byte(randomString), nil
}

func TestConvertToApplicationData(t *testing.T) {

	var data []model.ApplicationData
	var err error

	data, err = convertToApplicationData(sampleData, mockArch)

	assert.Nil(t, err, "Error is not expected for processing sample data - %v", sampleData)
	assert.Equal(t, 3, len(data))
	assert.Equal(t, mockArch, data[0].Architecture, "Architecture must be - %v", mockArch)
}

func TestExecutePowershellCommands(t *testing.T) {

	var data []model.ApplicationData
	c := context.NewMockDefault()
	mockCmd := "RandomCommand"
	mockArgs := "RandomCommandArgs"

	//testing command executor without errors
	cmdExecutor = MockTestExecutorWithoutError
	data = executePowershellCommands(c, mockCmd, mockArgs, mockArch)

	assert.Equal(t, 3, len(data), "There must be 3 applications for given sample data - %v", sampleData)

	//testing command executor with errors
	cmdExecutor = MockTestExecutorWithError
	data = executePowershellCommands(c, mockCmd, mockArgs, mockArch)

	assert.Equal(t, 0, len(data), "On encountering error - application dataset must be empty")

	//testing command executor with ConvertToApplicationData throwing errors
	cmdExecutor = MockTestExecutorWithConvertToApplicationDataReturningRandomString
	data = executePowershellCommands(c, mockCmd, mockArgs, mockArch)

	assert.Equal(t, 0, len(data), "On encountering error during json conversion - application dataset must be empty")
}

func TestCollectApplicationData(t *testing.T) {

	var data []model.ApplicationData
	c := context.NewMockDefault()

	//testing command executor without errors
	cmdExecutor = MockTestExecutorWithoutError
	data = collectPlatformDependentApplicationData(c)

	assert.Equal(t, 6, data, "MockExecutor will be called 2 times hence total entries must be 6")

	//testing command executor with errors
	cmdExecutor = MockTestExecutorWithError
	data = collectPlatformDependentApplicationData(c)

	assert.Equal(t, 0, data, "If MockExecutor throws error, application dataset must be empty")
}
