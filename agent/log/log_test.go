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
	"bytes"
	"fmt"
	"testing"

	seelog "github.com/cihub/seelog"
	"github.com/stretchr/testify/assert"
)

type TestCase struct {
	Context   string
	LogFormat string
	Level     seelog.LogLevel
	Message   string
	Params    []interface{}
	Output    string
}

func generateTestCase(t *testing.T, level seelog.LogLevel, callingFunctionName string, message string, params ...interface{}) TestCase {
	testCase := TestCase{
		Context:   "<some context>",
		LogFormat: "%FuncShort [%Level] %Msg%n",
		Level:     level,
		Message:   message,
		Params:    params,
	}
	var levelStr string
	switch level {
	case seelog.ErrorLvl:
		levelStr = "Error"

	case seelog.InfoLvl:
		levelStr = "Info"

	case seelog.DebugLvl:
		levelStr = "Debug"

	default:
		assert.Fail(t, "Unexpected log level", level)
	}

	msg := fmt.Sprintf(testCase.Message, testCase.Params...)
	testCase.Output = fmt.Sprintf("%s [%v] %v %v\n", callingFunctionName, levelStr, testCase.Context, msg)
	return testCase
}

func TestLoggerWithContext(t *testing.T) {
	var testCases []TestCase

	callingFunctionName := "testLoggerWithContext"
	for _, logLevel := range []seelog.LogLevel{seelog.DebugLvl, seelog.InfoLvl, seelog.ErrorLvl} {
		testCases = append(testCases, generateTestCase(t, logLevel, callingFunctionName, "(some message without parameters)"))
		testCases = append(testCases, generateTestCase(t, logLevel, callingFunctionName, "(some message with %v as param)", []interface{}{"|a param|"}))
	}

	for _, testCase := range testCases {
		testLoggerWithContext(t, testCase)
	}
}

func testLoggerWithContext(t *testing.T, testCase TestCase) {
	// create seelog logger that outputs to buffer
	var out bytes.Buffer
	seelogger, err := seelog.LoggerFromWriterWithMinLevelAndFormat(&out, seelog.TraceLvl, testCase.LogFormat)
	assert.Nil(t, err)

	// call method under test
	logger := withContext(seelogger, testCase.Context)

	// exercise logger
	switch testCase.Level {
	case seelog.ErrorLvl:
		if len(testCase.Params) > 0 {
			logger.Errorf(testCase.Message, testCase.Params...)
		} else {
			logger.Error(testCase.Message)
		}

	case seelog.InfoLvl:
		if len(testCase.Params) > 0 {
			logger.Infof(testCase.Message, testCase.Params...)
		} else {
			logger.Info(testCase.Message)
		}

	case seelog.DebugLvl:
		if len(testCase.Params) > 0 {
			logger.Debugf(testCase.Message, testCase.Params...)
		} else {
			logger.Debug(testCase.Message)
		}

	default:
		assert.Fail(t, "Unexpected log level", testCase.Level)
	}
	logger.Flush()

	// check result
	assert.Equal(t, testCase.Output, out.String())
}
