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

package diagnostics

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/cli/diagnosticsutil"
)

const (
	systemctlServiceStopExitCode     = 3
	systemctlServiceNotFoundExitCode = 4
	upstartServiceNotFoundExitCode   = 1

	commandNotFoundExitCode = 0

	serviceManagerTimeoutSeconds = 2

	serviceCheckStrAgentNotRunningSnap         = "Agent is installed as snap service but is not running"
	serviceCheckStrSystemctlUnexpectedExitCode = "Unexpected exit code from systemctl status: %v"
	serviceCheckStrSystemctlTimeout            = "Command timeout when requesting systemctl status"
	serviceCheckStrSystemctlUnexpectedError    = "Unexpected error from systemctl status: %v"
	serviceCheckStrAgentNotRunningSystemctl    = "Agent is installed as a systemctl service but is not running"
	serviceCheckStrUpstartUnexpectedExitCode   = "Unexpected exit code from upstart status: %v"
	serviceCheckStrUpstartTimeout              = "Command timeout when requesting upstart status"
	serviceCheckStrUpstartUnexpectedError      = "Unexpected error from upstart status: %v"
	serviceCheckStrUpstartNotRunningUpstart    = "Agent is installed as a upstart service but is not running"
	serviceCheckStrUpstartUnexpectedOutput     = "Received unexpected output from upstart: %s"
	serviceCheckStrCantFindAgent               = "Unable to find ssm agent on this instance"
)

func isServiceRunningAsSnap() (bool, error) {
	// Check if agent is running in snap
	if diagnosticsutil.IsAgentInstalledSnap() {
		// Agent is installed as snap, check if service is running
		_, err := diagnosticsutil.ExecuteCommandWithTimeout(serviceManagerTimeoutSeconds*time.Second, "systemctl", "status", "snap.amazon-ssm-agent.amazon-ssm-agent.service")
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				if exitError.ExitCode() == systemctlServiceStopExitCode {
					return false, fmt.Errorf(serviceCheckStrAgentNotRunningSnap)
				} else if exitError.ExitCode() == commandNotFoundExitCode || exitError.ExitCode() == systemctlServiceNotFoundExitCode {
					// systemctl is not installed on the instance or amazon-ssm-agent snap service is not in systemctl
					return false, nil
				} else {
					return false, fmt.Errorf(serviceCheckStrSystemctlUnexpectedExitCode, exitError.ExitCode())
				}
			} else if err == context.DeadlineExceeded {
				return false, fmt.Errorf(serviceCheckStrSystemctlTimeout)
			} else {
				return false, fmt.Errorf(serviceCheckStrSystemctlUnexpectedError, err)
			}
		} else {
			// service is installed as snap and is running
			return true, nil
		}
	} else {
		// service is not installed as snap
		return false, nil
	}
}

func isServiceRunningSystemctl() (bool, error) {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return false, nil
	}
	_, err := diagnosticsutil.ExecuteCommandWithTimeout(serviceManagerTimeoutSeconds*time.Second, "systemctl", "status", "amazon-ssm-agent.service")
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == systemctlServiceStopExitCode {
				return false, fmt.Errorf(serviceCheckStrAgentNotRunningSystemctl)
			} else if exitError.ExitCode() == systemctlServiceNotFoundExitCode {
				// systemctl is not installed on the instance or amazon-ssm-agent service is not in systemctl
				return false, nil
			} else {
				return false, fmt.Errorf(serviceCheckStrSystemctlUnexpectedExitCode, exitError.ExitCode())
			}
		} else if err == context.DeadlineExceeded {
			return false, fmt.Errorf(serviceCheckStrSystemctlTimeout)
		} else {
			return false, fmt.Errorf(serviceCheckStrSystemctlUnexpectedError, err)
		}
	} else {
		// service is installed as systemctl service and is running
		return true, nil
	}
}

func isServiceRunningUpstart() (bool, error) {
	if _, err := exec.LookPath("status"); err != nil {
		return false, nil
	}
	output, err := diagnosticsutil.ExecuteCommandWithTimeout(serviceManagerTimeoutSeconds*time.Second, "status", "amazon-ssm-agent")

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == upstartServiceNotFoundExitCode {
				// upstart is not installed or the amazon-ssm-agent is not in upstart
				return false, nil
			}
			return false, fmt.Errorf(serviceCheckStrUpstartUnexpectedExitCode, exitError.ExitCode())
		} else if err == context.DeadlineExceeded {
			return false, fmt.Errorf(serviceCheckStrUpstartTimeout)
		}
		return false, fmt.Errorf(serviceCheckStrUpstartUnexpectedError, err)
	}

	// upstart returns exit code 0 when service is running or stopped
	if strings.Contains(output, "start/running") {
		// Agent is running as upstart service
		return true, nil
	} else if strings.Contains(string(output), "stop/waiting") {
		return false, fmt.Errorf(serviceCheckStrUpstartNotRunningUpstart)
	}

	return false, fmt.Errorf(serviceCheckStrUpstartUnexpectedOutput, output)
}

func isServiceRunning() error {

	isRunningSnap, snapErr := isServiceRunningAsSnap()
	// Failed to confirm agent is running as snap, return error
	if snapErr != nil {
		return snapErr
	}

	// If no error, check if agent is running as snap, if it is return nil
	if isRunningSnap {
		return nil
	}

	// Check if agent is running in systemctl
	isRunningSystemctl, systemctlErr := isServiceRunningSystemctl()
	// Failed to confirm agent is running as systemctl, return error
	if systemctlErr != nil {
		return systemctlErr
	}

	// If no error, check if agent is running as systemctl, if it is return nil
	if isRunningSystemctl {
		return nil
	}

	// Check if agent is running in systemd
	isRunningSystemd, systemdErr := isServiceRunningUpstart()
	// Failed to confirm agent is running as systemd, return error
	if systemdErr != nil {
		return systemdErr
	}

	// If no error, check if agent is running as systemctl, if it is return nil
	if isRunningSystemd {
		return nil
	}

	return fmt.Errorf(serviceCheckStrCantFindAgent)
}
