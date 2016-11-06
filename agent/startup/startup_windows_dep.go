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

// Package startup implements startup plugin processor
package startup

import "os/exec"

var cmdExec cmdExecutor = &cmdExecutorImp{}

type cmdExecutor interface {
	ExecuteCommand(command string, args ...string) ([]byte, error)
}

type cmdExecutorImp struct{}

// ExecuteCommand is a wrapper of executes exec.Command
func (cmdExecutorImp) ExecuteCommand(command string, args ...string) ([]byte, error) {
	// decoupling exec.Command for easy testability
	return exec.Command(command, args...).CombinedOutput()
}
