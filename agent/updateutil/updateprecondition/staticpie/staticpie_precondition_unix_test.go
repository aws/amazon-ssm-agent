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

package staticpieprecondition

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func fakeExecCommand(stdOut string) func(string, ...string) *exec.Cmd {
	return func(string, ...string) *exec.Cmd {
		return exec.Command("echo", stdOut)
	}
}

func fakeExecCommandWithError(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestExecCommandHelperProcess", "-test.error", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func TestHasValidKernelVersion_ErrorExec(t *testing.T) {
	execCommand = fakeExecCommandWithError

	err := hasValidKernelVersion()
	assert.Error(t, err)
}

func TestHasValidKernelVersion_InvalidVersion(t *testing.T) {
	execCommand = fakeExecCommand("1.1")

	err := hasValidKernelVersion()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Unexpected kernel version format: 1.1")
}

func TestHasValidKernelVersion_ErrVersionCompare(t *testing.T) {
	execCommand = fakeExecCommand("!@#.!@#.!@#.!@#")

	err := hasValidKernelVersion()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid version string !@#.!@#")
}

func TestHasValidKernelVersion_LowerKernelVersion(t *testing.T) {
	execCommand = fakeExecCommand("3.0.95-47.164.amzn2int.x86_64")

	err := hasValidKernelVersion()
	assert.Error(t, err)
	assert.Equal(t, "Minimum kernel version is 3.2 but instance kernel version is 3.0", err.Error())
}

func TestHasValidKernelVersion_Success(t *testing.T) {
	execCommand = fakeExecCommand("5.4.95-47.164.amzn2int.x86_64")

	err := hasValidKernelVersion()
	assert.NoError(t, err)
}
