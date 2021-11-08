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

func TestSystemctlManager_StartAgent(t *testing.T) {
	u, helperMock := createSystemCtlManager()

	helperMock.On("RunCommand", mock.Anything, mock.Anything, u.serviceName).Return("", fmt.Errorf("SomeError")).Once()
	assert.Error(t, u.StartAgent())

	helperMock.On("RunCommand", mock.Anything, mock.Anything, u.serviceName).Return("success", nil).Once()
	assert.NoError(t, u.StartAgent())
}

func TestSystemctlManager_StopAgent(t *testing.T) {
	u, helperMock := createSystemCtlManager()

	helperMock.On("RunCommand", mock.Anything, mock.Anything, u.serviceName).Return("", fmt.Errorf("SomeError")).Once()
	assert.Error(t, u.StopAgent())

	helperMock.On("RunCommand", mock.Anything, mock.Anything, u.serviceName).Return("success", nil).Once()
	assert.NoError(t, u.StopAgent())
}

func TestSystemctlManager_GetAgentStatus(t *testing.T) {
	u, helperMock := createSystemCtlManager()

	// Test isActive true
	helperMock.On("RunCommand", mock.Anything, "is-active", u.serviceName).Return("", nil).Once()
	status, err := u.GetAgentStatus()
	assert.NoError(t, err)
	assert.Equal(t, common.Running, status)

	// Test fallback - is running
	helperMock.On("RunCommand", mock.Anything, "is-active", u.serviceName).Return("", fmt.Errorf("SomeActiveError")).Once()
	helperMock.On("RunCommand", mock.Anything, "status", u.serviceName).Return("", nil).Once()
	status, err = u.GetAgentStatus()
	assert.NoError(t, err)
	assert.Equal(t, common.Running, status)

	// Test stopped
	helperMock.On("RunCommand", mock.Anything, "is-active", u.serviceName).Return("", fmt.Errorf("SomeActiveError")).Once()
	helperMock.On("RunCommand", mock.Anything, "status", u.serviceName).Return("", fmt.Errorf("SomeStatusError")).Once()
	helperMock.On("IsExitCodeError", mock.Anything).Return(true).Once()
	helperMock.On("GetExitCode", mock.Anything).Return(systemCtlServiceStoppedExitCode).Once()
	status, err = u.GetAgentStatus()
	assert.NoError(t, err)
	assert.Equal(t, common.Stopped, status)

	// Test not installed
	helperMock.On("RunCommand", mock.Anything, "is-active", u.serviceName).Return("", fmt.Errorf("SomeActiveError")).Once()
	helperMock.On("RunCommand", mock.Anything, "status", u.serviceName).Return("", fmt.Errorf("SomeStatusError")).Once()
	helperMock.On("IsExitCodeError", mock.Anything).Return(true).Once()
	helperMock.On("GetExitCode", mock.Anything).Return(systemCtlServiceNotFoundExitCode).Once()
	status, err = u.GetAgentStatus()
	assert.NoError(t, err)
	assert.Equal(t, common.NotInstalled, status)

	// Test undefined exit code
	helperMock.On("RunCommand", mock.Anything, "is-active", u.serviceName).Return("", fmt.Errorf("SomeActiveError")).Once()
	helperMock.On("RunCommand", mock.Anything, "status", u.serviceName).Return("", fmt.Errorf("SomeStatusError")).Once()
	helperMock.On("IsExitCodeError", mock.Anything).Return(true).Once()
	helperMock.On("GetExitCode", mock.Anything).Return(0).Once()
	status, err = u.GetAgentStatus()
	assert.Error(t, err)
	assert.Equal(t, common.UndefinedStatus, status)

	// Test timeout error
	helperMock.On("RunCommand", mock.Anything, "is-active", u.serviceName).Return("", fmt.Errorf("SomeActiveError")).Once()
	helperMock.On("RunCommand", mock.Anything, "status", u.serviceName).Return("", fmt.Errorf("SomeStatusError")).Once()
	helperMock.On("IsExitCodeError", mock.Anything).Return(false).Once()
	helperMock.On("IsTimeoutError", mock.Anything).Return(true).Once()
	status, err = u.GetAgentStatus()
	assert.Error(t, err)
	assert.Equal(t, common.UndefinedStatus, status)

	// Test unexpected error
	helperMock.On("RunCommand", mock.Anything, "is-active", u.serviceName).Return("", fmt.Errorf("SomeActiveError")).Once()
	helperMock.On("RunCommand", mock.Anything, "status", u.serviceName).Return("", fmt.Errorf("SomeStatusError")).Once()
	helperMock.On("IsExitCodeError", mock.Anything).Return(false).Once()
	helperMock.On("IsTimeoutError", mock.Anything).Return(false).Once()
	status, err = u.GetAgentStatus()
	assert.Error(t, err)
	assert.Equal(t, common.UndefinedStatus, status)
}

func TestSystemctlManager_ReloadManager(t *testing.T) {
	u, helperMock := createSystemCtlManager()

	helperMock.On("RunCommand", mock.Anything, mock.Anything).Return("", fmt.Errorf("SomeError")).Once()
	assert.Error(t, u.ReloadManager())

	helperMock.On("RunCommand", mock.Anything, mock.Anything).Return("success", nil).Once()
	assert.NoError(t, u.ReloadManager())
}

func TestSystemctlManager_IsManagerEnvironment(t *testing.T) {
	u, helperMock := createSystemCtlManager()

	// Has all
	helperMock.On("IsCommandAvailable", u.dependentBinaries[0]).Return(true).Once()
	helperMock.On("IsCommandAvailable", u.dependentBinaries[1]).Return(true).Once()
	assert.True(t, u.IsManagerEnvironment())

	// Missing one
	helperMock.On("IsCommandAvailable", u.dependentBinaries[0]).Return(true).Once()
	helperMock.On("IsCommandAvailable", u.dependentBinaries[1]).Return(false).Once()
	assert.False(t, u.IsManagerEnvironment())
}

func TestSystemctlManager_AssertInfo(t *testing.T) {
	managerName := "RandomManagerName"
	u := systemCtlManager{
		managerName: managerName,
		managerType: SystemCtl,
	}

	assert.Equal(t, managerName, u.GetName())
	assert.Equal(t, SystemCtl, u.GetType())
}

func createSystemCtlManager() (*systemCtlManager, *mhMock.IManagerHelper) {
	helperMock := &mhMock.IManagerHelper{}

	return &systemCtlManager{
		helperMock,
		"SomeService",
		"SomeManagerName",
		Snap,
		[]string{"SomeBin1", "SomeBin2"},
	}, helperMock
}
