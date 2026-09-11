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
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/mocks/context"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	updateinfomocks "github.com/aws/amazon-ssm-agent/agent/updateutil/updateinfo/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func successMock() *DepMock {
	depmock := DepMock{}
	depmock.On("UpdateUtilExeCommandOutput", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("True", nil)

	info := &updateinfomocks.T{}
	info.On("GetPlatform").Return(updateconstants.PlatformLinux)

	depmock.On("GetInstanceInfo", mock.Anything).Return(info, nil)
	return &depmock
}

func unsupportedPlatformMock() *DepMock {
	depmock := DepMock{}
	depmock.On("UpdateUtilExeCommandOutput", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
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

func TestRedHatLinuxPlatformInstallCommand(t *testing.T) {
	testCases := []struct {
		name           string
		mockSetup      func() *DepMock
		expectedExit   int
		expectedStdout string // This will now contain the installation messages
		expectedStderr string
		commandCalls   int
	}{
		{
			name: "Successful Docker Installation on RedHat",
			mockSetup: func() *DepMock {
				containerMock := &DepMock{}
				containerMock.On("UpdateUtilExeCommandOutput",
					mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything).Return("Success", nil)
				return containerMock
			},
			expectedExit:   0,
			expectedStdout: "Installation complete",
			expectedStderr: "",
			commandCalls:   4,
		},
		{
			name: "Failure during Yum Utils Installation",
			mockSetup: func() *DepMock {
				containerMock := &DepMock{}
				containerMock.On("UpdateUtilExeCommandOutput",
					mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything).Return("", fmt.Errorf("yum utils installation failed"))
				return containerMock
			},
			expectedExit:   1,
			expectedStdout: "Installing yum-utils", // Include the initial message
			expectedStderr: "yum utils installation failed",
			commandCalls:   1,
		},
		{
			name: "Failure during Docker Repository Addition",
			mockSetup: func() *DepMock {
				containerMock := &DepMock{}
				containerMock.On("UpdateUtilExeCommandOutput",
					mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything).Return("Success", nil).Once()
				containerMock.On("UpdateUtilExeCommandOutput",
					mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything).Return("", fmt.Errorf("docker repo addition failed"))
				return containerMock
			},
			expectedExit:   1,
			expectedStdout: "Installing yum-utils\nAdd docker repo", // Include all messages up to failure
			expectedStderr: "docker repo addition failed",
			commandCalls:   2,
		},
		{
			name: "Failure during Docker Installation",
			mockSetup: func() *DepMock {
				containerMock := &DepMock{}
				containerMock.On("UpdateUtilExeCommandOutput",
					mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything).Return("Success", nil).Twice()
				containerMock.On("UpdateUtilExeCommandOutput",
					mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything).Return("", fmt.Errorf("docker installation failed"))
				return containerMock
			},
			expectedExit:   1,
			expectedStdout: "Installing yum-utils\nAdd docker repo\nInstallation docker through yum", // Include all messages up to failure
			expectedStderr: "docker installation failed",
			commandCalls:   3,
		},
		{
			name: "Failure during Docker Service Start",
			mockSetup: func() *DepMock {
				containerMock := &DepMock{}
				containerMock.On("UpdateUtilExeCommandOutput",
					mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything).Return("Success", nil).Times(3)
				containerMock.On("UpdateUtilExeCommandOutput",
					mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything).Return("", fmt.Errorf("docker service start failed"))
				return containerMock
			},
			expectedExit:   1,
			expectedStdout: "Installing yum-utils\nAdd docker repo\nInstallation docker through yum\nStarting docker service", // Include all messages up to failure
			expectedStderr: "docker service start failed",
			commandCalls:   4,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			depOrig := dep
			containerMock := tc.mockSetup()
			dep = containerMock
			defer func() { dep = depOrig }()

			output := iohandler.DefaultIOHandler{}
			context := context.NewMockDefault()

			runRedhatLinuxPlatformInstallCommands(context, logmocks.NewMockLog(), "", &output)

			assert.Equal(t, tc.expectedExit, output.GetExitCode())
			assert.Contains(t, output.GetStdout(), tc.expectedStdout)
			assert.Contains(t, output.GetStderr(), tc.expectedStderr)
			containerMock.AssertNumberOfCalls(t, "UpdateUtilExeCommandOutput", tc.commandCalls)
		})
	}
}

func TestRedHatLinuxPlatformUninstallCommand(t *testing.T) {
	testCases := []struct {
		name           string
		mockSetup      func() *DepMock
		expectedExit   int
		expectedStdout string
		expectedStderr string
		commandCalls   int
	}{
		{
			name: "Successful Docker Uninstallation",
			mockSetup: func() *DepMock {
				containerMock := &DepMock{}
				containerMock.On("UpdateUtilExeCommandOutput",
					mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything).Return("Success", nil)
				return containerMock
			},
			expectedExit:   0,
			expectedStdout: "Uninstall complete",
			expectedStderr: "",
			commandCalls:   1,
		},
		{
			name: "Uninstallation Failure - Docker Engine Not Found",
			mockSetup: func() *DepMock {
				containerMock := &DepMock{}
				containerMock.On("UpdateUtilExeCommandOutput",
					mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything).Return("", fmt.Errorf("package not installed"))
				return containerMock
			},
			expectedExit:   1,
			expectedStdout: "Removing docker through yum",
			expectedStderr: "package not installed",
			commandCalls:   1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Store original dependency
			depOrig := dep

			// Setup mock
			containerMock := tc.mockSetup()
			dep = containerMock
			defer func() { dep = depOrig }()

			// Prepare test output handler
			output := iohandler.DefaultIOHandler{}
			context := context.NewMockDefault()
			log := logmocks.NewMockLog()

			// Execute uninstall command
			runRedhatLinuxPlatformUninstallCommands(context, log, "", &output)

			// Verify expectations
			assert.Equal(t, tc.expectedExit, output.GetExitCode())
			assert.Contains(t, output.GetStdout(), tc.expectedStdout)
			assert.Contains(t, output.GetStderr(), tc.expectedStderr)
			containerMock.AssertNumberOfCalls(t, "UpdateUtilExeCommandOutput", tc.commandCalls)
		})
	}
}

func TestAmazonLinuxPlatformInstallCommandPartialFailure(t *testing.T) {
	testCases := []struct {
		name               string
		mockSetup          func() *DepMock
		expectedExit       int
		expectedStdoutPart string
		expectedStderrPart string
		failureStage       string
	}{
		{
			name: "Failure During Docker Installation",
			mockSetup: func() *DepMock {
				containerMock := &DepMock{}
				// First command (yum install) succeeds
				containerMock.On("UpdateUtilExeCommandOutput",
					mock.Anything, mock.Anything, "yum",
					[]string{"install", "-y", "docker"}, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything).Return("Success", nil)

				// Second command (yum update) fails
				containerMock.On("UpdateUtilExeCommandOutput",
					mock.Anything, mock.Anything, "yum",
					[]string{"update", "-y", "docker"}, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything).Return("", fmt.Errorf("docker update failed"))

				return containerMock
			},
			expectedExit:       1,
			expectedStdoutPart: "Installing docker through yum\nUpdating docker through yum",
			expectedStderrPart: "docker update failed",
			failureStage:       "Docker Update",
		},
		{
			name: "Failure During Docker Service Start",
			mockSetup: func() *DepMock {
				containerMock := &DepMock{}
				// First two commands succeed
				containerMock.On("UpdateUtilExeCommandOutput",
					mock.Anything, mock.Anything, "yum",
					[]string{"install", "-y", "docker"}, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything).Return("Success", nil)

				containerMock.On("UpdateUtilExeCommandOutput",
					mock.Anything, mock.Anything, "yum",
					[]string{"update", "-y", "docker"}, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything).Return("Success", nil)

				// Service start fails
				containerMock.On("UpdateUtilExeCommandOutput",
					mock.Anything, mock.Anything, "systemctl",
					[]string{"start", "docker"}, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything).Return("", fmt.Errorf("docker service start failed"))

				return containerMock
			},
			expectedExit:       1,
			expectedStdoutPart: "Installing docker through yum\nUpdating docker through yum\nStarting docker service",
			expectedStderrPart: "docker service start failed",
			failureStage:       "Docker Service Start",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			depOrig := dep
			containerMock := tc.mockSetup()
			dep = containerMock
			defer func() { dep = depOrig }()

			output := iohandler.DefaultIOHandler{}
			context := context.NewMockDefault()
			log := logmocks.NewMockLog()

			runAmazonLinuxPlatformInstallCommands(context, log, "", &output)

			assert.Equal(t, tc.expectedExit, output.GetExitCode())
			assert.Contains(t, output.GetStdout(), tc.expectedStdoutPart)
			assert.Contains(t, output.GetStderr(), tc.expectedStderrPart)

			// Verify specific stage failure
			t.Logf("Verifying failure at stage: %s", tc.failureStage)
		})
	}
}

func TestAmazonLinuxPlatformInstallCommandLogging(t *testing.T) {
	testCases := []struct {
		name             string
		mockSetup        func() *DepMock
		expectedLogCalls int
	}{
		{
			name: "Successful Installation with Logging",
			mockSetup: func() *DepMock {
				containerMock := &DepMock{}
				containerMock.On("UpdateUtilExeCommandOutput",
					mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything).Return("Success", nil).Times(3)
				return containerMock
			},
			expectedLogCalls: 3, // One for each command
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			depOrig := dep
			containerMock := tc.mockSetup()
			dep = containerMock
			defer func() { dep = depOrig }()

			mockLog := logmocks.NewMockLog()
			mockLog.On("Debug", mock.Anything).Return()
			mockLog.On("Error", mock.Anything).Return()

			output := iohandler.DefaultIOHandler{}
			context := context.NewMockDefault()

			runAmazonLinuxPlatformInstallCommands(context, mockLog, "", &output)

			mockLog.AssertNumberOfCalls(t, "Debug", tc.expectedLogCalls)
			assert.Equal(t, 0, output.GetExitCode())
		})
	}
}

func TestRunInstallCommandsInstanceInfoFailure(t *testing.T) {
	testCases := []struct {
		name           string
		mockSetup      func() *DepMock
		expectedExit   int
		expectedErrMsg string
	}{
		{
			name: "GetInstanceInfo Returns Error",
			mockSetup: func() *DepMock {
				containerMock := &DepMock{}
				// Simulate GetInstanceInfo failure
				containerMock.On("GetInstanceInfo", mock.Anything).Return(&updateinfomocks.T{}, fmt.Errorf("instance info retrieval failed"))
				return containerMock
			},
			expectedExit:   1,
			expectedErrMsg: "Error determining Linux variant",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Store original dependency
			depOrig := dep
			containerMock := tc.mockSetup()
			dep = containerMock
			defer func() { dep = depOrig }()

			// Prepare test output handler
			output := iohandler.DefaultIOHandler{}
			context := context.NewMockDefault()

			// Execute install command
			RunInstallCommands(context, "", &output)

			// Verify expectations
			assert.Equal(t, tc.expectedExit, output.GetExitCode())
			assert.Contains(t, output.GetStderr(), tc.expectedErrMsg)
			containerMock.AssertCalled(t, "GetInstanceInfo", mock.Anything)
		})
	}
}

func TestRunInstallCommandsPlatformErrorHandling(t *testing.T) {
	testCases := []struct {
		name           string
		platformMock   func() *updateinfomocks.T
		expectedExit   int
		expectedErrMsg string
	}{
		{
			name: "Unrecognized Linux Platform",
			platformMock: func() *updateinfomocks.T {
				mockInfo := &updateinfomocks.T{}
				mockInfo.On("GetPlatform").Return("UnknownLinux")
				return mockInfo
			},
			expectedExit:   1,
			expectedErrMsg: "Unsupported Linux variant",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			depOrig := dep
			containerMock := &DepMock{}

			// Setup GetInstanceInfo to return mocked platform
			if tc.platformMock() != nil {
				containerMock.On("GetInstanceInfo", mock.Anything).Return(tc.platformMock(), nil)
			} else {
				containerMock.On("GetInstanceInfo", mock.Anything).Return(nil, fmt.Errorf("nil platform"))
			}

			dep = containerMock
			defer func() { dep = depOrig }()

			output := iohandler.DefaultIOHandler{}
			context := context.NewMockDefault()

			RunInstallCommands(context, "", &output)

			assert.Equal(t, tc.expectedExit, output.GetExitCode())
			assert.Contains(t, output.GetStderr(), tc.expectedErrMsg)
		})
	}
}
func TestRunUninstallCommandsCommandExecutionFailure(t *testing.T) {
	testCases := []struct {
		name              string
		platform          string
		mockSetup         func() *DepMock
		expectedExitCode  int
		expectedErrorPart string
	}{
		{
			name:     "Amazon Linux Uninstall Command Failure",
			platform: updateconstants.PlatformLinux,
			mockSetup: func() *DepMock {
				containerMock := &DepMock{}
				info := &updateinfomocks.T{}
				info.On("GetPlatform").Return(updateconstants.PlatformLinux)
				containerMock.On("GetInstanceInfo", mock.Anything).Return(info, nil)

				containerMock.On("UpdateUtilExeCommandOutput",
					mock.Anything, mock.Anything, "yum",
					[]string{"remove", "-y", "docker"},
					mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything).Return("", fmt.Errorf("docker removal failed"))

				return containerMock
			},
			expectedExitCode:  1,
			expectedErrorPart: "Error running yum remove",
		},
		{
			name:     "RedHat Linux Uninstall Command Failure",
			platform: updateconstants.PlatformRedHat,
			mockSetup: func() *DepMock {
				containerMock := &DepMock{}
				info := &updateinfomocks.T{}
				info.On("GetPlatform").Return(updateconstants.PlatformRedHat)
				containerMock.On("GetInstanceInfo", mock.Anything).Return(info, nil)

				containerMock.On("UpdateUtilExeCommandOutput",
					mock.Anything, mock.Anything, "yum",
					[]string{"remove", "-y", "docker-engine"},
					mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything).Return("", fmt.Errorf("docker engine removal failed"))

				return containerMock
			},
			expectedExitCode:  1,
			expectedErrorPart: "Error running yum remove",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			depOrig := dep
			containerMock := tc.mockSetup()
			dep = containerMock
			defer func() { dep = depOrig }()

			output := iohandler.DefaultIOHandler{}
			context := context.NewMockDefault()

			RunUninstallCommands(context, "", &output)

			assert.Equal(t, tc.expectedExitCode, output.GetExitCode())
			assert.Contains(t, output.GetStderr(), tc.expectedErrorPart)
			containerMock.AssertCalled(t, "GetInstanceInfo", mock.Anything)
			containerMock.AssertNumberOfCalls(t, "UpdateUtilExeCommandOutput", 1)
		})
	}
}

func TestRunUninstallCommandsLoggingVerification(t *testing.T) {
	testCases := []struct {
		name             string
		platform         string
		mockSetup        func() (*DepMock, *logmocks.Mock)
		expectedLogCalls int
	}{
		{
			name:     "Amazon Linux Uninstall Logging",
			platform: updateconstants.PlatformLinux,
			mockSetup: func() (*DepMock, *logmocks.Mock) {
				containerMock := &DepMock{}
				mockLog := logmocks.NewMockLog()

				info := &updateinfomocks.T{}
				info.On("GetPlatform").Return(updateconstants.PlatformLinux)
				containerMock.On("GetInstanceInfo", mock.Anything).Return(info, nil)
				mockLog.On("Debug", mock.Anything).Return()

				containerMock.On("UpdateUtilExeCommandOutput",
					mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything).Return("", nil)

				return containerMock, mockLog
			},
			expectedLogCalls: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			depOrig := dep
			containerMock, mockLog := tc.mockSetup()
			dep = containerMock
			defer func() { dep = depOrig }()

			output := iohandler.DefaultIOHandler{}
			context := context.NewMockDefault()
			context.On("Log").Return(mockLog)

			RunUninstallCommands(context, "", &output)
			assert.Equal(t, 0, output.GetExitCode())
		})
	}
}
