// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
//
// +build darwin freebsd linux netbsd openbsd

// utility package implements all the shared methods between clients.
package utility

import (
	"fmt"
	"os/exec"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/session/plugins/shell"
)

// ResetPasswordIfDefaultUserExists resets default RunAs user password if user exists
func (u *SessionUtil) ResetPasswordIfDefaultUserExists(context context.T) (err error) {
	// Do nothing here as no password is required for unix platform local user
	return nil
}

// DoesUserExist checks if given user already exists
func (u *SessionUtil) DoesUserExist(username string) (bool, error) {
	shellCmdArgs := append(shell.ShellPluginCommandArgs, fmt.Sprintf("id %s", username))
	cmd := exec.Command(shell.ShellPluginCommandName, shellCmdArgs...)
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// The program has exited with an exit code != 0
			return false, fmt.Errorf("encountered an error while checking for %s: %v", appconfig.DefaultRunAsUserName, exitErr.Error())
		}
		return false, nil
	}
	return true, nil
}
