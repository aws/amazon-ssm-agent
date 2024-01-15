// Copyright 2023 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

//go:build freebsd || linux || netbsd || openbsd || darwin
// +build freebsd linux netbsd openbsd darwin

// Package common contains common constants and functions needed to be accessed across ssm-setup-cli
package common

import (
	"testing"

	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// Define ExecHelper TestSuite struct
type ExecHelperLinuxTestSuite struct {
	suite.Suite
	logMock *logmocks.Mock
}

// Initialize the ExecHelperLinux test suite struct
func (suite *ExecHelperLinuxTestSuite) SetupTest() {
	logMock := logmocks.NewMockLog()
	suite.logMock = logMock
}

// Test function for Exec Helper
func (suite *ExecHelperLinuxTestSuite) TestExecHelper_SetPlatformSpecificCommand() {
	expectedOutput := []string{"arg1", "arg2"}
	actualOutput := setPlatformSpecificCommand(expectedOutput)
	assert.Equal(suite.T(), expectedOutput, actualOutput, "setPlatformSpecificCommand contains mismatched output")
}

func TestExecHelperLinuxTestSuite(t *testing.T) {
	suite.Run(t, new(ExecHelperLinuxTestSuite))
}
