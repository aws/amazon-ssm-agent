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

package registermanager

import (
	"fmt"
	"testing"

	mhMock "github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRegisterAgent_RegisterWithTags_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}

	rm := registerManager{helperMock, "SomeBinPath"}

	helperMock.On("RunCommand", mock.Anything, mock.Anything, mock.Anything, mock.Anything, "SomeRegion", mock.Anything, "SomeRole", mock.Anything, "MyTags").Return("", nil).Once()
	err := rm.RegisterAgent("SomeRegion", "SomeRole", "MyTags")
	assert.NoError(t, err)
}

func TestRegisterAgent_RegisterWithoutTags_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}

	rm := registerManager{helperMock, "SomeBinPath"}

	helperMock.On("RunCommand", mock.Anything, mock.Anything, mock.Anything, mock.Anything, "SomeRegion", mock.Anything, "SomeRole").Return("", nil).Once()
	err := rm.RegisterAgent("SomeRegion", "SomeRole", "")
	assert.NoError(t, err)
}

func TestRegisterAgent_Failure(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}

	rm := registerManager{helperMock, "SomeBinPath"}

	// Exit code error
	helperMock.On("RunCommand", mock.Anything, mock.Anything, mock.Anything, mock.Anything, "SomeRegion", mock.Anything, "SomeRole").Return("SomeOutput", fmt.Errorf("SomeError")).Once()
	helperMock.On("IsExitCodeError", mock.Anything).Return(true).Once()
	err := rm.RegisterAgent("SomeRegion", "SomeRole", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed with output")

	// Timeout error
	helperMock.On("RunCommand", mock.Anything, mock.Anything, mock.Anything, mock.Anything, "SomeRegion", mock.Anything, "SomeRole").Return("SomeOutput", fmt.Errorf("SomeError")).Once()
	helperMock.On("IsExitCodeError", mock.Anything).Return(false).Once()
	helperMock.On("IsTimeoutError", mock.Anything).Return(true).Once()
	err = rm.RegisterAgent("SomeRegion", "SomeRole", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timed out with output")

	// unexpected error
	helperMock.On("RunCommand", mock.Anything, mock.Anything, mock.Anything, mock.Anything, "SomeRegion", mock.Anything, "SomeRole").Return("SomeOutput", fmt.Errorf("SomeError")).Once()
	helperMock.On("IsExitCodeError", mock.Anything).Return(false).Once()
	helperMock.On("IsTimeoutError", mock.Anything).Return(false).Once()
	err = rm.RegisterAgent("SomeRegion", "SomeRole", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected error")
}
