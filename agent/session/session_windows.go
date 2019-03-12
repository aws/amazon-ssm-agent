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
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/session/utility"
)

// createLocalAdminUser creates a local OS user on the instance with admin permissions.
func (s *Session) createLocalAdminUser() (err error) {
	log := s.context.Log()

	u := &utility.SessionUtil{}
	var newPassword string
	if newPassword, err = u.GeneratePasswordForDefaultUser(); err != nil {
		return
	}

	var userExists bool
	if userExists, err = u.AddNewUser(appconfig.DefaultRunAsUserName, newPassword); err != nil {
		return fmt.Errorf("Failed to create %s: %v", appconfig.DefaultRunAsUserName, err)
	}

	if userExists {
		log.Infof("%s already exists.", appconfig.DefaultRunAsUserName)
		return
	}
	log.Infof("Successfully created %s", appconfig.DefaultRunAsUserName)

	var adminGroupName string
	if adminGroupName, err = u.AddUserToLocalAdministratorsGroup(appconfig.DefaultRunAsUserName); err != nil {
		return fmt.Errorf("Failed to add %s to local admin group: %v", appconfig.DefaultRunAsUserName, err)
	}
	log.Infof("Added %s to %s group", appconfig.DefaultRunAsUserName, adminGroupName)

	return
}
