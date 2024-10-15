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

//go:build darwin
// +build darwin

package diagnostics

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

const (
	darwinExitCodeServiceNotFound = 113

	serviceCheckStrServiceNotFound    = "Service not found in launchctl"
	serviceCheckStrUnexpectedExitCode = "Unexpected exit code from launchctl: %v"
	serviceCheckStrCommandTimeout     = "Command timeout when requesting launchctl status"
	serviceCheckStrUnexpectedError    = "Unexpected error from launchctl: %v"
)

func isServiceRunning() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := exec.CommandContext(ctx, "launchctl", "list", "com.amazon.aws.ssm").Output()

	// Check if error is ExitError
	if exitError, ok := err.(*exec.ExitError); ok {
		if exitError.ExitCode() == darwinExitCodeServiceNotFound {
			return fmt.Errorf(serviceCheckStrServiceNotFound)
		}
		return fmt.Errorf(serviceCheckStrUnexpectedExitCode, exitError.ExitCode())
	} else if err == context.DeadlineExceeded {
		return fmt.Errorf(serviceCheckStrCommandTimeout)
	}

	if err != nil {
		return fmt.Errorf(serviceCheckStrUnexpectedError, err)
	}

	return nil
}
