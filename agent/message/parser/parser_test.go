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

// package parser contains utilities for parsing and encoding MDS/SSM messages.
package parser

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/message/contracts"
	"github.com/stretchr/testify/assert"
)

var sampleMessageFiles = []string{
	"../testdata/sampleMsg.json",
	"../testdata/sampleMsgVersion2_0.json",
}

var sampleMessageReplacedParamsFiles = []string{
	"../testdata/sampleMsgReplacedParams.json",
	"../testdata/sampleMsgReplacedParamsVersion2_0.json",
}

var logger = log.NewMockLog()

func TestParseMessageWithParams(t *testing.T) {
	type testCase struct {
		Input  string
		Output messageContracts.SendCommandPayload
	}

	// generate test cases
	var testCases []testCase
	for i, msgFileName := range sampleMessageFiles {
		msgReplacedParamsFileName := sampleMessageReplacedParamsFiles[i]
		testCases = append(testCases, testCase{
			Input:  string(loadFile(t, msgFileName)),
			Output: loadMessageFromFile(t, msgReplacedParamsFileName),
		})
	}

	// run tests
	for _, tst := range testCases {
		// call method
		parsedMsg, err := ParseMessageWithParams(logger, tst.Input)

		// check results
		assert.Nil(t, err)
		assert.Equal(t, tst.Output, parsedMsg)
	}
}

func loadFile(t *testing.T, fileName string) (result []byte) {
	result, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatal(err)
	}
	return
}

func loadMessageFromFile(t *testing.T, fileName string) (message messageContracts.SendCommandPayload) {
	b := loadFile(t, fileName)
	err := json.Unmarshal(b, &message)
	if err != nil {
		t.Fatal(err)
	}
	return message
}
