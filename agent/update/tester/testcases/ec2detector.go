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

// Package testcases contains test cases from all testStages
package testcases

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	testCommon "github.com/aws/amazon-ssm-agent/agent/update/tester/common"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2/ec2detector"
)

var ec2DetectorTestCaseName = "Ec2Detector"

// Ec2DetectorTestCase represents the test case testing the ec2 detection module in ec2 environments
type Ec2DetectorTestCase struct {
	context  context.T
	detector ec2detector.Ec2Detector
}

// Initialize initializes values needed for this test case
func (l *Ec2DetectorTestCase) Initialize() {
	l.context = l.context.With("[Test" + l.GetTestCaseName() + "]")
	l.detector = ec2detector.New(l.context.AppConfig())
}

// ExecuteTestCase executes the ec2 detector test case
// test only runs when instance id starts with i-
func (l *Ec2DetectorTestCase) ExecuteTestCase() testCommon.TestOutput {
	var output testCommon.TestOutput

	output.Result = testCommon.TestCaseFail
	if l.detector.IsEC2Instance() {
		output.Result = testCommon.TestCasePass
	}

	return output
}

// GetTestCaseName gets the test case name
func (l *Ec2DetectorTestCase) GetTestCaseName() string {
	return ec2DetectorTestCaseName
}

// CleanupTestCase cleans up the test case
func (l *Ec2DetectorTestCase) CleanupTestCase() {
}

func NewEc2DetectorTestCase(context context.T) *Ec2DetectorTestCase {
	return &Ec2DetectorTestCase{
		context: context,
	}
}
