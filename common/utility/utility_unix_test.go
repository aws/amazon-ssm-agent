// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

//go:build integration && (freebsd || linux || netbsd || openbsd)

package utility

import (
	mocklog "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"os"
	"os/exec"
	"os/user"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_IsRunningElevatedPermissions_Success(t *testing.T) {
	userCurrent = func() (*user.User, error) {
		return &user.User{Username: ExpectedServiceRunningUser}, nil
	}
	err := IsRunningElevatedPermissions()
	assert.Nil(t, err)
}

func Test_IsRunningElevatedPermissions_Failure(t *testing.T) {
	userCurrent = func() (*user.User, error) {
		return &user.User{Username: "DummyUser"}, nil
	}
	err := IsRunningElevatedPermissions()
	assert.NotNil(t, err)
}

func TestWaitForCloudInit_CloudInitFinished(t *testing.T) {
	// Arrange
	log := mocklog.NewMockLog()
	fileInfo := &MockFileInfo{}
	fileInfo.On("Mode").Return(os.FileMode(0500)).Repeatability = 0
	osStat = func(string) (os.FileInfo, error) {
		return fileInfo, nil
	}

	execCommand = func(command string, args ...string) *exec.Cmd {
		assert.Equal(t, cloudInitPath, command)
		assert.Equal(t, "status", args[0])
		assert.Equal(t, "--wait", args[1])
		log.Info("Running fake `cloud-init status --wait` command")
		return exec.Command("echo", "(Fake Call) cloud-init status --wait")
	}

	// Act
	err := WaitForCloudInit(log, 600)

	// Assert
	assert.NoError(t, err, "Error should be nil")

}

func TestWaitForCloudInit_WhenCloudInitNotExecutable_ReturnsError(t *testing.T) {
	// Arrange
	log := mocklog.NewMockLog()
	fileInfo := &MockFileInfo{}
	fileInfo.On("Mode").Return(os.FileMode(0600)).Repeatability = 0
	osStat = func(string) (os.FileInfo, error) {
		return fileInfo, nil
	}

	// Act
	err := WaitForCloudInit(log, 600)

	// Assert
	assert.Error(t, err, "Should not execute cloud-init if the file if not executable")
	assert.Contains(t, err.Error(), "not executable", "Error message should contain 'not executable'")

}

func TestWaitForCloudInit_TimesOutWaitingForCloudInit(t *testing.T) {
	// Arrange
	log := mocklog.NewMockLog()
	fileInfo := &MockFileInfo{}
	fileInfo.On("Mode").Return(os.FileMode(0500)).Repeatability = 0
	osStat = func(string) (os.FileInfo, error) {
		return fileInfo, nil
	}

	execCommand = func(command string, args ...string) *exec.Cmd {
		assert.Equal(t, cloudInitPath, command)
		assert.Equal(t, "status", args[0])
		assert.Equal(t, "--wait", args[1])
		log.Info("Sleeping for 3 seconds to simulate a long running `cloud-init status --wait` command")
		return exec.Command("sleep", "3")
	}

	// Act
	err := WaitForCloudInit(log, 1)

	// Assert
	assert.Error(t, err, "Error should not be nil")
	assert.Contains(t, err.Error(), "timed out", "Error message should contain 'timed out'")

}

func TestKillProcessOnTimeout_DoesntSendKillSignal_WhenProcessIsDone(t *testing.T) {
	// Arrange
	log := mocklog.NewMockLog()
	command := exec.Command("sleep", "1")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	doneChan := make(chan struct{})
	killTimer := time.NewTimer(3 * time.Second)

	// Act
	command.Start()
	go killProcessGroupOnTimeout(log, command, killTimer, doneChan)
	err := command.Wait()
	doneChan <- struct{}{}
	close(doneChan)

	// Assert
	if timedOut := !killTimer.Stop(); timedOut {
		assert.Fail(t, "Process should not have timed out")
	}

	select {
	case _, ok := <-killTimer.C:
		if ok {
			assert.Fail(t, "Timer channel should have been flushed due to timeout")
		}
	default:
		break
	}

	assert.NoError(t, err, "Error should be nil")
}

func TestKillProcessOnTimeout_KillsProcessOnTimeout(t *testing.T) {
	// Arrange
	log := mocklog.NewMockLog()
	command := exec.Command("sleep", "3")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	doneChan := make(chan struct{})
	killTimer := time.NewTimer(1 * time.Second)

	// Act
	command.Start()
	go killProcessGroupOnTimeout(log, command, killTimer, doneChan)
	err := command.Wait()

	// Assert
	if timedOut := !killTimer.Stop(); !timedOut {
		assert.Fail(t, "Process should have timed out")
		doneChan <- struct{}{}
		close(doneChan)
	}

	select {
	case _, ok := <-killTimer.C:
		if ok {
			assert.Fail(t, "Timer channel should have been flushed due to timeout")
		}
	default:
		break
	}

	assert.Error(t, err, "Error should not be nil")
	assert.Contains(t, err.Error(), "signal: killed", "Error message should contain 'signal: killed'")
}

func TestWaitForCloudInit_WhenCloudInitDoesntExist_ReturnsError(t *testing.T) {
	// Arrange
	fileInfo := &MockFileInfo{}
	log := mocklog.NewMockLog()
	osStat = func(string) (os.FileInfo, error) {
		return fileInfo, &os.PathError{}
	}

	// Act
	err := WaitForCloudInit(log, 600)

	// Assert
	assert.Error(t, err, "Error should not be nil")
	assert.Contains(t, err.Error(), "cloud-init binary not found", "Error message should contain 'cloud-init binary not found'")
}

func TestWaitForCloudInit_WhenCloudInitNotFound_ReturnsError(t *testing.T) {
	// Arrange
	fileInfo := &MockFileInfo{}
	fileInfo.On("Mode").Return(os.FileMode(0500)).Repeatability = 0
	osStat = func(string) (os.FileInfo, error) {
		return fileInfo, nil
	}

	log := mocklog.NewMockLog()
	execCommand = func(command string, args ...string) *exec.Cmd {
		assert.Equal(t, cloudInitPath, command)
		assert.Equal(t, "status", args[0])
		assert.Equal(t, "--wait", args[1])
		log.Info("Running fake `cloud-init status --wait` command")
		return exec.Command("/tmp/path/shouldnt/exist", "status", "--wait")
	}

	// Act
	err := WaitForCloudInit(log, 600)

	// Assert
	assert.Error(t, err, "Error should not be nil")
	assert.Contains(t, err.Error(), "failed to start cloud-init command", "Error message should contain 'failed to start cloud-init command'")
}

func TestWaitForCloudInit_WhenCloudInitFailedExecution_ReturnsError(t *testing.T) {
	// Arrange
	fileInfo := &MockFileInfo{}
	fileInfo.On("Mode").Return(os.FileMode(0500)).Repeatability = 0
	osStat = func(string) (os.FileInfo, error) {
		return fileInfo, nil
	}

	log := mocklog.NewMockLog()
	execCommand = func(command string, args ...string) *exec.Cmd {
		assert.Equal(t, cloudInitPath, command)
		assert.Equal(t, "status", args[0])
		assert.Equal(t, "--wait", args[1])
		log.Info("Running fake `cloud-init status --wait` command")
		return exec.Command("ls", "bad_arg")
	}

	// Act
	err := WaitForCloudInit(log, 600)

	// Assert
	assert.Error(t, err, "Error should not be nil")
	assert.Contains(t, err.Error(), "error response from cloud-init command", "Error message should contain 'error response from cloud-init command'")
}

func TestWaitForCloudInit_WhenTimeoutIsZero_ReturnsError(t *testing.T) {
	for _, timeout := range []int{0, -1} {
		// Arrange
		log := mocklog.NewMockLog()
		// Act
		err := WaitForCloudInit(log, timeout)
		// Assert
		assert.Error(t, err, "Error should not be nil")
		assert.Contains(t, err.Error(), "invalid timeout value", "Error message should contain 'invalid timeout value'")
	}
}
