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

package servicemanagers

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common"
	mhMock "github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestUpstartManager_StartAgent(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}

	u := upstartManager{
		helperMock,
	}

	helperMock.On("RunCommand", mock.Anything, mock.Anything).Return("", fmt.Errorf("SomeError")).Once()
	assert.Error(t, u.StartAgent())

	helperMock.On("RunCommand", mock.Anything, mock.Anything).Return("success", nil).Once()
	assert.NoError(t, u.StartAgent())
}

func TestUpstartManager_StopAgent(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}

	u := upstartManager{
		helperMock,
	}

	helperMock.On("RunCommand", mock.Anything, mock.Anything).Return("", fmt.Errorf("SomeError")).Once()
	assert.Error(t, u.StopAgent())

	helperMock.On("RunCommand", mock.Anything, mock.Anything).Return("success", nil).Once()
	assert.NoError(t, u.StopAgent())
}

func TestUpstartManager_GetAgentStatus(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}

	u := upstartManager{
		helperMock,
	}

	// Test not installed
	helperMock.On("RunCommand", mock.Anything, mock.Anything).Return("", fmt.Errorf("SomeError")).Once()
	helperMock.On("IsExitCodeError", mock.Anything).Return(true).Once()
	helperMock.On("GetExitCode", mock.Anything).Return(upstartServiceNotFoundExitCode).Once()
	status, err := u.GetAgentStatus()
	assert.NoError(t, err)
	assert.Equal(t, common.NotInstalled, status)

	// Unexpected exit code
	helperMock.On("RunCommand", mock.Anything, mock.Anything).Return("", fmt.Errorf("SomeError")).Once()
	helperMock.On("IsExitCodeError", mock.Anything).Return(true).Once()
	helperMock.On("GetExitCode", mock.Anything).Return(0).Once()
	status, err = u.GetAgentStatus()
	assert.Error(t, err)
	assert.Equal(t, common.UndefinedStatus, status)

	// Test timeout error
	helperMock.On("RunCommand", mock.Anything, mock.Anything).Return("", fmt.Errorf("SomeError")).Once()
	helperMock.On("IsExitCodeError", mock.Anything).Return(false).Once()
	helperMock.On("IsTimeoutError", mock.Anything).Return(true).Once()
	status, err = u.GetAgentStatus()
	assert.Error(t, err)
	assert.Equal(t, common.UndefinedStatus, status)

	// Test unexpected error
	helperMock.On("RunCommand", mock.Anything, mock.Anything).Return("", fmt.Errorf("SomeError")).Once()
	helperMock.On("IsExitCodeError", mock.Anything).Return(false).Once()
	helperMock.On("IsTimeoutError", mock.Anything).Return(false).Once()
	status, err = u.GetAgentStatus()
	assert.Error(t, err)
	assert.Equal(t, common.UndefinedStatus, status)

	// Test running
	helperMock.On("RunCommand", mock.Anything, mock.Anything).Return("start/running", nil).Once()
	status, err = u.GetAgentStatus()
	assert.NoError(t, err)
	assert.Equal(t, common.Running, status)

	// Test stopped
	helperMock.On("RunCommand", mock.Anything, mock.Anything).Return("stop/waiting", nil).Once()
	status, err = u.GetAgentStatus()
	assert.NoError(t, err)
	assert.Equal(t, common.Stopped, status)

	// Test unexpected output
	helperMock.On("RunCommand", mock.Anything, mock.Anything).Return("SomeRandomOutput", nil).Once()
	status, err = u.GetAgentStatus()
	assert.Error(t, err)
	assert.Equal(t, common.UndefinedStatus, status)
}

func TestUpstartManager_ReloadManager(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}

	u := upstartManager{
		helperMock,
	}

	helperMock.On("RunCommand", mock.Anything, mock.Anything).Return("", fmt.Errorf("SomeError")).Once()
	assert.Error(t, u.ReloadManager())

	helperMock.On("RunCommand", mock.Anything, mock.Anything).Return("success", nil).Once()
	assert.NoError(t, u.ReloadManager())
}

func TestUpstartManager_IsManagerEnvironment(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}

	u := upstartManager{
		helperMock,
	}

	// Has all
	helperMock.On("IsCommandAvailable", "start").Return(true).Once()
	helperMock.On("IsCommandAvailable", "status").Return(true).Once()
	helperMock.On("IsCommandAvailable", "stop").Return(true).Once()
	helperMock.On("IsCommandAvailable", "initctl").Return(true).Once()
	assert.True(t, u.IsManagerEnvironment())

	// Missing one
	helperMock.On("IsCommandAvailable", "start").Return(false).Once()
	helperMock.On("IsCommandAvailable", "status").Return(true).Once()
	helperMock.On("IsCommandAvailable", "stop").Return(true).Once()
	helperMock.On("IsCommandAvailable", "initctl").Return(true).Once()
	assert.False(t, u.IsManagerEnvironment())
}

func TestUpstartManager_AssertInfo(t *testing.T) {
	u := upstartManager{}

	assert.Equal(t, "upstart", u.GetName())
	assert.Equal(t, Upstart, u.GetType())
}
