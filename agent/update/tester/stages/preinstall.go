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
// permissions and limitations under the License
//
// package stages holds testStages and testStages are responsible for executing test cases
package stages

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
	testCommon "github.com/aws/amazon-ssm-agent/agent/update/tester/common"

	"github.com/aws/amazon-ssm-agent/agent/update/tester/stages/testcases"
)

// GetPreInstallTestObj returns preInstallTester instance with initial values
func GetPreInstallTestObj(logger log.T) testCommon.ITestStage {
	return &preInstallTester{
		logger: logger,
	}
}

// preInstallTester is responsible for running test cases needed to be run on customer's machine before install
type preInstallTester struct {
	logger              log.T
	passedTests         []string
	currentTest         string
	registeredTestCases map[string]testCommon.ITestCase
}

// Initialize initializes values needed for this test stage
func (t *preInstallTester) Initialize() {
	t.passedTests = make([]string, 0)
	namedPipeTestCaseObj := &testcases.NamedPipeTestCase{}
	t.RegisterTestCase(namedPipeTestCaseObj.GetTestCaseName(), namedPipeTestCaseObj)
}

// RegisterTestCase registers test case for this stage
func (t *preInstallTester) RegisterTestCase(metricName string, testCase testCommon.ITestCase) {
	if t.registeredTestCases == nil {
		t.registeredTestCases = make(map[string]testCommon.ITestCase)
	}
	t.registeredTestCases[metricName] = testCase
}

// RunTests runs the test case based on registeredTestCases map.
func (t *preInstallTester) RunTests() bool {
	for testCaseName, testCaseObj := range t.registeredTestCases {
		t.currentTest = testCaseName
		testCaseObj.Initialize(t.logger)
		testCaseOutput := testCaseObj.ExecuteTestCase()
		testCaseObj.CleanupTestCase()
		if testCaseOutput.Result == testCommon.TestCaseFail {
			return false
		} else {
			t.passedTests = append(t.passedTests, testCaseName)
		}
	}
	t.currentTest = "" // reset when all the tests are done
	return true
}

// CleanUpTests invokes the clean up handle from test cases
func (t *preInstallTester) CleanUpTests() {
	metrics := t.passedTests
	if t.currentTest != "" {
		metrics = append(metrics, t.currentTest)
	}

	for _, val := range metrics {
		obj := t.registeredTestCases[val]
		cleanSetup := obj.GetTestSetUpCleanupEventHandle()
		if cleanSetup != nil {
			cleanSetup()
		}
	}
}

// GetCurrentRunningTest gets the current running test case
func (t *preInstallTester) GetCurrentRunningTest() string {
	return t.currentTest
}
