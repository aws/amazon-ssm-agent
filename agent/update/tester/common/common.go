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
// Package common contains common methods, interfaces and variables used across the tester packages
package common

import (
	"os"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/common/message"
)

// ITestStage interface should be implemented by
// various testing stages
type ITestStage interface {
	Initialize()
	RunTests() bool
	RegisterTestCase(string, ITestCase)
	CleanUpTests()
	GetCurrentRunningTest() string
}

// ITestCase interface should be implemented by test cases
// to be picked up by test setup
type ITestCase interface {
	Initialize(context.T)
	ExecuteTestCase() TestOutput
	CleanupTestCase()
	GetTestCaseName() string
	GetTestSetUpCleanupEventHandle() func()
}

// TestOutput is structure of test case execution result
type TestOutput struct {
	Result TestResult
	Err    error
}

const (
	// NamedPipeTestCaseName is named pipe test case name
	NamedPipeTestCaseName string = "NamedPipe"

	defaultFileCreateMode = 0750
)

var (
	// TestIPCChannel is test ipc channel used in this tester package
	TestIPCChannel = message.DefaultIPCPrefix + message.DefaultCoreAgentChannel + "test"

	// DefaultTestTimeOutSeconds is default timeout of test stages
	DefaultTestTimeOutSeconds = 5 //seconds
)

type TestStage string

const (
	// PreInstallTest denotes the pre-install test stage
	PreInstallTest TestStage = "PreInstall"

	// PreInstallTest denotes the post-install test stage
	PostInstallTest TestStage = "PostInstall"
)

type TestResult string

const (
	// TestCasePass denotes test case pass
	TestCasePass TestResult = "Pass"

	// TestCaseFail denotes test case fail
	TestCaseFail TestResult = "Fail"
)

func createIfNotExist(dir string) (err error) {
	if _, err = os.Stat(dir); os.IsNotExist(err) {
		//configure it to be not accessible by others
		err = os.MkdirAll(dir, defaultFileCreateMode)
	}
	return
}
