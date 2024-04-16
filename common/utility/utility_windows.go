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

//go:build windows
// +build windows

package utility

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
)

const (
	defaultCommandTimeOut = 30 * time.Second
)

var (
	executePowershellCommandWithTimeoutFunc = executePowershellCommandWithTimeout
)

var powershellArgs = []string{"-InputFormat", "None", "-Noninteractive", "-NoProfile", "-ExecutionPolicy", "unrestricted"}

// IsRunningElevatedPermissions checks if current user is administrator
func IsRunningElevatedPermissions() error {
	checkAdminCmd := `([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] 'Administrator')`
	isAdminTrue := "True"
	isAdminFalse := "False"

	output, err := executePowershellCommandWithTimeoutFunc(defaultCommandTimeOut, checkAdminCmd)
	if err != nil {
		return fmt.Errorf("failed to check permissions: %v", err)
	}

	if output == isAdminTrue {
		return nil
	} else if output == isAdminFalse {
		return fmt.Errorf("binary needs to be executed by administrator")
	} else {
		return fmt.Errorf("unexpected permission check output: %v", output)
	}
}

func executePowershellCommandWithTimeout(timeout time.Duration, command string) (string, error) {
	args := append(powershellArgs, "-Command", command)
	return executeCommandWithTimeout(timeout, appconfig.PowerShellPluginCommandName, args...)
}

func executeCommandWithTimeout(timeout time.Duration, cmd string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	byteArr, err := exec.CommandContext(ctx, cmd, args...).Output()
	output := strings.TrimSpace(string(byteArr))

	return output, err
}
