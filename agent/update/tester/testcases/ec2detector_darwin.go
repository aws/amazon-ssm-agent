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
// permissions and limitations under the License

//go:build darwin
// +build darwin

package testcases

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	testCommon "github.com/aws/amazon-ssm-agent/agent/update/tester/common"
)

var ec2DetectorTestCaseName = "Ec2DetectorDarwin"

// Ec2DetectorTestCase represents the test case testing the ec2 detection module in ec2 environments
type Ec2DetectorTestCase struct {
	context context.T
}

// ShouldRunTest determines if test should run
func (l *Ec2DetectorTestCase) ShouldRunTest() bool {
	// ec2detector is currently not supported on darwin
	return false
}

// ExecuteTestCase executes the ec2 detector test case
// test only runs when instance id starts with i-
func (l *Ec2DetectorTestCase) ExecuteTestCase() testCommon.TestOutput {
	// Test should not be executed on darwin
	return testCommon.TestOutput{}
}
