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
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	// Environment variables set for processes executed by ssm agent should have names that start with AWS_SSM_
	testInstanceID = "i-f00f00f00f00f00ba"
	testRegionName = "foo-bar-3"
	testError      = "FooBar"
)

type instanceInfoStub struct {
	instanceID      string
	instanceIDError error
	regionName      string
	regionNameError error
}

func (m *instanceInfoStub) InstanceID() (string, error) {
	return m.instanceID, m.instanceIDError
}

func (m *instanceInfoStub) Region() (string, error) {
	return m.regionName, m.regionNameError
}

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

func getTestCommand(t *testing.T) *exec.Cmd {
	command := exec.Command("test")
	assert.Empty(t, getEnvVariableValue(command.Env, envVarInstanceID), fmt.Sprintf("%s is already defined", envVarInstanceID))
	assert.Empty(t, getEnvVariableValue(command.Env, envVarRegionName), fmt.Sprintf("%s is already defined", envVarRegionName))

	return command
}

func TestEnvironmentVariables_All(t *testing.T) {
	instanceTemp := instance
	instance = &instanceInfoStub{instanceID: testInstanceID, regionName: testRegionName}
	defer func() { instance = instanceTemp }()

	command := getTestCommand(t)
	prepareEnvironment(command)

	assert.Equal(t, getEnvVariableValue(command.Env, envVarInstanceID), testInstanceID)
	assert.Equal(t, getEnvVariableValue(command.Env, envVarRegionName), testRegionName)
}

func TestEnvironmentVariables_None(t *testing.T) {
	instanceTemp := instance
	instance = &instanceInfoStub{"", errors.New(testError), "", errors.New(testError)}
	defer func() { instance = instanceTemp }()

	command := getTestCommand(t)
	prepareEnvironment(command)

	assert.Empty(t, getEnvVariableValue(command.Env, envVarInstanceID))
	assert.Empty(t, getEnvVariableValue(command.Env, envVarRegionName))
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
