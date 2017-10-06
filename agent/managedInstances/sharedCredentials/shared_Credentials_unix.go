// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// +build darwin freebsd linux netbsd openbsd

// package sharedCredentials provides access to the aws shared credentials file.
package sharedCredentials

import (
	"os"
	"os/user"

	alt_user "github.com/aws/amazon-ssm-agent/agent/user"
)

func getPlatformSpecificHomeLocation() string {
	// Look for credentials in the following order
	// 1. AWS_SHARED_CREDENTIALS_FILE
	// 2. HOME environment variable (for backward compatibility)
	// 3. Current user's home directory
	//
	// Platform specific directories
	// Linux/OSX: "$HOME/.aws/credentials"

	// 1. get it from $HOME
	homeDir := os.Getenv("HOME")
	if homeDir != "" {
		return homeDir
	}

	// 2. use cgo
	usr, err := user.Current()
	if err == nil && usr.HomeDir != "" {
		return usr.HomeDir
	}

	// 3. use own implementation
	usr, err = alt_user.Current()
	if err == nil && usr.HomeDir != "" {
		return usr.HomeDir
	}

	return ""
}
