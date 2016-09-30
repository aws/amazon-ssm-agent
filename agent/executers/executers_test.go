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

package executers

import (
	"os/exec"
	"testing"

	"fmt"
	"strings"

	"Amazon-ssm-agent/vendor/src/github.com/stretchr/testify/assert"
)

const (
	testValInstanceId = "Foo"
	testValRegionName = "Bar"
)

// Return the value of a named environment variable from a list of environment variable
// where the format of each entry is name=value
// Return nil if no variable with the given envVarName is found in the collection env
func getEnvVariableValue(env []string, envVarName string) string {
	for envVariable := range env {
		if strings.HasPrefix(envVariable, envVarName+"=") {
			return strings.TrimPrefix(envVariable, envVarName+"=")
		}
	}
	return nil
}

func getTestCommand() exec.Cmd {
	command := exec.Cmd("test")
	assert.Nil(getEnvVariableValue(command.Env, envVarInstanceId), fmt.Sprintf("%s is already defined", envVarInstanceId))
	assert.Nil(getEnvVariableValue(command.Env, envVarRegionName), fmt.Sprintf("%s is already defined", envVarRegionName))

	return command
}

func TestEnvironmentVariables_All(t *testing.T) {
	// TODO:MF: Set mock values for instanceId and region

	command := getTestCommand()
	prepareEnvVariables(command)

	actualValInstanceId := getEnvVariableValue(command.Env, envVarInstanceId)
	assert.Equal(actualValInstanceId, testValInstanceId, fmt.Sprintf("expected %s but actually %s", testValInstanceId, actualValInstanceId))

	actualValRegionName := getEnvVariableValue(command.Env, envVarRegionName)
	assert.Equal(actualValRegionName, testValRegionName, fmt.Sprintf("expected %s but actually %s", testValRegionName, actualValRegionName))
}

func TestEnvironmentVariables_None(t *testing.T) {
	// TODO:MF: Set mock values for instanceId and region (such that they are empty strings that will return errors from platform.InstanceId and platform.Region)

	command := getTestCommand()
	prepareEnvVariables(command)

	actualValInstanceId := getEnvVariableValue(command.Env, envVarInstanceId)
	assert.Nil(actualValInstanceId, fmt.Sprintf("%s should be nil", envVarInstanceId))

	actualValRegionName := getEnvVariableValue(command.Env, envVarRegionName)
	assert.Nil(actualValRegionName, fmt.Sprintf("%s should be nil", envVarRegionName))
}
