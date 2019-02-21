// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package session implements the core module to start web-socket connection with message gateway service.
package session

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/session/plugins/shell"
	"github.com/aws/amazon-ssm-agent/agent/session/utility"
)

const sudoersFile = "/etc/sudoers.d/ssm-agent-users"
const sudoersFileMode = 0440

// createLocalAdminUser creates a local OS user on the instance with admin permissions.
func (s *Session) createLocalAdminUser() error {
	log := s.context.Log()

	u := &utility.SessionUtil{}
	userExists, err := u.DoesUserExist(appconfig.DefaultRunAsUserName)
	if err != nil {
		log.Errorf("Error occurred while checking if %s user exists, %v", appconfig.DefaultRunAsUserName, err)
		return err
	}

	if userExists {
		log.Infof("%s already exists.", appconfig.DefaultRunAsUserName)
	} else {
		if err := s.createLocalUser(); err != nil {
			return err
		}
	}

	if err := s.createSudoersFileIfNotPresent(); err != nil {
		return err
	}
	return nil
}

// createLocalUser creates an OS local user.
func (s *Session) createLocalUser() error {
	log := s.context.Log()

	commandArgs := append(shell.ShellPluginCommandArgs, fmt.Sprintf("useradd -m %s", appconfig.DefaultRunAsUserName))
	cmd := exec.Command(shell.ShellPluginCommandName, commandArgs...)
	if err := cmd.Run(); err != nil {
		log.Errorf("Failed to create %s: %v", appconfig.DefaultRunAsUserName, err)
		return err
	}
	log.Infof("Successfully created %s", appconfig.DefaultRunAsUserName)
	return nil
}

// createSudoersFileIfNotPresent will create the sudoers file if not present.
func (s *Session) createSudoersFileIfNotPresent() error {
	log := s.context.Log()

	// Return if the file exists
	if _, err := os.Stat(sudoersFile); err == nil {
		log.Infof("File %s already exists", sudoersFile)
		s.changeModeOfSudoersFile()
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
	s.changeModeOfSudoersFile()
	return nil
}

// changeModeOfSudoersFile will change the sudoersFile mode to 0440 (read only).
// This file is created with mode 0666 using os.Create() so needs to be updated to read only with chmod.
func (s *Session) changeModeOfSudoersFile() error {
	log := s.context.Log()
	fileMode := os.FileMode(sudoersFileMode)
	if err := os.Chmod(sudoersFile, fileMode); err != nil {
		log.Errorf("Failed to change mode of %s to %d: %v", sudoersFile, sudoersFileMode, err)
		return err
	}
	log.Infof("Successfully changed mode of %s to %d", sudoersFile, sudoersFileMode)
	return nil
}
