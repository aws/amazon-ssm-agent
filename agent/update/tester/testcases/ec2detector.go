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
)

// Initialize initializes values needed for this test case
func (l *Ec2DetectorTestCase) Initialize() {
	l.context = l.context.With("[Test" + l.GetTestCaseName() + "]")
}

// GetTestCaseName gets the test case name
func (l *Ec2DetectorTestCase) GetTestCaseName() string {
	return ec2DetectorTestCaseName
}

// CleanupTestCase cleans up the test case
func (l *Ec2DetectorTestCase) CleanupTestCase() {}

func NewEc2DetectorTestCase(context context.T) *Ec2DetectorTestCase {
	return &Ec2DetectorTestCase{
		context: context,
	}
}
