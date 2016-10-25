// +build windows

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
//
//
// Package rundaemon implements rundaemon plugin and its configuration
package rundaemon

import (
	"strings"
)

const (
	// PowershellArgs specifies the default arguments that we pass to powershell
	// Use Unrestricted as Execution Policy for running the script.
	// https://technet.microsoft.com/en-us/library/hh847748.aspx
	// The reason that PowerShellArgs is being redefined and not leveraged as is from existing
	// utilities is because the -f option doesnt work with exe. The below options will work
	// for both exe and powershell .
	PowerShellArgsForExe = "-InputFormat None -Noninteractive -NoProfile -ExecutionPolicy unrestricted"
)

func GetShellArguments() []string {
	return strings.Split(PowerShellArgsForExe, " ")
}
