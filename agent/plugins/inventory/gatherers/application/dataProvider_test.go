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

// Package application contains a application gatherer.
package application

import (
	"strings"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/localpackages"
	repomock "github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/localpackages/mock"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestComponentType(t *testing.T) {
	awsComponents := []string{"amazon-ssm-agent", "aws-apitools-mon", "aws-amitools-ec2", "AWS Tools for Windows", "AWS PV Drivers"}
	nonawsComponents := []string{"Notepad++", "Google Update Helper", "accountsservice", "pcre", "kbd-misc"}

	for _, name := range awsComponents {
		assert.Equal(t, model.AWSComponent, componentType(name))
	}

	for _, name := range nonawsComponents {
		assert.Equal(t, model.ComponentType(0), componentType(name))
	}
}

func MockPackageRepositoryEmpty() localpackages.Repository {
	mockRepo := repomock.MockedRepository{}
	mockRepo.On("GetInventoryData", mock.Anything).Return([]model.ApplicationData{})
	return &mockRepo
}

func MockPackageRepository(result []model.ApplicationData) localpackages.Repository {
	mockRepo := repomock.MockedRepository{}
	mockRepo.On("GetInventoryData", mock.Anything).Return(result)
	return &mockRepo
}

func TestCleanupJsonField(t *testing.T) {
	inOut := [][]string{
		{"a\nb", `a`},
		{"a\tb\nc", `a\tb`},
		{`a\b`, `a\\b`},
		{`a"b`, `a\"b`},
		{`\"b` + "\n", `\\\"b`},
		{"description\non\nmulti\nline", `description`},
		{"a simple text", `a simple text`},
	}
	for _, test := range inOut {
		input, output := test[0], test[1]
		result := cleanupJsonField(input)
		assert.Equal(t, output, result)
	}
}

func TestReplaceMarkedFields(t *testing.T) {
	identity := func(a string) string { return a }
	replaceWithDummy := func(a string) string { return "dummy" }
	type testCase struct {
		input       string
		startMarker string
		endMarker   string
		replacer    func(string) string
		output      string
	}
	inOut := []testCase{
		{"a<-tom->s", "<-", "->", identity, "atoms"},
		{"a<-tom->s", "<-", "->", replaceWithDummy, "adummys"},
		{"a<>t</>s", "<>", "</>", strings.ToUpper, "aTs"},
		{`a<tom>abc<de>`, "<", ">", strings.ToUpper, `aTOMabcDE`},
		{`|tom|abc|de|`, "|", "|", strings.ToUpper, `TOMabcDE`},
		{"atoms", "[missingMarker]", "[/missingMarker]", strings.ToUpper, "atoms"},
		{"at<start>oms", "<start>", "</missingEnd>", strings.ToUpper, ""}, // error case
	}
	for _, tst := range inOut {
		result, err := replaceMarkedFields(tst.input, tst.startMarker, tst.endMarker, tst.replacer)
		if tst.output != "" {
			assert.Equal(t, tst.output, result)
		} else {
			assert.NotNil(t, err)
		}
	}
}
