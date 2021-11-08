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

package common

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"
)

const defaultTimeout = 30 * time.Second

type IManagerHelper interface {
	// RunCommand executes command with timeout
	RunCommand(cmd string, args ...string) (string, error)
	// IsCommandAvailable checks if command is available on the host
	IsCommandAvailable(cmd string) bool
	// IsTimeoutError returns true if error is context timeout error
	IsTimeoutError(err error) bool
	// IsExitCodeError returns true if error is command exit code error
	IsExitCodeError(err error) bool
	// GetExitCode returns the exit code for of exit code error, defaults to -1 if error is not exit code error
	GetExitCode(err error) int
}

type ManagerHelper struct {
	Timeout time.Duration
}

func (m *ManagerHelper) RunCommand(cmd string, args ...string) (string, error) {
	if m.Timeout == time.Duration(0) {
		m.Timeout = defaultTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), m.Timeout)
	defer cancel()

	byteArr, err := exec.CommandContext(ctx, cmd, args...).Output()
	output := strings.TrimSpace(string(byteArr))

	return output, err
}

func (m *ManagerHelper) IsTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, context.DeadlineExceeded)
}

func (m *ManagerHelper) IsExitCodeError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*exec.ExitError)
	return ok
}

func (m *ManagerHelper) GetExitCode(err error) int {
	if !m.IsExitCodeError(err) {
		return -1
	}

	return err.(*exec.ExitError).ExitCode()
}

func (m *ManagerHelper) IsCommandAvailable(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func FileExists(filePath string) (bool, error) {
	_, err := os.Stat(filePath)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}
