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

package ssmlog

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
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

func TestReplaceLogger(t *testing.T) {
	var out bytes.Buffer
	msg := "Some Message"

	context := "<context>"
	callingFunctionName := "TestReplaceLogger"
	oldLevelStr := "Debug"
	newLevelStr := "Info"
	oldFormat := "%FuncShort [%Level] %Msg%n"
	newFormat := "%FuncShort %Level %Msg%n"
	oldOutput := fmt.Sprintf("%s [%v] %v %v\n", callingFunctionName, oldLevelStr, context, msg)
	newOutput := fmt.Sprintf("%s %v %v %v\n", callingFunctionName, newLevelStr, context, msg)

	// create old (to be replaced) seelog logger that outputs to buffer
	seelogger, err := seelog.LoggerFromWriterWithMinLevelAndFormat(&out, seelog.DebugLvl, oldFormat)
	assert.Nil(t, err)

	// create logger with context
	logger := withContext(seelogger, context)

	// test the logger
	logger.Debug(msg)
	logger.Flush()
	assert.Equal(t, oldOutput, out.String())

	// Check for correct type of logger
	wrapper, ok := logger.(*log.Wrapper)
	assert.True(t, ok, "withContext did not create a logger of type *Wrapper. Conversion not ok")

	// create new (to be replaced with) seelog logger that outputs to buffer
	newSeelogger, err := seelog.LoggerFromWriterWithMinLevelAndFormat(&out, seelog.InfoLvl, newFormat)
	assert.Nil(t, err)
	setStackDepth(newSeelogger)

	// Replace the underlying base logger in wrapper
	wrapper.ReplaceDelegate(newSeelogger)

	// Use the same original context logger and check difference in logging
	// Reset test buffer

	out.Reset()

	// test the logger
	logger.Info(msg)
	logger.Flush()
	assert.Equal(t, newOutput, out.String())

}
