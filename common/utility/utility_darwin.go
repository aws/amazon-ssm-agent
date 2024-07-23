// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package utility

import (
	"fmt"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"os/user"
)

var (
	userCurrent = user.Current
)

const (
	// ExpectedServiceRunningUser is the user we expect the agent to be running as
	ExpectedServiceRunningUser = "root"
)

// WaitForCloudInit is a no-op on darwin and returns nil
func WaitForCloudInit(log log.T, timeoutSeconds int) error {
	return nil
}

// IsRunningElevatedPermissions checks if current user is administrator
func IsRunningElevatedPermissions() error {
	currentUser, err := userCurrent()
	if err != nil {
		return err
	}

	if currentUser.Username == ExpectedServiceRunningUser {
		return nil
	} else {
		return fmt.Errorf("binary needs to be executed by %s", ExpectedServiceRunningUser)
	}
}
