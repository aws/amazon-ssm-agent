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
// Package mocks contains mocks for testStage and testCase struct
package mocks

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	testCommon "github.com/aws/amazon-ssm-agent/agent/update/tester/common"
	"github.com/stretchr/testify/mock"
)

// ITestCase is mock for struct which implements ITestStage
type ITestCase struct {
	mock.Mock
}

// CleanupTestCase is a mock function for CleanupTestCase in
// struct which implements ITestCase
func (_m *ITestCase) CleanupTestCase() {
	_m.Called()
}

// Initialize is a mock function for Initialize in
// struct which implements ITestCase
func (_m *ITestCase) Initialize(context context.T) {
	_m.Called(context)
}

// GetTestSetUpCleanupEventHandle is a mock function for GetTestSetUpCleanupEventHandle in
// struct which implements ITestCase
func (_m *ITestCase) GetTestSetUpCleanupEventHandle() func() {
	ret := _m.Called()
	var r0 func()
	if rf, ok := ret.Get(0).(func() func()); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(func())
	}
	return r0
}

// ExecuteTestCase is a mock function for ExecuteTestCase in
// struct which implements ITestCase
func (_m *ITestCase) ExecuteTestCase() testCommon.TestOutput {
	ret := _m.Called()
	var r0 testCommon.TestOutput
	if rf, ok := ret.Get(0).(func() testCommon.TestOutput); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(testCommon.TestOutput)
	}
	return r0
}

// GetTestCaseName is a mock function for GetTestCaseName in
// struct which implements ITestCase
func (_m *ITestCase) GetTestCaseName() string {
	ret := _m.Called()
	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}
	return r0
}
