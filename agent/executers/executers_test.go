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

// Package executers contains general purpose (shell) command executing objects.
package executers

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/mocks/context"
	mockIdentity "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/stretchr/testify/assert"
)

const (
	// Environment variables set for processes executed by ssm agent should have names that start with AWS_SSM_
	testInstanceID = "i-f00f00f00f00f00ba"
	testRegionName = "foo-bar-3"
	testError      = "FooBar"
)

// Return the value of a named environment variable from a list of environment variable
// where the format of each entry is name=value
// Return nil if no variable with the given envVarName is found in the collection env
func getEnvVariableValue(env []string, envVarName string) string {
	for _, envVariable := range env {
		if strings.HasPrefix(envVariable, envVarName+"=") {
			return strings.TrimPrefix(envVariable, envVarName+"=")
		}
	}
	return ""
}

func getTestCommand() *exec.Cmd {
	command := exec.Command("test")
	command.Env = []string{}

	return command
}

func TestEnvironmentVariables_All(t *testing.T) {
	os.Clearenv()
	context := context.NewMockDefault()
	command := getTestCommand()
	env := make(map[string]string)
	env["envKey"] = "envVal"
	prepareEnvironment(context, command, env)

	assert.Equal(t, mockIdentity.MockInstanceID, getEnvVariableValue(command.Env, envVarInstanceID))
	assert.Equal(t, mockIdentity.MockRegion, getEnvVariableValue(command.Env, envVarRegionName))
	assert.Equal(t, "envVal", getEnvVariableValue(command.Env, "envKey"))
}

func TestEnvironmentVariables_None(t *testing.T) {
	os.Clearenv()
	context := context.NewMockDefault()

	command := getTestCommand()
	prepareEnvironment(context, command, make(map[string]string))

	assert.Empty(t, getEnvVariableValue(command.Env, mockIdentity.MockInstanceID))
	assert.Empty(t, getEnvVariableValue(command.Env, mockIdentity.MockRegion))
}

func TestQuoteShString(t *testing.T) {
	var result string

	result = QuoteShString("")
	assert.Equal(t, "''", result)

	result = QuoteShString("abc")
	assert.Equal(t, "'abc'", result)
}

func TestQuoteShStringWithQuotes(t *testing.T) {
	var result string

	result = QuoteShString("\"abc\"")
	assert.Equal(t, "'\"abc\"'", result)

	result = QuoteShString("'abc'")
	assert.Equal(t, "''\\''abc'\\'''", result)
}

func TestQuotePsString(t *testing.T) {
	var result string

	result = QuotePsString("")
	assert.Equal(t, "\"\"", result)

	result = QuotePsString("abc")
	assert.Equal(t, "\"abc\"", result)
}

func TestQuotePsStringWithQuotes(t *testing.T) {
	var result string

	result = QuotePsString("\"abc\"")
	assert.Equal(t, "\"`\"abc`\"\"", result)

	result = QuotePsString("'abc'")
	assert.Equal(t, "\"'abc'\"", result)

	result = QuotePsString("`abc`")
	assert.Equal(t, "\"``abc``\"", result)
}
