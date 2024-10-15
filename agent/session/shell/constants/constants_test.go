// Copyright 2022 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package constants manages the configuration of the session shell.
package constants

import (
	"strings"
	"testing"

	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/stretchr/testify/assert"
)

var (
	shellProps = mgsContracts.ShellProperties{
		Linux: mgsContracts.ShellConfig{
			Commands:              "ls",
			RunAsElevated:         false,
			SeparateOutputStream:  true,
			StdOutSeparatorPrefix: "STD_OUT:",
			StdErrSeparatorPrefix: "STD_ERR:",
		},
		Windows: mgsContracts.ShellConfig{
			Commands:              "ls",
			RunAsElevated:         false,
			SeparateOutputStream:  true,
			StdOutSeparatorPrefix: "STD_OUT:",
			StdErrSeparatorPrefix: "STD_ERR:",
		},
		MacOS: mgsContracts.ShellConfig{
			Commands:              "ls",
			RunAsElevated:         false,
			SeparateOutputStream:  true,
			StdOutSeparatorPrefix: "STD_OUT:",
			StdErrSeparatorPrefix: "STD_ERR:",
		},
	}
)

// Testing GetShellCommand
func TestGetShellCommand(t *testing.T) {
	command := GetShellCommand(shellProps)
	assert.Equal(t, command, "ls")
}

// Testing GetRunAsElevated
func TestGetRunAsElevated(t *testing.T) {
	runAsElevated := GetRunAsElevated(shellProps)
	assert.Equal(t, runAsElevated, false)
}

// Testing GetSeparateOutputStream
func TestGetSeparateOutputStream(t *testing.T) {
	separateOutputStream, err := GetSeparateOutputStream(shellProps)
	assert.Nil(t, err)
	assert.Equal(t, separateOutputStream, true)
}

// Testing GetSeparateOutputStream with string type
func TestGetSeparateOutputStreamWithStringType(t *testing.T) {
	shellPropsWithStringTypeFeatureValue := mgsContracts.ShellProperties{
		Linux: mgsContracts.ShellConfig{
			Commands:             "ls",
			RunAsElevated:        false,
			SeparateOutputStream: "true",
		},
		Windows: mgsContracts.ShellConfig{
			Commands:             "ls",
			RunAsElevated:        false,
			SeparateOutputStream: "true",
		},
		MacOS: mgsContracts.ShellConfig{
			Commands:             "ls",
			RunAsElevated:        false,
			SeparateOutputStream: "true",
		},
	}

	separateOutputStream, err := GetSeparateOutputStream(shellPropsWithStringTypeFeatureValue)
	assert.Nil(t, err)
	assert.Equal(t, separateOutputStream, true)
}

// Testing GetSeparateOutputStream
func TestGetSeparateOutputStreamWithError(t *testing.T) {
	errShellProps := mgsContracts.ShellProperties{
		Linux: mgsContracts.ShellConfig{
			Commands:             "ls",
			RunAsElevated:        false,
			SeparateOutputStream: "error",
		},
		Windows: mgsContracts.ShellConfig{
			Commands:             "ls",
			RunAsElevated:        false,
			SeparateOutputStream: "error",
		},
		MacOS: mgsContracts.ShellConfig{
			Commands:             "ls",
			RunAsElevated:        false,
			SeparateOutputStream: "error",
		},
	}

	_, err := GetSeparateOutputStream(errShellProps)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "unable to convert separateOutPutStream:"))
}

// Testing GetStdOutSeparatorPrefix
func TestGetStdOutSeparatorPrefix(t *testing.T) {
	stdOutSeparatorPrefix := GetStdOutSeparatorPrefix(shellProps)
	assert.Equal(t, stdOutSeparatorPrefix, "STD_OUT:")
}

// Testing GetStdErrSeparatorPrefix
func TestGetStdErrSeparatorPrefix(t *testing.T) {
	stdOutSeparatorPrefix := GetStdErrSeparatorPrefix(shellProps)
	assert.Equal(t, stdOutSeparatorPrefix, "STD_ERR:")
}
