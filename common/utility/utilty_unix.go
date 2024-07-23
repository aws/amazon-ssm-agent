// Copyright 2024 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package utility

import (
	"fmt"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"os"
	"os/exec"
	"os/user"
	"runtime/debug"
	"strings"
	"syscall"
	"time"
)

const (
	// ExpectedServiceRunningUser is the user we expect the agent to be running as
	ExpectedServiceRunningUser = "root"
	cloudInitPath              = "/usr/bin/cloud-init"
)

var (
	userCurrent = user.Current
	osStat      = os.Stat
	execCommand = exec.Command
)

func killProcessGroupOnTimeout(log log.T, command *exec.Cmd, timer *time.Timer, doneChan chan struct{}) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("kill process on timeout panic: \n%v", r)
			log.Errorf("stacktrace:\n%s", debug.Stack())
		}
	}()

	select {
	case <-timer.C:
		if command.Process == nil {
			log.Warn("process already exited")
			return
		}

		log.Errorf("timeout reached, killing process %v", command.Process.Pid)
		if err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL); err != nil {
			log.Errorf("failed to kill process %v. Err: %v", command.Process.Pid, err)
		}

		log.Tracef("process %v killed", command.Process.Pid)
	case <-doneChan:
		// Drain channel
		<-timer.C
		return
	}

}

func sigTermProcessGroupOnTimeout(log log.T, command *exec.Cmd, timer *time.Timer, doneChan chan struct{}) {
	select {
	case <-timer.C:
		if command.Process == nil {
			log.Warn("process already exited")
			return
		}

		log.Errorf("process %v timed out, sending SIGTERM", command.Process.Pid)
	case <-doneChan:
		// Drain channel
		<-timer.C
		return
	}

	if err := syscall.Kill(-command.Process.Pid, syscall.SIGTERM); err != nil {
		log.Errorf("failed to send SIGTERM to pid %v. Killing process. Err: %v", command.Process.Pid, err)
		if err := command.Process.Kill(); err != nil {
			log.Errorf("failed to kill process %v. Err: %v", command.Process.Pid, err)
			return
		}
	}

	killTimeout := 30 * time.Second
	killTimer := time.NewTimer(killTimeout)
	go killProcessGroupOnTimeout(log, command, killTimer, doneChan)
}

// WaitForCloudInit waits for cloud-init to complete
func WaitForCloudInit(log log.T, timeoutSeconds int) error {
	if timeoutSeconds <= 0 {
		return fmt.Errorf("invalid timeout value %d", timeoutSeconds)
	}

	// Check if cloud-init is installed and executable
	if fileInfo, err := osStat(cloudInitPath); err != nil {
		return fmt.Errorf("cloud-init binary not found at %s. Err: %w", cloudInitPath, err)
	} else if fileInfo.Mode().Perm()&0111 == 0 {
		return fmt.Errorf("cloud-init binary is found but not executable")
	} else {
		// Wait for cloud-init
		log.Debug("Waiting for cloud-init completion...")
		command := execCommand(cloudInitPath, "status", "--wait")
		// Set process group ID so the command and all its children become a new
		// process group and all sub-process children can be sent SIGTERM and SIGKILL signals
		// Group ID is set to the process ID of the command
		command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		if err := command.Start(); err != nil || command.Process == nil {
			return fmt.Errorf("failed to start cloud-init command. Err: %v", err)
		}

		timer := time.NewTimer(time.Duration(timeoutSeconds) * time.Second)
		var doneChan = make(chan struct{})
		go sigTermProcessGroupOnTimeout(log, command, timer, doneChan)
		err = command.Wait()
		timedOut := !timer.Stop()
		if !timedOut {
			doneChan <- struct{}{}
			close(doneChan)
		}

		if err == nil {
			log.Debug("cloud-init is finished execution.")
			return nil
		} else if strings.Contains(err.Error(), "no child processes") {
			return fmt.Errorf("cloud-init timed out after %d seconds", timeoutSeconds)
		}

		log.Debugf("command returned error %v", err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			// The program has exited with an exit code != 0
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				exitCode := status.ExitStatus()
				if exitCode == -1 && timedOut {
					return fmt.Errorf("timed out waiting for cloud-init")
				}
			}
		}

		return fmt.Errorf("error response from cloud-init command: %w", err)
	}
}

// IsRunningElevatedPermissions checks if current user is administrator
func IsRunningElevatedPermissions() error {
	currentUser, err := userCurrent()
	if err != nil {
		return err
	}

	if currentUser.Username == ExpectedServiceRunningUser {
		return nil
	} else {
		return fmt.Errorf("binary needs to be executed by %s", ExpectedServiceRunningUser)
	}
}
