// Copyright 2023 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package registermanager contains functions related to register
package registermanager

import (
	"fmt"
	"testing"
	"time"

	mhMock "github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRegisterAgent_RegisterWithTags_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}

	rm := registerManager{helperMock, "SomeBinPath"}

	helperMock.On("RunCommand", mock.Anything, mock.Anything, mock.Anything, mock.Anything, "SomeRegion", mock.Anything, "SomeRole", mock.Anything, "MyTags").Return("", nil).Once()
	input := &RegisterAgentInputModel{
		Region: "SomeRegion",
		Role:   "SomeRole",
		Tags:   "MyTags",
	}
	err := rm.RegisterAgent(input)
	assert.NoError(t, err)
}

func TestRegisterAgent_Success_Onprem(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}

	rm := registerManager{helperMock, "SomeBinPath"}
	helperMock.On("RunCommandWithCustomTimeout", 60*time.Second, "SomeBinPath", "-register", mock.Anything, mock.Anything, "Region", "-code", "test1", "-id", "test2").Return("", nil).Once()
	input := &RegisterAgentInputModel{
		Region:         "Region",
		ActivationCode: "test1",
		ActivationId:   "test2",
	}
	err := rm.RegisterAgent(input)
	assert.NoError(t, err)
}

func TestRegisterAgent_Failure_Onprem(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}

	rm := registerManager{helperMock, ""}
	helperMock.On("RunCommandWithCustomTimeout", 60*time.Second, "", "-register", mock.Anything, mock.Anything, "Region", "-code", "test1", "-id", "test2").Return("", nil).Once()
	input := &RegisterAgentInputModel{
		Region:         "Region",
		ActivationCode: "test1",
		ActivationId:   "test2",
	}
	err := rm.RegisterAgent(input)
	assert.Error(t, err)
}

func TestRegisterAgent_RegisterWithoutTags_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}

	rm := registerManager{helperMock, "SomeBinPath"}

	helperMock.On("RunCommand", mock.Anything, mock.Anything, mock.Anything, mock.Anything, "SomeRegion", mock.Anything, "SomeRole").Return("", nil).Once()
	input := &RegisterAgentInputModel{
		Region: "SomeRegion",
		Role:   "SomeRole",
		Tags:   "",
	}
	err := rm.RegisterAgent(input)
	assert.NoError(t, err)
}

func TestRegisterAgent_InvalidActionIdActionCode_Failure(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}

	rm := registerManager{helperMock, "SomeBinPath"}
	input := &RegisterAgentInputModel{
		Region:         "SomeRegion",
		ActivationId:   "SomeActivationId",
		ActivationCode: "",
	}
	err := rm.RegisterAgent(input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed with empty activation code")

	input = &RegisterAgentInputModel{
		Region:         "SomeRegion",
		ActivationId:   "",
		ActivationCode: "SomeActivationCode",
	}
	err = rm.RegisterAgent(input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed with empty activation id")
}

func TestRegisterAgent_ValidActionIdActionCode_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}

	rm := registerManager{helperMock, "SomeBinPath"}
	helperMock.On("RunCommandWithCustomTimeout", 60*time.Second, mock.Anything, mock.Anything, mock.Anything,
		"-region", "SomeRegion",
		"-code", "SomeActivationCode",
		"-id", "SomeActivationId").Return("", nil).Once()
	input := &RegisterAgentInputModel{
		Region:         "SomeRegion",
		ActivationId:   "SomeActivationId",
		ActivationCode: "SomeActivationCode",
	}
	err := rm.RegisterAgent(input)
	assert.NoError(t, err)
}

func TestRegisterAgent_Failure(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}

	rm := registerManager{helperMock, "SomeBinPath"}

	// Exit code error
	helperMock.On("RunCommand", mock.Anything, mock.Anything, mock.Anything, mock.Anything, "SomeRegion", mock.Anything, "SomeRole").Return("SomeOutput", fmt.Errorf("SomeError")).Once()
	helperMock.On("IsExitCodeError", mock.Anything).Return(true).Once()
	input := &RegisterAgentInputModel{
		Region: "SomeRegion",
		Role:   "SomeRole",
		Tags:   "",
	}
	err := rm.RegisterAgent(input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed with output")

	// Timeout error
	helperMock.On("RunCommand", mock.Anything, mock.Anything, mock.Anything, mock.Anything, "SomeRegion", mock.Anything, "SomeRole").Return("SomeOutput", fmt.Errorf("SomeError")).Once()
	helperMock.On("IsExitCodeError", mock.Anything).Return(false).Once()
	helperMock.On("IsTimeoutError", mock.Anything).Return(true).Once()
	err = rm.RegisterAgent(input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timed out with output")

	// unexpected error
	helperMock.On("RunCommand", mock.Anything, mock.Anything, mock.Anything, mock.Anything, "SomeRegion", mock.Anything, "SomeRole").Return("SomeOutput", fmt.Errorf("SomeError")).Once()
	helperMock.On("IsExitCodeError", mock.Anything).Return(false).Once()
	helperMock.On("IsTimeoutError", mock.Anything).Return(false).Once()
	err = rm.RegisterAgent(input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected error")
}

func TestGetAgentBinaryPath_Success(t *testing.T) {
	utilFileExists = func(filePath string) (bool, error) {
		return true, nil
	}
	path := getAgentBinaryPath()
	assert.NotEmpty(t, path)
}

func TestGetAgentBinaryPath_Failure(t *testing.T) {
	utilFileExists = func(filePath string) (bool, error) {
		return false, nil
	}
	path := getAgentBinaryPath()
	assert.Empty(t, path)
}
