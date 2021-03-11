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

package iohandler

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	iomodulemock "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler/iomodule/mock"
	multiwritermock "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler/multiwriter/mock"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type truncateOutputTest struct {
	stdout   string
	stderr   string
	capacity int
	expected string
}

const (
	sampleSize  = 100
	longMessage = `This is a sample text. This is a sample text. This is a sample text. This is a sample text. This is a sample text.
This is a sample text. This is a sample text. This is a sample text. This is a sample text. This is a sample text.
This is a sample text. This is a sample text. This is a sample text. This is a sample text. This is a sample text.
This is a sample text. This is a sample text. This is a sample text. This is a sample text. This is a sample text.
This is a sample text. This is a sample text. This is a sample text. This is a sample text. This is a sample text.
This is a sample text. This is a sample text. This is a sample text. This is a sample text. This is a sample text.
This is a sample text. This is a sample text. This is a sample text. This is a sample text. This is a sample text.
This is a sample text. This is a sample text. This is a sample text. This is a sample text. This is a sample text.
This is a sample text. This is a sample text. This is a sample text. This is a sample text. This is a sample text.
This is a sample text. This is a sample text. This is a sample text. This is a sample text. This is a sample text.
1234567890. This is a sample text. This is a sample text`
)

var testData = []truncateOutputTest{
	//{stdout, stderr, capacity, expected}
	{"", "", sampleSize, ""},
	{"sample output", "", sampleSize, "sample output"},
	{"", "sample error", sampleSize, "\n----------ERROR-------\nsample error"},
	{"sample output", "sample error", sampleSize, "sample output\n----------ERROR-------\nsample error"},
	{longMessage, "", sampleSize, "This is a sample text. This is a sample text. This is a sample text. This is \n---Output truncated---"},
	{"", longMessage, sampleSize, "\n----------ERROR-------\nThis is a sample text. This is a sample text. This is\n---Error truncated----"},
	{longMessage, longMessage, sampleSize, "This is a sampl\n---Output truncated---\n----------ERROR-------\nThis is a sampl\n---Error truncated----"},
}

func TestTruncateOutput(t *testing.T) {
	for i, test := range testData {
		actual := TruncateOutput(test.stdout, test.stderr, test.capacity)
		assert.Equal(t, test.expected, actual, "failed test case: %v", i)
	}
}

var logger = log.NewMockLog()

func TestRegisterOutputSource(t *testing.T) {
	mockDocumentIOMultiWriter := new(multiwritermock.MockDocumentIOMultiWriter)
	mockContext := context.NewMockDefault()

	mockDocumentIOMultiWriter.On("AddWriter", mock.Anything).Times(2)
	wg := new(sync.WaitGroup)
	mockDocumentIOMultiWriter.On("GetWaitGroup").Return(wg)

	// Add 2 to WaitGroup to simulate two AddWriter calls
	wg.Add(2)

	// Create multiple test IOModules
	testModule1 := new(iomodulemock.MockIOModule)
	testModule1.On("Read", mockContext, mock.Anything, mock.AnythingOfType("int")).Return()
	testModule2 := new(iomodulemock.MockIOModule)
	testModule2.On("Read", mockContext, mock.Anything, mock.AnythingOfType("int")).Return()

	output := NewDefaultIOHandler(mockContext, contracts.IOConfiguration{})

	output.RegisterOutputSource(mockDocumentIOMultiWriter, testModule1, testModule2)

	// Sleep a bit to allow threads to finish in RegisterOutputSource to check WaitGroup
	time.Sleep(250 * time.Millisecond)
}

func TestSucceeded(t *testing.T) {
	output := DefaultIOHandler{}

	output.MarkAsSucceeded()

	assert.Equal(t, output.ExitCode, 0)
	assert.Equal(t, output.Status, contracts.ResultStatusSuccess)
	assert.True(t, output.Status.IsSuccess())
	assert.False(t, output.Status.IsReboot())
}

func TestFailed(t *testing.T) {
	output := DefaultIOHandler{}

	output.MarkAsFailed(fmt.Errorf("Error message"))

	assert.Equal(t, output.ExitCode, 1)
	assert.Equal(t, output.Status, contracts.ResultStatusFailed)
	assert.Contains(t, output.GetStderr(), "Error message")
	assert.False(t, output.Status.IsSuccess())
	assert.False(t, output.Status.IsReboot())
}

func TestMarkAsInProgress(t *testing.T) {
	output := DefaultIOHandler{}

	output.MarkAsInProgress()

	assert.Equal(t, output.ExitCode, 0)
	assert.Equal(t, output.Status, contracts.ResultStatusInProgress)
	assert.False(t, output.Status.IsSuccess())
	assert.False(t, output.Status.IsReboot())
}

func TestMarkAsSuccessWithReboot(t *testing.T) {
	output := DefaultIOHandler{}

	output.MarkAsSuccessWithReboot()

	assert.Equal(t, output.ExitCode, 0)
	assert.Equal(t, output.Status, contracts.ResultStatusSuccessAndReboot)
	assert.True(t, output.Status.IsSuccess())
	assert.True(t, output.Status.IsReboot())
}

func TestAppendInfo(t *testing.T) {
	output := DefaultIOHandler{}

	output.AppendInfo("Info message")
	output.AppendInfo("Second entry")

	assert.Contains(t, output.GetStdout(), "Info message")
	assert.Contains(t, output.GetStdout(), "Second entry")
}

func TestAppendSpecialChars(t *testing.T) {
	output := DefaultIOHandler{}

	var testString = "%v`~!@#$%^&*()-_=+[{]}|\\;:'\",<.>/?"
	output.AppendInfo(testString)
	output.AppendError(testString)

	assert.Contains(t, output.GetStdout(), testString)
	assert.Contains(t, output.GetStderr(), testString)
}

func TestAppendFormat(t *testing.T) {
	output := DefaultIOHandler{}

	var testString = "%v`~!@#$%^&*()-_=+[{]}|\\;:'\",<.>/?%%"

	// The first % is a %v - a variable to be replaced and we provided a value for it.
	// The second % isn't escaped and is treated as a fmt parameter, but no value is provided for it.
	// The double %% is an escaped single literal %.
	var testStringFormatted = "foo`~!@#$%!^(MISSING)&*()-_=+[{]}|\\;:'\",<.>/?%"
	output.AppendInfof(testString, "foo")
	output.AppendErrorf(testString, "foo")

	assert.Contains(t, output.GetStdout(), testStringFormatted)
	assert.Contains(t, output.GetStderr(), testStringFormatted)
}
