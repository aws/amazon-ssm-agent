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
	"os/exec"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/session/utility"
)

const administrators = "administrators"

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
		if err := s.createUser(); err != nil {
			return err
		}
	}
	return nil
}

// createUser creates an OS local user and adds it to Administrators group.
func (s *Session) createUser() error {
	log := s.context.Log()

	// Create local user
	commandArgs := []string{"net", "user", "/add", appconfig.DefaultRunAsUserName}
	cmd := exec.Command(appconfig.PowerShellPluginCommandName, commandArgs...)
	if err := cmd.Run(); err != nil {
		log.Errorf("Failed to create %s: %v", appconfig.DefaultRunAsUserName, err)
		return err
	}
	log.Infof("Successfully created %s", appconfig.DefaultRunAsUserName)

	// Add to admins group
	commandArgs = []string{"net", "localgroup", administrators, appconfig.DefaultRunAsUserName, "/add"}
	cmd = exec.Command(appconfig.PowerShellPluginCommandName, commandArgs...)
	if err := cmd.Run(); err != nil {
		log.Errorf("Failed to add %s to %s group: %v", appconfig.DefaultRunAsUserName, administrators, err)
		return err
	}
	log.Infof("Successfully added %s to %s group", appconfig.DefaultRunAsUserName, administrators)
	return nil
}
