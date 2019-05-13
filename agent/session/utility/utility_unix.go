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
	"os"
	"os/exec"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

var ShellPluginCommandName = "sh"
var ShellPluginCommandArgs = []string{"-c"}

const sudoersFile = "/etc/sudoers.d/ssm-agent-users"
const sudoersFileMode = 0440

// ResetPasswordIfDefaultUserExists resets default RunAs user password if user exists
func (u *SessionUtil) ResetPasswordIfDefaultUserExists(context context.T) (err error) {
	// Do nothing here as no password is required for unix platform local user
	return nil
}

// DoesUserExist checks if given user already exists
func (u *SessionUtil) DoesUserExist(username string) (bool, error) {
	shellCmdArgs := append(ShellPluginCommandArgs, fmt.Sprintf("id %s", username))
	cmd := exec.Command(ShellPluginCommandName, shellCmdArgs...)
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// The program has exited with an exit code != 0
			return false, fmt.Errorf("encountered an error while checking for %s: %v", appconfig.DefaultRunAsUserName, exitErr.Error())
		}
		return false, nil
	}
	return true, nil
}

// createLocalAdminUser creates a local OS user on the instance with admin permissions. The password will alway be empty
func (u *SessionUtil) CreateLocalAdminUser(log log.T) (newPassword string, err error) {

	userExists, _ := u.DoesUserExist(appconfig.DefaultRunAsUserName)

	if userExists {
		log.Infof("%s already exists.", appconfig.DefaultRunAsUserName)
	} else {
		if err = u.createLocalUser(log); err != nil {
			return
		}
		// only create sudoers file when user does not exist
		err = u.createSudoersFileIfNotPresent(log)
	}

	return
}

// createLocalUser creates an OS local user.
func (u *SessionUtil) createLocalUser(log log.T) error {

	commandArgs := append(ShellPluginCommandArgs, fmt.Sprintf("useradd -m %s", appconfig.DefaultRunAsUserName))
	cmd := exec.Command(ShellPluginCommandName, commandArgs...)
	if err := cmd.Run(); err != nil {
		log.Errorf("Failed to create %s: %v", appconfig.DefaultRunAsUserName, err)
		return err
	}
	log.Infof("Successfully created %s", appconfig.DefaultRunAsUserName)
	return nil
}

// createSudoersFileIfNotPresent will create the sudoers file if not present.
func (u *SessionUtil) createSudoersFileIfNotPresent(log log.T) error {

	// Return if the file exists
	if _, err := os.Stat(sudoersFile); err == nil {
		log.Infof("File %s already exists", sudoersFile)
		u.changeModeOfSudoersFile(log)
		return err
	}

	// Create a sudoers file for ssm-user
	file, err := os.Create(sudoersFile)
	if err != nil {
		log.Errorf("Failed to add %s to sudoers file: %v", appconfig.DefaultRunAsUserName, err)
		return err
	}
	defer file.Close()

	file.WriteString(fmt.Sprintf("# User rules for %s\n", appconfig.DefaultRunAsUserName))
	file.WriteString(fmt.Sprintf("%s ALL=(ALL) NOPASSWD:ALL\n", appconfig.DefaultRunAsUserName))
	log.Infof("Successfully created file %s", sudoersFile)
	u.changeModeOfSudoersFile(log)
	return nil
}

// changeModeOfSudoersFile will change the sudoersFile mode to 0440 (read only).
// This file is created with mode 0666 using os.Create() so needs to be updated to read only with chmod.
func (u *SessionUtil) changeModeOfSudoersFile(log log.T) error {
	fileMode := os.FileMode(sudoersFileMode)
	if err := os.Chmod(sudoersFile, fileMode); err != nil {
		log.Errorf("Failed to change mode of %s to %d: %v", sudoersFile, sudoersFileMode, err)
		return err
	}
	log.Infof("Successfully changed mode of %s to %d", sudoersFile, sudoersFileMode)
	return nil
}

func (u *SessionUtil) DisableLocalUser(log log.T) (err error) {
	// Do nothing here as no password is required for unix platform local user, so that no need to disable user.
	return nil
}
