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

//go:build windows
// +build windows

// Package servicemanagers contains functions related to service manager
package servicemanagers

import (
	"fmt"
	"testing"

	mhMock "github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common/mocks"
	"github.com/stretchr/testify/assert"
)

func TestWindowsManager_StartAgent_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	helperMock.On("RunCommand", netExecPath, "start", serviceName).Return("", nil)
	windowsMgr := windowsManager{helperMock}
	err := windowsMgr.StartAgent()
	assert.NoError(t, err)
}

func TestWindowsManager_StartAgent_ServiceAlreadyStarted_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	helperMock.On("RunCommand", netExecPath, "start", serviceName).Return("dssd Service has already been started dsd", fmt.Errorf("err1"))
	windowsMgr := windowsManager{helperMock}
	err := windowsMgr.StartAgent()
	assert.NoError(t, err)
}

func TestWindowsManager_StartAgent_Failure(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	helperMock.On("RunCommand", netExecPath, "start", serviceName).Return("dssd Service has ", fmt.Errorf("err1"))
	windowsMgr := windowsManager{helperMock}
	err := windowsMgr.StartAgent()
	assert.Error(t, err)
}

func TestWindowsManager_StopAgent_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	helperMock.On("RunCommand", netExecPath, "stop", serviceName).Return("", nil)
	windowsMgr := windowsManager{helperMock}
	err := windowsMgr.StopAgent()
	assert.NoError(t, err)
}

func TestWindowsManager_StopAgent_ServiceAlreadyStarted_Success(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	helperMock.On("RunCommand", netExecPath, "stop", serviceName).Return("dssd service is not started dsd", fmt.Errorf("err1"))
	windowsMgr := windowsManager{helperMock}
	err := windowsMgr.StopAgent()
	assert.NoError(t, err)
}

func TestWindowsManager_StopAgent_Failure(t *testing.T) {
	helperMock := &mhMock.IManagerHelper{}
	helperMock.On("RunCommand", netExecPath, "stop", serviceName).Return("dssd Service has ", fmt.Errorf("err1"))
	windowsMgr := windowsManager{helperMock}
	err := windowsMgr.StopAgent()
	assert.Error(t, err)
}
