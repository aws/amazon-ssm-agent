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
	"github.com/aws/amazon-ssm-agent/agent/inventory/model"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

const (
	sampleData        = `{"Name":"amazon-ssm-agent","Version":"1.2.0.0-1","Publisher":"Amazon.com, Inc. <ec2-ssm-feedback@amazon.com>","ApplicationType":"admin","Architecture":"amd64","Url":""},{"Name":"adduser","Version":"3.113+nmu3ubuntu3","Publisher":"Ubuntu Core Developers <ubuntu-devel-discuss@lists.ubuntu.com>","ApplicationType":"admin","Architecture":"all","Url":"http://alioth.debian.org/projects/adduser/"},`
	ubuntuOSName      = "Ubuntu"
	amzLinuxOSName    = "Amazon Linux Ami"
	unsupportedOSName = "Unsupported OS"
)

func MockTestExecutorWithError(command string, args ...string) ([]byte, error) {
	var result []byte
	return result, fmt.Errorf("Random Error")
}

func MockTestExecutorWithoutError(command string, args ...string) ([]byte, error) {
	return []byte(sampleData), nil
}

func MockPlatformInfoProviderReturningError(log log.T) (name string, err error) {
	return "", fmt.Errorf("Random Error")
}

func MockPlatformInfoProviderReturningAmazonLinux(log log.T) (name string, err error) {
	return amzLinuxOSName, nil
}

func MockPlatformInfoProviderReturningUbuntu(log log.T) (name string, err error) {
	return ubuntuOSName, nil
}

func MockPlatformInfoProviderReturningUnsupportedOS(log log.T) (name string, err error) {
	return unsupportedOSName, nil
}

func TestConvertToApplicationData(t *testing.T) {
	data, err := ConvertToApplicationData(sampleData)

	assert.Nil(t, err, "Check conversion logic - since sample data in unit test is tied to implementation")
	assert.Equal(t, 2, len(data), "Given sample data must return 2 entries of application data")
}

func TestGetApplicationData(t *testing.T) {

	var data []inventory.ApplicationData
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

	var data []inventory.ApplicationData

	//setup
	c := context.NewMockDefault()

	//testing when platform info provider throws error
	osInfoProvider = MockPlatformInfoProviderReturningError
	data = CollectApplicationData(c)

	assert.Equal(t, 0, len(data), "Application dataset must be empty - when platform provider throws error")

	//testing for unsupported OS
	osInfoProvider = MockPlatformInfoProviderReturningUnsupportedOS
	data = CollectApplicationData(c)

	assert.Equal(t, 0, len(data), "For unsupported OS - application dataset must be empty")

	//testing for amazon linux

	//testing when command executor doesn't return any error
	osInfoProvider = MockPlatformInfoProviderReturningAmazonLinux
	cmdExecutor = MockTestExecutorWithoutError

	data = CollectApplicationData(c)
	assert.Equal(t, 2, len(data), "Given sample data must return 2 entries of application data")

	//testing when command executor return error
	osInfoProvider = MockPlatformInfoProviderReturningAmazonLinux
	cmdExecutor = MockTestExecutorWithError

	data = CollectApplicationData(c)
	assert.Equal(t, 0, len(data), "Application dataset must be empty - if command fails for any OS")

	//testing for Ubuntu

	//testing when command executor doesn't return any error
	osInfoProvider = MockPlatformInfoProviderReturningUbuntu
	cmdExecutor = MockTestExecutorWithoutError

	data = CollectApplicationData(c)
	assert.Equal(t, 2, len(data), "Given sample data must return 2 entries of application data")

	//testing when command executor return error
	osInfoProvider = MockPlatformInfoProviderReturningUbuntu
	cmdExecutor = MockTestExecutorWithError

	data = CollectApplicationData(c)
	assert.Equal(t, 0, len(data), "Application dataset must be empty - if command fails for any OS")

}
