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

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/session/plugins/shell"
)

const sudoersFile = "/etc/sudoers.d/ssm-agent-users"

var commandName = shell.ShellPluginCommandName
var commandArgs = append(shell.ShellPluginCommandArgs, fmt.Sprintf("useradd -m %s", appconfig.DefaultRunAsUserName))

// createLocalAdminUser creates a local OS user on the instance with admin permissions.
func (s *Session) createLocalAdminUser() {
	s.createLocalUser()
	s.addUserToOSAdminGroup()
}

// addUserToOSAdminGroup will add user to OS specific admin group.
func (s *Session) addUserToOSAdminGroup() {
	log := s.context.Log()

	// Return if the file exists
	if _, err := os.Stat(sudoersFile); err == nil {
		log.Infof("File %s already exists", sudoersFile)
		return
	}

	// Create a sudoers file for ssm-user
	file, err := os.Create(sudoersFile)
	if err != nil {
		log.Errorf("Failed to add %s to sudoers file: %v", appconfig.DefaultRunAsUserName, err)
		return
	}
	defer file.Close()
	// Set permissions for sudoers file
	if chmod, err := os.Chmod(sudoersFile, 0440); err == nil {
		log.Infof("Updated permissions for %s ", sudoersFile)
		return
	}

	file.WriteString(fmt.Sprintf("# User rules for %s\n", appconfig.DefaultRunAsUserName))
	file.WriteString(fmt.Sprintf("%s ALL=(ALL) NOPASSWD:ALL\n", appconfig.DefaultRunAsUserName))
	log.Infof("Successfully created file %s", sudoersFile)
}
