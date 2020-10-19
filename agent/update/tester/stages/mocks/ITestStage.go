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
	"github.com/stretchr/testify/mock"

	testCommon "github.com/aws/amazon-ssm-agent/agent/update/tester/common"
)

// ITestStage is mock for struct which implements ITestStage
type ITestStage struct {
	mock.Mock
}

// CleanUpTests is a mock function for CleanUpTests in
// struct which implements ITestStage
func (_m *ITestStage) CleanUpTests() {
	_m.Called()
}

// Initialize is a mock function for Initialize in
// struct which implements ITestStage
func (_m *ITestStage) Initialize() {
	_m.Called()
}

// GetCurrentRunningTest is a mock function for GetCurrentRunningTest in
// struct which implements ITestStage
func (_m *ITestStage) GetCurrentRunningTest() string {
	ret := _m.Called()
	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}
	return r0
}

// RegisterTestCase is a mock function for RegisterTestCase in
// struct which implements ITestStage
func (_m *ITestStage) RegisterTestCase(elem string, stage testCommon.ITestCase) {
	_m.Called(elem, stage)
}

// RunTests is a mock function for RunTests in preInstallTester
func (_m *ITestStage) RunTests() bool {
	ret := _m.Called()
	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}
	return r0
}
