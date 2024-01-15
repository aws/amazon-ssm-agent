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
	"time"

	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common"
	mhMock "github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common/mocks"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/servicemanagers"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/verificationmanagers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDpkgManager_GetFilesReqForInstall_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	dpkgMgr := dpkgManager{helperMock}
	logMock := logmocks.NewMockLog()
	file := dpkgMgr.GetFilesReqForInstall(logMock)
	assert.Equal(t, file[0], debFile)
}

func TestDpkgManager_InstallAgent_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	folderPath := "temp1"
	debPath := filepath.Join(folderPath, debFile)
	helperMock.On("RunCommandWithCustomTimeout", time.Minute, "dpkg", "-i", debPath).Return("", nil)
	dpkgMgr := dpkgManager{helperMock}
	logMock := logmocks.NewMockLog()
	err := dpkgMgr.InstallAgent(logMock, folderPath)
	assert.NoError(t, err)
}

func TestDpkgManager_InstallAgent_Timeout_Failure(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	folderPath := "temp1"
	debPath := filepath.Join(folderPath, debFile)
	helperMock.On("RunCommandWithCustomTimeout", time.Minute, "dpkg", "-i", debPath).Return("", fmt.Errorf("err"))
	helperMock.On("IsTimeoutError", mock.Anything).Return(true)
	dpkgMgr := dpkgManager{helperMock}
	logMock := logmocks.NewMockLog()
	err := dpkgMgr.InstallAgent(logMock, folderPath)
	assert.Error(t, err)
}

func TestDpkgManager_InstallAgent_NoTimeout_Failure(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	folderPath := "temp1"
	debPath := filepath.Join(folderPath, debFile)
	helperMock.On("RunCommandWithCustomTimeout", time.Minute, "dpkg", "-i", debPath).Return("", fmt.Errorf("err"))
	helperMock.On("IsTimeoutError", mock.Anything).Return(false)
	dpkgMgr := dpkgManager{helperMock}
	logMock := logmocks.NewMockLog()
	err := dpkgMgr.InstallAgent(logMock, folderPath)
	assert.Error(t, err)
}

func TestDpkgManager_UninstallAgent_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	folderPath := "temp1"
	helperMock.On("RunCommand", "dpkg", "-P", "amazon-ssm-agent").Return("", nil)
	dpkgMgr := dpkgManager{helperMock}
	logMock := logmocks.NewMockLog()
	err := dpkgMgr.UninstallAgent(logMock, folderPath)
	assert.NoError(t, err)
}

func TestDpkgManager_UninstallAgent_Timeout_Failure(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	folderPath := "temp1"
	helperMock.On("RunCommand", "dpkg", "-P", "amazon-ssm-agent").Return("", fmt.Errorf("err"))
	helperMock.On("IsTimeoutError", mock.Anything).Return(true)
	dpkgMgr := dpkgManager{helperMock}
	logMock := logmocks.NewMockLog()
	err := dpkgMgr.UninstallAgent(logMock, folderPath)
	assert.Error(t, err)
}

func TestDpkgManager_UninstallAgent_NoTimeout_Failure(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	folderPath := "temp1"
	helperMock.On("RunCommand", "dpkg", "-P", "amazon-ssm-agent").Return("", fmt.Errorf("err"))
	helperMock.On("IsTimeoutError", mock.Anything).Return(false)
	dpkgMgr := dpkgManager{helperMock}
	logMock := logmocks.NewMockLog()
	err := dpkgMgr.UninstallAgent(logMock, folderPath)
	assert.Error(t, err)
}

func TestDpkgManager_IsAgentInstalled_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	helperMock.On("RunCommand", "dpkg", "-s", "amazon-ssm-agent").Return("Test\nStatus: install ok installed", nil)
	dpkgMgr := dpkgManager{helperMock}
	installed, err := dpkgMgr.IsAgentInstalled()
	assert.True(t, installed)
	assert.NoError(t, err)
}

func TestDpkgManager_IsAgentNotInstalled_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	helperMock.On("RunCommand", "dpkg", "-s", "amazon-ssm-agent").Return("Test\nStatus: deinstall ok config-files", nil)
	dpkgMgr := dpkgManager{helperMock}
	installed, err := dpkgMgr.IsAgentInstalled()
	assert.False(t, installed)
	assert.NoError(t, err)
}

func TestDpkgManager_IsAgentNotInstalledRandomStatus_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	helperMock.On("RunCommand", "dpkg", "-s", "amazon-ssm-agent").Return("Test\nStatus: dsfsdfs", nil)
	dpkgMgr := dpkgManager{helperMock}
	installed, err := dpkgMgr.IsAgentInstalled()
	assert.False(t, installed)
	assert.Error(t, err)
}

func TestDpkgManager_IsAgentNotInstalledPkgNotInstalled_Failure(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	helperMock.On("RunCommand", "dpkg", "-s", "amazon-ssm-agent").Return("Test\nStatus: dsfsdfs", nil)
	dpkgMgr := dpkgManager{helperMock}
	installed, err := dpkgMgr.IsAgentInstalled()
	assert.False(t, installed)
	assert.Error(t, err)
}

func TestDpkgManager_IsAgentInstalledIsExitCodeError_One_Failure(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	helperMock.On("RunCommand", "dpkg", "-s", "amazon-ssm-agent").Return("Test\nStatus: dsfsdfs", fmt.Errorf("err1"))
	helperMock.On("IsExitCodeError", mock.Anything).Return(true)
	helperMock.On("GetExitCode", mock.Anything).Return(1)
	dpkgMgr := dpkgManager{helperMock}
	installed, err := dpkgMgr.IsAgentInstalled()
	assert.False(t, installed)
	assert.NoError(t, err)
}

func TestDpkgManager_IsAgentInstalledIsExitCodeError_Zero_Failure(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	helperMock.On("RunCommand", "dpkg", "-s", "amazon-ssm-agent").Return("Test\nStatus: dsfsdfs", fmt.Errorf("err1"))
	helperMock.On("IsExitCodeError", mock.Anything).Return(true)
	helperMock.On("GetExitCode", mock.Anything).Return(0)
	dpkgMgr := dpkgManager{helperMock}
	installed, err := dpkgMgr.IsAgentInstalled()
	assert.False(t, installed)
	assert.Error(t, err)
}

func TestDpkgManager_IsAgentInstalledIsTimeoutError_Failure(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	helperMock.On("RunCommand", "dpkg", "-s", "amazon-ssm-agent").Return("Test\nStatus: dsfsdfs", fmt.Errorf("err1"))
	helperMock.On("IsExitCodeError", mock.Anything).Return(false)
	helperMock.On("IsTimeoutError", mock.Anything).Return(true)
	dpkgMgr := dpkgManager{helperMock}
	installed, err := dpkgMgr.IsAgentInstalled()
	assert.False(t, installed)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Command timed out")
}

func TestDpkgManager_IsAgentInstalledIsGeneralError_Failure(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	helperMock.On("RunCommand", "dpkg", "-s", "amazon-ssm-agent").Return("Test\nStatus: dsfsdfs", fmt.Errorf("err1"))
	helperMock.On("IsExitCodeError", mock.Anything).Return(false)
	helperMock.On("IsTimeoutError", mock.Anything).Return(false)
	dpkgMgr := dpkgManager{helperMock}
	installed, err := dpkgMgr.IsAgentInstalled()
	assert.False(t, installed)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Unexpected error with output")
}

func TestDpkgManager_IsManagerEnvironment_CommandFound(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	helperMock.On("IsCommandAvailable", "dpkg").Return(true)
	dpkgMgr := dpkgManager{helperMock}
	isMgrEnv := dpkgMgr.IsManagerEnvironment()
	assert.True(t, isMgrEnv)
}

func TestDpkgManager_IsManagerEnvironment_CommandNotFound(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	helperMock.On("IsCommandAvailable", "dpkg").Return(false)
	dpkgMgr := dpkgManager{helperMock}
	isMgrEnv := dpkgMgr.IsManagerEnvironment()
	assert.False(t, isMgrEnv)
}

func TestDpkgManager_GetName(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	dpkgMgr := dpkgManager{helperMock}
	mgrEnv := dpkgMgr.GetName()
	assert.Equal(t, mgrEnv, "dpkg")
}

func TestDpkgManager_GetSupportedServiceManagers_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	dpkgMgr := dpkgManager{helperMock}
	svcMgr := dpkgMgr.GetSupportedServiceManagers()
	assert.Equal(t, []servicemanagers.ServiceManager{servicemanagers.SystemCtl, servicemanagers.Upstart}, svcMgr)
}

func TestDpkgManager_GetSupportedVerificationManager_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	dpkgMgr := dpkgManager{helperMock}
	verMgr := dpkgMgr.GetSupportedVerificationManager()
	assert.Equal(t, verificationmanagers.Linux, verMgr)
}

func TestDpkgManager_GetFileExtension_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	dpkgMgr := dpkgManager{helperMock}
	ext := dpkgMgr.GetFileExtension()
	assert.Equal(t, ext, ".deb")
}

func TestDpkgManager_GetInstalledAgentVersion_AgentBinary_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	helperMock.On("RunCommand", "dpkg", "-s", "amazon-ssm-agent").Return("Test\nStatus: install ok installed", nil).Once()
	helperMock.On("RunCommand", "amazon-ssm-agent", "--version").Return(" 3.2.2.2 ", nil).Once()
	dpkgMgr := dpkgManager{helperMock}
	version, err := dpkgMgr.GetInstalledAgentVersion()
	assert.Equal(t, version, "3.2.2.2")
	assert.NoError(t, err)
}

func TestDpkgManager_GetInstalledAgentVersion_Dpkg_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	helperMock.On("RunCommand", "dpkg", "-s", "amazon-ssm-agent").Return("Test\nStatus: install ok installed", nil).Once()
	helperMock.On("RunCommand", "amazon-ssm-agent", "--version").Return(" ", fmt.Errorf("err1")).Once()
	helperMock.On("RunCommand", "dpkg-query", "-f=${version}", "-W", "amazon-ssm-agent").Return(" 3.2.2.2 ", nil).Once()
	dpkgMgr := dpkgManager{helperMock}
	version, err := dpkgMgr.GetInstalledAgentVersion()
	assert.Equal(t, version, "3.2.2.2")
	assert.NoError(t, err)
}

func TestDpkgManager_GetInstalledAgentVersion_IsExitCodeError_Failure(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	helperMock.On("RunCommand", "dpkg", "-s", "amazon-ssm-agent").Return("Test\nStatus: install ok installed", nil).Once()
	helperMock.On("RunCommand", "amazon-ssm-agent", "--version").Return(" ", fmt.Errorf("err1")).Once()
	helperMock.On("RunCommand", "dpkg-query", "-f=${version}", "-W", "amazon-ssm-agent").Return(" 3.2.2.2 ", fmt.Errorf("err1")).Once()
	helperMock.On("IsExitCodeError", mock.Anything).Return(true)
	helperMock.On("GetExitCode", mock.Anything).Return(1)
	dpkgMgr := dpkgManager{helperMock}
	version, err := dpkgMgr.GetInstalledAgentVersion()
	assert.Equal(t, version, "")
	assert.Error(t, err)
}

func TestDpkgManager_GetInstalledAgentVersion_IsTimeOut_Failure(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	helperMock.On("RunCommand", "dpkg", "-s", "amazon-ssm-agent").Return("Test\nStatus: install ok installed", nil).Once()
	helperMock.On("RunCommand", "amazon-ssm-agent", "--version").Return(" ", fmt.Errorf("err1")).Once()
	helperMock.On("RunCommand", "dpkg-query", "-f=${version}", "-W", "amazon-ssm-agent").Return(" 3.2.2.2 ", fmt.Errorf("err1")).Once()
	helperMock.On("IsExitCodeError", mock.Anything).Return(false).Once()
	helperMock.On("IsTimeoutError", mock.Anything).Return(true).Once()

	dpkgMgr := dpkgManager{helperMock}
	version, err := dpkgMgr.GetInstalledAgentVersion()
	assert.Equal(t, version, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Command timed out")
	helperMock.AssertExpectations(t)

	helperMock.On("RunCommand", "dpkg", "-s", "amazon-ssm-agent").Return("Test\nStatus: install ok installed", nil).Once()
	helperMock.On("RunCommand", "amazon-ssm-agent", "--version").Return(" ", fmt.Errorf("err1")).Once()
	helperMock.On("RunCommand", "dpkg-query", "-f=${version}", "-W", "amazon-ssm-agent").Return(" 3.2.2.2 ", fmt.Errorf("err1")).Once()
	helperMock.On("IsExitCodeError", mock.Anything).Return(false).Once()
	helperMock.On("IsTimeoutError", mock.Anything).Return(false).Once()
	version, err = dpkgMgr.GetInstalledAgentVersion()
	assert.Error(t, err)
	assert.Equal(t, version, "")
	helperMock.AssertExpectations(t)

	helperMock.On("RunCommand", "dpkg", "-s", "amazon-ssm-agent").Return("", fmt.Errorf("err1")).Once()
	helperMock.On("IsExitCodeError", mock.Anything).Return(true).Once()
	helperMock.On("GetExitCode", mock.Anything).Return(common.PackageNotInstalledExitCode).Once()
	version, err = dpkgMgr.GetInstalledAgentVersion()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent is not installed with dpkg")
	helperMock.AssertExpectations(t)
}
