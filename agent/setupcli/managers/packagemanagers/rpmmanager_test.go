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

//go:build freebsd || linux || netbsd || openbsd
// +build freebsd linux netbsd openbsd

// Package packagemanagers holds functions querying using local package manager
package packagemanagers

import (
	"fmt"
	"path/filepath"
	"testing"

	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common"
	mhMock "github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common/mocks"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/servicemanagers"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/verificationmanagers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRpmManager_GetFilesReqForInstall_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	rpmMgr := rpmManager{helperMock}
	logMock := logmocks.NewMockLog()

	file := rpmMgr.GetFilesReqForInstall(logMock)
	assert.Equal(t, file[0], rpmFile)
}

func TestRpmManager_InstallAgent_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	folderPath := "temp1"
	rpmPath := filepath.Join(folderPath, rpmFile)
	helperMock.On("RunCommand", "rpm", "-U", rpmPath).Return("", nil)
	rpmMgr := rpmManager{helperMock}
	logMock := logmocks.NewMockLog()
	err := rpmMgr.InstallAgent(logMock, folderPath)
	assert.NoError(t, err)
}

func TestRpmManager_InstallAgent_Timeout_Failure(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	folderPath := "temp1"
	rpmPath := filepath.Join(folderPath, rpmFile)
	helperMock.On("RunCommand", "rpm", "-U", rpmPath).Return("", fmt.Errorf("err1"))
	helperMock.On("IsTimeoutError", mock.Anything).Return(true)
	rpmMgr := rpmManager{helperMock}
	logMock := logmocks.NewMockLog()
	err := rpmMgr.InstallAgent(logMock, folderPath)
	assert.Error(t, err)
}

func TestRpmManager_InstallAgent_NoTimeout_Failure(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	folderPath := "temp1"
	rpmPath := filepath.Join(folderPath, rpmFile)
	helperMock.On("RunCommand", "rpm", "-U", rpmPath).Return("", fmt.Errorf("err1"))
	helperMock.On("IsTimeoutError", mock.Anything).Return(false)
	rpmMgr := rpmManager{helperMock}
	logMock := logmocks.NewMockLog()
	err := rpmMgr.InstallAgent(logMock, folderPath)
	assert.Error(t, err)
}

func TestRpmManager_UninstallAgent_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	folderPath := "temp1"
	helperMock.On("RunCommand", "rpm", "-e", "amazon-ssm-agent").Return("", nil)
	rpmMgr := rpmManager{helperMock}
	logMock := logmocks.NewMockLog()
	err := rpmMgr.UninstallAgent(logMock, folderPath)
	assert.NoError(t, err)
}

func TestRpmManager_UninstallAgent_Timeout_Failure(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	folderPath := "temp1"
	helperMock.On("RunCommand", "rpm", "-e", "amazon-ssm-agent").Return("", fmt.Errorf("err1"))
	helperMock.On("IsTimeoutError", mock.Anything).Return(true)
	rpmMgr := rpmManager{helperMock}
	logMock := logmocks.NewMockLog()
	err := rpmMgr.UninstallAgent(logMock, folderPath)
	assert.Error(t, err)
}

func TestRpmManager_UninstallAgent_NoTimeout_Failure(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	folderPath := "temp1"
	helperMock.On("RunCommand", "rpm", "-e", "amazon-ssm-agent").Return("", fmt.Errorf("err1"))
	helperMock.On("IsTimeoutError", mock.Anything).Return(false)
	rpmMgr := rpmManager{helperMock}
	logMock := logmocks.NewMockLog()
	err := rpmMgr.UninstallAgent(logMock, folderPath)
	assert.Error(t, err)
}

func TestRpmManager_IsAgentInstalled_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	helperMock.On("RunCommand", "rpm", "-q", "amazon-ssm-agent").Return("3.2.2.2", nil)
	rpmMgr := rpmManager{helperMock}
	installed, err := rpmMgr.IsAgentInstalled()
	assert.True(t, installed)
	assert.NoError(t, err)
}

func TestRpmManager_IsAgentInstalledIsExitCodeError_One_Failure(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	helperMock.On("RunCommand", "rpm", "-q", "amazon-ssm-agent").Return("3.2.2.2", fmt.Errorf("err1"))
	helperMock.On("IsExitCodeError", mock.Anything).Return(true)
	helperMock.On("GetExitCode", mock.Anything).Return(1)
	rpmMgr := rpmManager{helperMock}
	installed, err := rpmMgr.IsAgentInstalled()
	assert.False(t, installed)
	assert.NoError(t, err)
}

func TestRpmManager_IsAgentInstalledIsExitCodeError_Zero_Failure(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	helperMock.On("RunCommand", "rpm", "-q", "amazon-ssm-agent").Return("3.2.2.2", fmt.Errorf("err1"))
	helperMock.On("IsExitCodeError", mock.Anything).Return(true)
	helperMock.On("GetExitCode", mock.Anything).Return(0)
	rpmMgr := rpmManager{helperMock}
	installed, err := rpmMgr.IsAgentInstalled()
	assert.False(t, installed)
	assert.Error(t, err)
}

func TestRpmManager_IsAgentInstalledIsTimeoutError_Failure(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	helperMock.On("RunCommand", "rpm", "-q", "amazon-ssm-agent").Return("3.2.2.2", fmt.Errorf("err1"))
	helperMock.On("IsExitCodeError", mock.Anything).Return(false)
	helperMock.On("IsTimeoutError", mock.Anything).Return(true)
	rpmMgr := rpmManager{helperMock}
	installed, err := rpmMgr.IsAgentInstalled()
	assert.False(t, installed)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Command timed out")
}

func TestRpmManager_IsAgentInstalledIsGeneralError_Failure(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	helperMock.On("RunCommand", "rpm", "-q", "amazon-ssm-agent").Return("3.2.2.2", fmt.Errorf("err1"))
	helperMock.On("IsExitCodeError", mock.Anything).Return(false)
	helperMock.On("IsTimeoutError", mock.Anything).Return(false)
	rpmMgr := rpmManager{helperMock}
	installed, err := rpmMgr.IsAgentInstalled()
	assert.False(t, installed)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Unexpected error with output")
}

func TestRpmManager_IsManagerEnvironment_CommandFound(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	helperMock.On("IsCommandAvailable", "rpm").Return(true)
	rpmMgr := rpmManager{helperMock}
	isMgrEnv := rpmMgr.IsManagerEnvironment()
	assert.True(t, isMgrEnv)
}

func TestRpmManager_IsManagerEnvironment_CommandNotFound(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	helperMock.On("IsCommandAvailable", "rpm").Return(false)
	rpmMgr := rpmManager{helperMock}
	isMgrEnv := rpmMgr.IsManagerEnvironment()
	assert.False(t, isMgrEnv)
}

func TestRpmManager_GetName(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	rpmMgr := rpmManager{helperMock}
	mgrEnv := rpmMgr.GetName()
	assert.Equal(t, mgrEnv, "rpm")
}

func TestRpmManager_GetSupportedServiceManagers_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	rpmMgr := rpmManager{helperMock}
	svcMgr := rpmMgr.GetSupportedServiceManagers()
	assert.Equal(t, []servicemanagers.ServiceManager{servicemanagers.SystemCtl, servicemanagers.Upstart}, svcMgr)
}

func TestRpmManager_GetSupportedVerificationManager_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	rpmMgr := rpmManager{helperMock}
	verMgr := rpmMgr.GetSupportedVerificationManager()
	assert.Equal(t, verificationmanagers.Linux, verMgr)
}

func TestRpmManager_GetFileExtension_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	rpmMgr := rpmManager{helperMock}
	ext := rpmMgr.GetFileExtension()
	assert.Equal(t, ext, ".rpm")
}

func TestRpmManager_GetInstalledAgentVersion_AgentBinary_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	helperMock.On("RunCommand", "rpm", "-q", "--qf", "%{VERSION}", "amazon-ssm-agent").Return("3.2.2.3", nil).Once()
	rpmMgr := rpmManager{helperMock}
	version, err := rpmMgr.GetInstalledAgentVersion()
	assert.Equal(t, version, "3.2.2.3")
	assert.NoError(t, err)
}

func TestRpmManager_GetInstalledAgentVersion_IsExitCodeError_Failure(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	helperMock.On("RunCommand", "rpm", "-q", "--qf", "%{VERSION}", "amazon-ssm-agent").Return("3.2.2.3", fmt.Errorf("err1")).Once()
	helperMock.On("IsExitCodeError", mock.Anything).Return(true)
	helperMock.On("GetExitCode", mock.Anything).Return(1)
	rpmMgr := rpmManager{helperMock}
	version, err := rpmMgr.GetInstalledAgentVersion()
	assert.Equal(t, version, "")
	assert.Error(t, err)
}

func TestRpmManager_GetInstalledAgentVersion_IsTimeOut_Failure(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	helperMock.On("RunCommand", "rpm", "-q", "--qf", "%{VERSION}", "amazon-ssm-agent").Return("3.2.2.3", fmt.Errorf("err1")).Once()
	helperMock.On("IsExitCodeError", mock.Anything).Return(false).Once()
	helperMock.On("IsTimeoutError", mock.Anything).Return(true).Once()

	rpmMgr := rpmManager{helperMock}
	version, err := rpmMgr.GetInstalledAgentVersion()
	assert.Equal(t, version, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Command timed out")
	helperMock.AssertExpectations(t)

	helperMock.On("RunCommand", "rpm", "-q", "--qf", "%{VERSION}", "amazon-ssm-agent").Return("3.2.2.3", fmt.Errorf("err1")).Once()
	helperMock.On("IsExitCodeError", mock.Anything).Return(false).Once()
	helperMock.On("IsTimeoutError", mock.Anything).Return(false).Once()
	version, err = rpmMgr.GetInstalledAgentVersion()
	assert.Error(t, err)
	assert.Equal(t, version, "")
	helperMock.AssertExpectations(t)

	helperMock.On("RunCommand", "rpm", "-q", "--qf", "%{VERSION}", "amazon-ssm-agent").Return("3.2.2.3", fmt.Errorf("err1")).Once()
	helperMock.On("IsExitCodeError", mock.Anything).Return(true).Once()
	helperMock.On("GetExitCode", mock.Anything).Return(common.PackageNotInstalledExitCode).Once()
	version, err = rpmMgr.GetInstalledAgentVersion()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent not installed with rpm")
	helperMock.AssertExpectations(t)
}
