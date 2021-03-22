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
package linuxcontainerutil

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	updateinfomocks "github.com/aws/amazon-ssm-agent/agent/updateutil/updateinfo/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func successMock() *DepMock {
	depmock := DepMock{}
	depmock.On("UpdateUtilExeCommandOutput", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("True", nil)

	info := &updateinfomocks.T{}
	info.On("GetPlatform").Return(updateconstants.PlatformLinux)

	depmock.On("GetInstanceInfo", mock.Anything).Return(info, nil)
	return &depmock
}

func unsupportedPlatformMock() *DepMock {
	depmock := DepMock{}
	depmock.On("UpdateUtilExeCommandOutput", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("True", nil)

	info := &updateinfomocks.T{}
	info.On("GetPlatform").Return(updateconstants.PlatformUbuntu)

	depmock.On("GetInstanceInfo", mock.Anything).Return(info, nil)
	return &depmock
}

func TestInstall(t *testing.T) {
	depOrig := dep
	containerMock := successMock()
	dep = containerMock
	defer func() { dep = depOrig }()

	output := iohandler.DefaultIOHandler{}
	RunInstallCommands(context.NewMockDefault(), "", &output)

	assert.Equal(t, output.GetExitCode(), 0)
	assert.Contains(t, output.GetStdout(), "Installation complete")
	containerMock.AssertCalled(t, "GetInstanceInfo", mock.Anything)
	containerMock.AssertNumberOfCalls(t, "UpdateUtilExeCommandOutput", 3)
}

func TestInstallUnsupportedPlatform(t *testing.T) {
	depOrig := dep
	containerMock := unsupportedPlatformMock()
	dep = containerMock
	defer func() { dep = depOrig }()

	output := iohandler.DefaultIOHandler{}
	RunInstallCommands(context.NewMockDefault(), "", &output)

	assert.Equal(t, output.GetExitCode(), 1)
	assert.Equal(t, output.GetStdout(), "")
	assert.NotEqual(t, output.GetStderr(), "")
	containerMock.AssertCalled(t, "GetInstanceInfo", mock.Anything)
	containerMock.AssertNumberOfCalls(t, "UpdateUtilExeCommandOutput", 0)
}

func TestUnInstall(t *testing.T) {
	depOrig := dep
	containerMock := successMock()
	dep = containerMock
	defer func() { dep = depOrig }()

	output := iohandler.DefaultIOHandler{}
	RunUninstallCommands(context.NewMockDefault(), "", &output)

	assert.Equal(t, output.GetExitCode(), 0)
	assert.Contains(t, output.GetStderr(), "")
	assert.Contains(t, output.GetStdout(), "Uninstall complete")
	containerMock.AssertCalled(t, "GetInstanceInfo", mock.Anything)
	containerMock.AssertNumberOfCalls(t, "UpdateUtilExeCommandOutput", 1)
}
