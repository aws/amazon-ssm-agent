// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License..

// +build darwin freebsd linux netbsd openbsd

// This file has unit tests to test the unix based functions
package executor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppendArgs_Linux(t *testing.T) {
	shellArgs := []string{"-c"}
	scriptArgs := []string{"args1", "args2"}

	commandArguments := appendArgs(shellArgs, scriptArgs, "file.py")

	expectedArgs := []string{"-c", "args1", "args2"}

	assert.Equal(t, expectedArgs, commandArguments)
}

func TestPopulateCommand_Linux(t *testing.T) {
	name, args := populateCommand("file.py")
	exp_args := []string{"-c"}
	assert.Equal(t, "file.py", name)
	assert.Equal(t, exp_args, args)
}
