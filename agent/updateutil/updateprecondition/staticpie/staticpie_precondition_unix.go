// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// +build freebsd linux netbsd openbsd

package staticpieprecondition

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/versionutil"
)

var execCommand = exec.Command

var hasValidKernelVersion = func() error {

	byteOutput, err := execCommand("uname", "-r").Output()

	if err != nil {
		return err
	}

	splitVersion := strings.Split(strings.TrimSpace(string(byteOutput)), ".")

	// Expecting at least major, minor, path
	if len(splitVersion) < 3 {
		return fmt.Errorf("Unexpected kernel version format: %s", byteOutput)
	}

	// Join major + minor version
	kernelVersion := strings.Join(splitVersion[:2], ".")

	comp, err := versionutil.VersionCompare(kernelVersion, minLinuxKernelVersion)

	if err != nil {
		return err
	}

	if comp < 0 {
		return fmt.Errorf("Minimum kernel version is %s but instance kernel version is %s", minLinuxKernelVersion, kernelVersion)
	}

	return nil
}
