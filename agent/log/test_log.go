// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package log

import (
	"fmt"

	"github.com/stretchr/testify/mock"
)

// Mock stands for a mocked log.
type Mock struct {
	mock.Mock
}

// NewMockLogger returns an instance of Mock with default expectations set.
func NewMockLog() *Mock {
	log := new(Mock)
	log.On("Close").Return()
	log.On("Flush").Return()
	log.On("Debug", mock.Anything).Return()
	log.On("Error", mock.Anything).Return(nil)
	log.On("Trace", mock.Anything).Return()
	log.On("Info", mock.Anything).Return()
	log.On("Debugf", mock.Anything, mock.Anything).Return()
	log.On("Errorf", mock.Anything, mock.Anything).Return(nil)
	log.On("Tracef", mock.Anything, mock.Anything).Return()
	log.On("Infof", mock.Anything, mock.Anything).Return()
	return log
}

// Tracef mocks the Tracef function.
func (_m *Mock) Tracef(format string, params ...interface{}) {
	//fmt.Printf("Tracef: "+format, params)
	_m.Called(format, params)
}

// Debugf mocks the Debugf function.
func (_m *Mock) Debugf(format string, params ...interface{}) {
	//fmt.Printf("Debugf: "+format, params)
	_m.Called(format, params)
}

// Infof mocks the Infof function.
func (_m *Mock) Infof(format string, params ...interface{}) {
	//fmt.Printf("Infof: "+format, params)
	_m.Called(format, params)
}

// Warnf mocks the Warnf function.
func (_m *Mock) Warnf(format string, params ...interface{}) error {
	//fmt.Printf("Warnf: "+format, params)
	ret := _m.Called(format, params)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, ...interface{}) error); ok {
		r0 = rf(format, params...)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Errorf mocks the Errorf function.
func (_m *Mock) Errorf(format string, params ...interface{}) error {
	//fmt.Printf("Errorf: "+format, params)
	ret := _m.Called(format, params)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, ...interface{}) error); ok {
		r0 = rf(format, params...)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Criticalf mocks the Criticalf function.
func (_m *Mock) Criticalf(format string, params ...interface{}) error {
	fmt.Printf("Criticalf: "+format, params)
	ret := _m.Called(format, params)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, ...interface{}) error); ok {
		r0 = rf(format, params...)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Trace mocks the Trace function.
func (_m *Mock) Trace(v ...interface{}) {
	//fmt.Printf("Trace: %v", v)
	_m.Called(v)
}

// Debug mocks the Debug function.
func (_m *Mock) Debug(v ...interface{}) {
	//fmt.Printf("Debug: %v", v)
	_m.Called(v)
}

// Info mocks the Info function.
func (_m *Mock) Info(v ...interface{}) {
	//fmt.Printf("Info %v", v)
	_m.Called(v)
}

// Warn mocks the Warn function.
func (_m *Mock) Warn(v ...interface{}) error {
	//fmt.Printf("Warn: %v", v)
	ret := _m.Called(v)

	var r0 error
	if rf, ok := ret.Get(0).(func(...interface{}) error); ok {
		r0 = rf(v...)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Error mocks the Error function.
func (_m *Mock) Error(v ...interface{}) error {
	//fmt.Printf("Error: %v", v)
	ret := _m.Called(v)

	var r0 error
	if rf, ok := ret.Get(0).(func(...interface{}) error); ok {
		r0 = rf(v...)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Critical mocks the Critical function.
func (_m *Mock) Critical(v ...interface{}) error {
	fmt.Printf("Critical: %v", v)
	ret := _m.Called(v)

	var r0 error
	if rf, ok := ret.Get(0).(func(...interface{}) error); ok {
		r0 = rf(v...)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Flush mocks the Flush function.
func (_m *Mock) Flush() {
	_m.Called()
}

// Close mocks the Close function.
func (_m *Mock) Close() {
	_m.Called()
}
