// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

//go:build windows
// +build windows

// Package executor wraps up the os.Process interface and also provides os-specific process lookup functions
package executor

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TODO add process start time
func TestIsProcessExists(t *testing.T) {
	cmd := exec.Command("cmd", "timeout", "5")
	err := cmd.Start()
	assert.NoError(t, err)
	pid := cmd.Process.Pid
	ppid := os.Getpid()

	processes, err := getProcess()
	found := false
	for _, process := range processes {
		if process.Pid == pid && process.PPid == ppid {
			found = true
		}
	}

	assert.True(t, found)
}
