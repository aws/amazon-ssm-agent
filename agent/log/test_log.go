// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package log

import (
	"errors"
	"fmt"

	"github.com/stretchr/testify/mock"
)

// Mock stands for a mocked log.
type Mock struct {
	mock.Mock
	context string
	silent  bool
}

// NewMockLogger returns an instance of Mock with default expectations set.
func NewMockLog() *Mock {
	log := new(Mock)
	log.On("Close").Return()
	log.On("Flush").Return()
	log.On("Debug", mock.Anything).Return()
	log.On("Error", mock.Anything).Return(mock.AnythingOfType("error"))
	log.On("Warn", mock.Anything).Return(mock.AnythingOfType("error"))
	log.On("Trace", mock.Anything).Return()
	log.On("Info", mock.Anything).Return()
	log.On("WriteEvent", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return()
	log.On("Debugf", mock.Anything, mock.Anything).Return()
	log.On("Errorf", mock.AnythingOfType("string"), mock.Anything).Return(mock.AnythingOfType("error"))
	log.On("Warnf", mock.AnythingOfType("string"), mock.Anything).Return(mock.AnythingOfType("error"))
	log.On("Tracef", mock.Anything, mock.Anything).Return()
	log.On("Infof", mock.Anything, mock.Anything).Return()
	log.On("Closed").Return(false)
	log.On("WithContext", mock.Anything).Return(log)
	return log
}

// NewSilentMockLogger returns an instance of Mock with default expectations set.
func NewSilentMockLog() *Mock {
	log := NewMockLog()
	log.silent = true
	return log
}

func NewMockLogWithContext(ctx string) *Mock {
	log := new(Mock)
	log.context = "[" + ctx + "]"
	log.On("Close").Return()
	log.On("Flush").Return()
	log.On("Debug", mock.Anything).Return()
	log.On("Error", mock.Anything).Return(mock.AnythingOfType("error"))
	log.On("Warn", mock.Anything).Return(mock.AnythingOfType("error"))
	log.On("Trace", mock.Anything).Return()
	log.On("Info", mock.Anything).Return()
	log.On("WriteEvent", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return()
	log.On("Debugf", mock.Anything, mock.Anything).Return()
	log.On("Errorf", mock.AnythingOfType("string"), mock.Anything).Return(mock.AnythingOfType("error"))
	log.On("Warnf", mock.AnythingOfType("string"), mock.Anything).Return(mock.AnythingOfType("error"))
	log.On("Tracef", mock.Anything, mock.Anything).Return()
	log.On("Infof", mock.Anything, mock.Anything).Return()
	log.On("Closed").Return(false)
	return log
}

func (_m *Mock) WithContext(context ...string) (contextLogger T) {
	if !_m.silent {
		fmt.Print(_m.context)
		fmt.Printf("WithContext: %v", context)
	}
	ret := _m.Called(context)
	return ret.Get(0).(T)
}

func (_m *Mock) WriteEvent(eventType string, agentVersion string, content string) {
	if !_m.silent {
		fmt.Print(_m.context)
		fmt.Print("Write Event: ")
		fmt.Println(content)
	}
	_m.Called(eventType, agentVersion, content)
}

// Tracef mocks the Tracef function.
func (_m *Mock) Tracef(format string, params ...interface{}) {
	if !_m.silent {
		fmt.Print(_m.context)
		fmt.Printf("Tracef: "+format+"\n", params...)
	}
	_m.Called(format, params)
}

// Debugf mocks the Debugf function.
func (_m *Mock) Debugf(format string, params ...interface{}) {
	if !_m.silent {
		fmt.Print(_m.context)
		fmt.Printf("Debugf: "+format, params...)
		fmt.Println()
	}
	_m.Called(format, params)
}

// Infof mocks the Infof function.
func (_m *Mock) Infof(format string, params ...interface{}) {
	if !_m.silent {
		fmt.Print(_m.context)
		fmt.Printf("Infof: "+format, params...)
		fmt.Println()
	}
	_m.Called(format, params)
}

// Warnf mocks the Warnf function.
func (_m *Mock) Warnf(format string, params ...interface{}) error {
	msg := fmt.Sprintf("Warnf: "+format, params...)
	if !_m.silent {
		fmt.Print(_m.context)
		fmt.Printf(msg)
		fmt.Println()
	}
	_m.Called(format, params)
	return errors.New(msg)
}

// Errorf mocks the Errorf function.
func (_m *Mock) Errorf(format string, params ...interface{}) error {
	msg := fmt.Sprintf("Errorf: "+format, params...)
	if !_m.silent {
		fmt.Print(_m.context)
		fmt.Printf(msg)
		fmt.Println()
	}
	_m.Called(format, params)
	return errors.New(msg)
}

// Criticalf mocks the Criticalf function.
func (_m *Mock) Criticalf(format string, params ...interface{}) error {
	msg := fmt.Sprintf("Criticalf: "+format, params...)
	if !_m.silent {
		fmt.Print(_m.context)
		fmt.Printf(msg)
		fmt.Println()
	}
	_m.Called(format, params)
	return errors.New(msg)
}

// Trace mocks the Trace function.
func (_m *Mock) Trace(v ...interface{}) {
	if !_m.silent {
		fmt.Print(_m.context)
		fmt.Print("Trace: ")
		fmt.Println(v...)
	}
	_m.Called(v)
}

// Debug mocks the Debug function.
func (_m *Mock) Debug(v ...interface{}) {
	if !_m.silent {
		fmt.Print(_m.context)
		fmt.Print("Debug: ")
		fmt.Println(v...)
	}
	_m.Called(v)
}

// Info mocks the Info function.
func (_m *Mock) Info(v ...interface{}) {
	if !_m.silent {
		fmt.Print(_m.context)
		fmt.Print("Info: ")
		fmt.Println(v...)
	}
	_m.Called(v)
}

// Warn mocks the Warn function.
func (_m *Mock) Warn(v ...interface{}) error {
	msg := "Warn: " + fmt.Sprint(v...)

	if !_m.silent {
		fmt.Print(_m.context)
		fmt.Print(msg)
		fmt.Println()
	}
	_m.Called(v)
	return errors.New(msg)
}

// Error mocks the Error function.
func (_m *Mock) Error(v ...interface{}) error {
	msg := fmt.Sprint("Error: ") + fmt.Sprint(v...)
	if !_m.silent {
		fmt.Print(_m.context)
		fmt.Printf(msg)
		fmt.Println()
	}
	_m.Called(v)
	return errors.New(msg)
}

// Critical mocks the Critical function.
func (_m *Mock) Critical(v ...interface{}) error {
	if !_m.silent {
		fmt.Print(_m.context)
		fmt.Print("Critical: ")
		fmt.Println(v...)
	}
	ret := _m.Called(v)
	return ret.Error(0)
}

// Flush mocks the Flush function.
func (_m *Mock) Flush() {
	_m.Called()
}

// Close mocks the Close function.
func (_m *Mock) Close() {
	_m.Called()
}

func (_m *Mock) Closed() bool {
	_m.Called()
	return false
}
