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
// +build windows

// Package session implements the core module to start web-socket connection with message gateway service.
package session

import (
	"bufio"
	"os/exec"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/session/utility"
)

const administrators = "administrators"

var u = &utility.SessionUtil{}

var commandName = appconfig.PowerShellPluginCommandName
var commandArgs = []string{"net", "user", "/add", appconfig.DefaultRunAsUserName, u.MustGeneratePasswordForDefaultUser(), "/Y"}

// createLocalAdminUser creates a local OS user on the instance with admin permissions.
func (s *Session) createLocalAdminUser() {
	// If error occurred, ignore adding/re-adding the ssm-user to administrators group. (Windows only)
	if err := s.createLocalUser(); err != nil {
		// Reset the password if ssm-user is already created.
		if strings.Contains(err.Error(), "already exists") {
			log := s.context.Log()
			log.Infof("Resetting password for %s", appconfig.DefaultRunAsUserName)
			if err = exec.Command(appconfig.PowerShellPluginCommandName, "net", "user", appconfig.DefaultRunAsUserName, u.MustGeneratePasswordForDefaultUser()).Run(); err != nil {
				panic(err)
			}
		}
		return
	}

	s.addUserToOSAdminGroup()
}

// addUserToOSAdminGroup will add user to OS specific admin group.
func (s *Session) addUserToOSAdminGroup() {
	log := s.context.Log()

	cmd := exec.Command(commandName, "net", "localgroup", administrators, appconfig.DefaultRunAsUserName, "/add")
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Errorf("Error occurred while adding %s to %s group: %v", appconfig.DefaultRunAsUserName, administrators, err)
		return
	}

	if err = cmd.Start(); err != nil {
		log.Errorf("Error occurred starting the command: %v", err)
		return
	}

	scanner := bufio.NewScanner(stderr)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "account name is already a member of the group") {
			log.Infof("%s is already a member of %s group.", appconfig.DefaultRunAsUserName, administrators)

			// Release all resources
			cmd.Wait()

			return
		}
	}

	if err = cmd.Wait(); err != nil {
		log.Errorf("Failed to add %s to %s group: %v", appconfig.DefaultRunAsUserName, administrators, err)
		return
	}

	log.Infof("Successfully added %s to %s group", appconfig.DefaultRunAsUserName, administrators)
}
