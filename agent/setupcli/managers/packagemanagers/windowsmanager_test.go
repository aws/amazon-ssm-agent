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

//go:build windows
// +build windows

// Package packagemanagers holds functions querying using local package manager
package packagemanagers

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common"
	mhMock "github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common/mocks"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/servicemanagers"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/verificationmanagers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestWindowsManager_GetFilesReqForInstall_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	windowsMgr := windowsManager{helperMock}
	logMock := logmocks.NewMockLog()
	file := windowsMgr.GetFilesReqForInstall(logMock)
	assert.Equal(t, file[0], common.AmazonWindowsSetupFile)
}

func TestWindowsManager_InstallAgent_NotNano_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	installedAgentVersionPath := "temp1"
	isNanoServer = func(log log.T) (bool, error) {
		return false, nil
	}
	timeSleep = func(d time.Duration) {
		return
	}
	setupExecPath := filepath.Join(installedAgentVersionPath, common.AmazonWindowsSetupFile)

	helperMock.On("RunCommandWithCustomTimeout", mock.Anything, appconfig.PowerShellPluginCommandName, "(Start-Process", "'"+setupExecPath+"'", "-ArgumentList @('ALLOWEC2INSTALL=YES', 'SKIPSYMLINKSCAN=YES', '/quiet', '/norestart')", "-Wait)").Return("", nil)
	windowsMgr := windowsManager{helperMock}
	logMock := logmocks.NewMockLog()
	err := windowsMgr.InstallAgent(logMock, installedAgentVersionPath)
	assert.NoError(t, err)
}

func TestWindowsManager_InstallAgent_Nano_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	installedAgentVersionPath := "temp1"
	isNanoServer = func(log log.T) (bool, error) {
		return true, fmt.Errorf("test1")
	}
	fileUnCompress = func(log log.T, src, dest string) error {
		return nil
	}
	fileMoveFiles = func(sourceDir, destDir string) error {
		return nil
	}
	timeSleep = func(d time.Duration) {
		return
	}
	tempFolder := "temp"
	tempPath := filepath.Join(installedAgentVersionPath, tempFolder)
	amazonExecutable := filepath.Join(tempPath, common.AmazonSSMExecutable)
	helperMock.On("RunCommand", "sc.exe", "create", "AmazonSSMAgent", "binpath='"+amazonExecutable+"'", "start=auto", "displayname='Amazon SSM Agent'").Return("", nil).Once()
	helperMock.On("RunCommand", "sc.exe", "description", "AmazonSSMAgent", "'Amazon SSM Agent'").Return("", nil).Once()
	helperMock.On("RunCommand", "sc.exe", "failureflag", "AmazonSSMAgent", "1").Return("", nil).Once()
	helperMock.On("RunCommand", "sc.exe", "failure", "AmazonSSMAgent", "reset=86400", "actions= restart/30000/restart/30000/restart/30000").Return("", nil).Once()
	helperMock.On("RunCommand", "C:\\Windows\\System32\\net.exe", "start", "AmazonSSMAgent").Return("", nil).Once()
	windowsMgr := windowsManager{helperMock}
	logMock := logmocks.NewMockLog()
	err := windowsMgr.InstallAgent(logMock, installedAgentVersionPath)
	assert.NoError(t, err)
}

func TestWindowsManager_InstallAgent_NonTimeout_Failure(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	installedAgentVersionPath := "temp1"
	isNanoServer = func(log log.T) (bool, error) {
		return true, fmt.Errorf("test1")
	}
	fileUnCompress = func(log log.T, src, dest string) error {
		return nil
	}
	fileMoveFiles = func(sourceDir, destDir string) error {
		return nil
	}
	timeSleep = func(d time.Duration) {
		return
	}
	tempFolder := "temp"
	tempPath := filepath.Join(installedAgentVersionPath, tempFolder)
	amazonExecutable := filepath.Join(tempPath, common.AmazonSSMExecutable)
	helperMock.On("RunCommand", "sc.exe", "create", "AmazonSSMAgent", "binpath='"+amazonExecutable+"'", "start=auto", "displayname='Amazon SSM Agent'").Return("", fmt.Errorf("err1")).Once()
	helperMock.On("IsTimeoutError", mock.Anything).Return(false)

	windowsMgr := windowsManager{helperMock}
	logMock := logmocks.NewMockLog()
	getInstalledAgentVersionFunc = func(m *windowsManager) (string, error) {
		return "", nil
	}
	err := windowsMgr.InstallAgent(logMock, installedAgentVersionPath)
	assert.Error(t, err)
}

func TestWindowsManager_InstallAgent_Timeout_Failure(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	installedAgentVersionPath := "temp1"
	isNanoServer = func(log log.T) (bool, error) {
		return true, fmt.Errorf("test1")
	}
	fileUnCompress = func(log log.T, src, dest string) error {
		return nil
	}
	fileMoveFiles = func(sourceDir, destDir string) error {
		return nil
	}
	timeSleep = func(d time.Duration) {
		return
	}
	tempFolder := "temp"
	tempPath := filepath.Join(installedAgentVersionPath, tempFolder)
	amazonExecutable := filepath.Join(tempPath, common.AmazonSSMExecutable)
	helperMock.On("RunCommand", "sc.exe", "create", "AmazonSSMAgent", "binpath='"+amazonExecutable+"'", "start=auto", "displayname='Amazon SSM Agent'").Return("", fmt.Errorf("err1")).Once()
	helperMock.On("IsTimeoutError", mock.Anything).Return(true)

	windowsMgr := windowsManager{helperMock}
	logMock := logmocks.NewMockLog()
	err := windowsMgr.InstallAgent(logMock, installedAgentVersionPath)
	getInstalledAgentVersionFunc = func(m *windowsManager) (string, error) {
		return "", nil
	}
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "command timed out")
}

func TestWindowsManager_UninstallAgent_Success(t *testing.T) {
	isNanoServer = func(log log.T) (bool, error) {
		return true, fmt.Errorf("test1")
	}
	helperMock := &mhMock.IManagerHelper{}
	folderPath := "temp1"
	helperMock.On("RunCommand", "C:\\Windows\\System32\\net.exe", "stop", "AmazonSSMAgent").Return("", nil).Once()
	helperMock.On("RunCommand", "Get-CimInstance", "-ClassName", "Win32_Service", "-Filter", "'Name=\"AmazonSSMAgent\"'", "|", "Invoke-CimMethod", "-MethodName", "Delete").Return("", nil)
	windowsMgr := windowsManager{helperMock}
	logMock := logmocks.NewMockLog()
	visitedCount := 0
	deleteDir = func(dirName string) (err error) {
		visitedCount++
		return nil
	}
	ioUtilReadDir = func(dirname string) ([]fs.FileInfo, error) {
		skipFiles := []fs.FileInfo{&MockFileInfo{
			name: appconfig.SeelogConfigFileName,
		}, &MockFileInfo{
			name: appconfig.AppConfigFileName,
		},
		}
		return skipFiles, nil
	}
	err := windowsMgr.UninstallAgent(logMock, folderPath)
	assert.Equal(t, visitedCount, 0)
	assert.NoError(t, err)
}

func TestWindowsManager_UninstallAgent_MultipleFiles_Success(t *testing.T) {
	isNanoServer = func(log log.T) (bool, error) {
		return true, fmt.Errorf("test1")
	}
	helperMock := &mhMock.IManagerHelper{}
	folderPath := "temp1"
	helperMock.On("RunCommand", "C:\\Windows\\System32\\net.exe", "stop", "AmazonSSMAgent").Return("", nil).Once()
	helperMock.On("RunCommand", "Get-CimInstance", "-ClassName", "Win32_Service", "-Filter", "'Name=\"AmazonSSMAgent\"'", "|", "Invoke-CimMethod", "-MethodName", "Delete").Return("", nil)
	windowsMgr := windowsManager{helperMock}
	logMock := logmocks.NewMockLog()
	visitedCount := 0
	deleteDir = func(dirName string) (err error) {
		visitedCount++
		return nil
	}
	ioUtilReadDir = func(dirname string) ([]fs.FileInfo, error) {
		skipFiles := []fs.FileInfo{&MockFileInfo{
			name: "dasfdsfds",
		}, &MockFileInfo{
			name: appconfig.AppConfigFileName,
		},
		}
		return skipFiles, nil
	}
	err := windowsMgr.UninstallAgent(logMock, folderPath)
	assert.Equal(t, visitedCount, 1)
	assert.NoError(t, err)
}

func TestWindowsManager_UnInstallAgent_NotNano_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	installedAgentVersionPath := "temp1"
	isNanoServer = func(log log.T) (bool, error) {
		return false, nil
	}
	timeSleep = func(d time.Duration) {
		return
	}
	setupExecPath := filepath.Join(installedAgentVersionPath, common.AmazonWindowsSetupFile)
	helperMock.On("RunCommandWithCustomTimeout", mock.Anything, appconfig.PowerShellPluginCommandName, "(Start-Process", "'"+setupExecPath+"'", "-ArgumentList @('/uninstall', '/quiet', '/norestart')", "-Wait)").Return("", nil)
	windowsMgr := windowsManager{helperMock}
	logMock := logmocks.NewMockLog()
	err := windowsMgr.UninstallAgent(logMock, installedAgentVersionPath)
	assert.NoError(t, err)
}

func TestWindowsManager_GetName(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	winMgr := windowsManager{helperMock}
	mgrEnv := winMgr.GetName()
	assert.Equal(t, mgrEnv, "windows")
}

func TestWindowsManager_GetSupportedServiceManagers_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	winMgr := windowsManager{helperMock}
	svcMgr := winMgr.GetSupportedServiceManagers()
	assert.Equal(t, []servicemanagers.ServiceManager{servicemanagers.Windows}, svcMgr)
}

func TestWindowsManager_GetSupportedVerificationManager_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	winMgr := windowsManager{helperMock}
	verMgr := winMgr.GetSupportedVerificationManager()
	assert.Equal(t, verificationmanagers.Windows, verMgr)
}

func TestWindowsManager_GetFileExtension_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	winMgr := windowsManager{helperMock}
	ext := winMgr.GetFileExtension()
	assert.Equal(t, ext, ".exe")
}

type MockFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

func (m MockFileInfo) Name() string {
	return m.name
}

func (m MockFileInfo) Size() int64 {
	return m.size
}

func (m MockFileInfo) Mode() os.FileMode {
	return m.mode
}

func (m MockFileInfo) ModTime() time.Time {
	return m.modTime
}

func (m MockFileInfo) IsDir() bool {
	return m.isDir
}

func (m MockFileInfo) Sys() interface{} {
	return nil
}
