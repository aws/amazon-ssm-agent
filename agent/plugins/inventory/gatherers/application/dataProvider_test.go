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

func TestCleanupJSONField(t *testing.T) {
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
		result := cleanupJSONField(input)
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

func TestConvertEntriesToJsonArray(t *testing.T) {
	inOut := [][]string{
		{
			`{"k1":"v1","k2":"v2"},{"s1":"t1"},`,
			`[{"k1":"v1","k2":"v2"},{"s1":"t1"}]`,
		},
		{
			`{"Name":"nss-softokn"},{"Name":"basesystem"},{"Name":"pcre"},`,
			`[{"Name":"nss-softokn"},{"Name":"basesystem"},{"Name":"pcre"}]`,
		},
		{`{"k1":"v1"},`, `[{"k1":"v1"}]`},
		{`{"k1":"v1"}`, `[{"k1":"v1"}]`},
		{`,`, `[]`},
		{``, `[]`},
	}
	for _, test := range inOut {
		input, output := test[0], test[1]
		result := convertEntriesToJsonArray(input)
		assert.Equal(t, output, result)
	}
}

func TestCleanupNewLines(t *testing.T) {
	inOut := [][]string{
		{"ab\nc", "abc"},
		{"\nab\n\rc\n\r", "abc"},
		{"abc\r", "abc"},
		{"a", "a"},
		{"", ""},
	}
	for _, test := range inOut {
		input, output := test[0], test[1]
		result := cleanupNewLines(input)
		assert.Equal(t, output, result)
	}
}

func TestStripCtlFromUTF8(t *testing.T) {
	input := []byte{65, 108, 116, 101, 114, 121, 120, 50, 48, 49, 0, 56, 46, 49, 12, 120, 54, 52, 83, 101, 114, 118, 101, 114}
	assert.Equal(t, stripCtlFromUTF8(string(input)), "Alteryx2018.1x64Server")
}

func assertEqual(t *testing.T, expected []model.ApplicationData, found []model.ApplicationData) {
	assert.Equal(t, len(expected), len(found))
	for i, expectedApp := range expected {
		foundApp := found[i]
		assertEqualApps(t, expectedApp, foundApp)
	}
}

func assertEqualApps(t *testing.T, a model.ApplicationData, b model.ApplicationData) {
	assert.Equal(t, a.Name, b.Name)
	assert.Equal(t, a.Publisher, b.Publisher)
	assert.Equal(t, a.Version, b.Version)
	assert.Equal(t, a.Release, b.Release)
	assert.Equal(t, a.Epoch, b.Epoch)
	assert.Equal(t, a.InstalledTime, b.InstalledTime)
	assert.Equal(t, a.ApplicationType, b.ApplicationType)
	assert.Equal(t, a.Architecture, b.Architecture)
	assert.Equal(t, a.URL, b.URL)
	assert.Equal(t, a.Summary, b.Summary)
	assert.Equal(t, a.PackageId, b.PackageId)
}

// createMockExecutor creates an executor that returns the given stdout values on subsequent invocations.
// If the number of invocations exceeds the number of outputs provided, the executor will return the last output.
// For example createMockExecutor("a", "b", "c") will return an executor that returns the following values:
// on first call -> "a"
// on second call -> "b"
// on third call -> "c"
// on every call after that -> "c"
func createMockExecutor(stdout ...string) func(string, ...string) ([]byte, error) {
	var index = 0
	return func(string, ...string) ([]byte, error) {
		if index < len(stdout) {
			index += 1
		}
		return []byte(stdout[index-1]), nil
	}
}
