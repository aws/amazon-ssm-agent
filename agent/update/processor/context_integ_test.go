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

// +build integration

// Package processor contains the methods for update ssm agent.
// It also provides methods for sendReply and updateInstanceInfo
package processor

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

//Valid context files
var sampleFiles = []string{
	"testdata/updatecontext.json",
}

type testCase struct {
	Input    string
	FileName string
	Output   UpdateContext
}

type FakeServiceWithFailure struct {
	Service
}

func (s *FakeServiceWithFailure) SendReply(log log.T, update *UpdateDetail) error {
	return fmt.Errorf("Failed to send reply")
}

func (s *FakeServiceWithFailure) DeleteMessage(log log.T, update *UpdateDetail) error {
	return fmt.Errorf("Failed to delete message")
}

func (s *FakeServiceWithFailure) UpdateHealthCheck(log log.T, update *UpdateDetail, errorCode string) error {
	return fmt.Errorf("Failed to update health check")
}

//Test loadContextFromFile file with valid context files
func TestParseContext(t *testing.T) {
	// generate test cases
	var testCases []testCase
	for _, contextFile := range sampleFiles {
		testCases = append(testCases, testCase{
			Input:    string(loadFile(t, contextFile)),
			Output:   loadContextFromFile(t, contextFile),
			FileName: contextFile,
		})
	}

	// run tests
	for _, tst := range testCases {
		// call method
		parsedContext, err := parseContext(log.DefaultLogger(), tst.FileName)

		// check results
		assert.Nil(t, err)
		assert.Equal(t, parsedContext.Current.SourceVersion, "")
		assert.Equal(t, len(parsedContext.Histories), 1)
		assert.Equal(t, parsedContext.Histories[0].SourceVersion, "0.0.3.0")
	}
}

var contextTests = []ContextTestCase{
	generateTestCase(),
}

func TestContext(t *testing.T) {
	// run tests
	for _, tst := range contextTests {
		// call method
		hasMessageID := tst.Context.Current.HasMessageID()
		// check results
		assert.Equal(t, hasMessageID, tst.HasMessageID)

		tst.Context.Current.AppendInfo(logger, tst.InfoMessage)
		assert.Equal(t, tst.Context.Current.StandardOut, tst.InfoMessage)

		tst.Context.Current.AppendError(logger, tst.ErrorMessage)
		assert.Equal(t, tst.Context.Current.StandardOut, fmt.Sprintf("%v\n%v", tst.InfoMessage, tst.ErrorMessage))
		assert.Equal(t, tst.Context.Current.StandardError, tst.ErrorMessage)
	}
}

func TestIsUpdateInProgress(t *testing.T) {
	context := generateTestCase().Context
	context.Current.StartDateTime = time.Now()
	context.Current.State = Staged

	result := context.IsUpdateInProgress(logger)

	assert.True(t, result)
}

func TestLoadUpdateContext(t *testing.T) {
	context, err := LoadUpdateContext(logger, sampleFiles[0])

	assert.NoError(t, err)
	assert.True(t, len(context.Histories) > 0)
}

//Load specified file from file system
func loadFile(t *testing.T, fileName string) (result []byte) {
	result, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatal(err)
	}
	return
}

//Parse manifest file
func loadContextFromFile(t *testing.T, fileName string) (context UpdateContext) {
	b := loadFile(t, fileName)
	err := json.Unmarshal(b, &context)
	if err != nil {
		t.Fatal(err)
	}
	return context
}
