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

// +build windows

// package sharedCredentials provides access to the aws shared credentials file.
package sharedCredentials

import (
	"os"
)

func getPlatformSpecificHomeLocation() string {
	// Look for credentials in the following order
	// 1. AWS_SHARED_CREDENTIALS_FILE
	// 2. USERPROFILE environment variable
	//
	// Platform specific directories
	// Windows:   "%USERPROFILE%\.aws\credentials"
	return os.Getenv("USERPROFILE")
}
