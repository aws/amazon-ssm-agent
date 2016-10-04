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
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	// Environment variables set for processes executed by ssm agent should have names that start with AWS_SSM_
	testInstanceId = "i-f00f00f00f00f00ba"
	testRegionName = "foo-bar-3"
	testError      = "FooBar"
)

type instanceInfoStub struct {
	instanceID      string
	instanceIDError string
	regionName      string
	regionNameError string
}

func (m *instanceInfoStub) InstanceID() (string, error) {
	return instanceInfoStub.instanceID, errors.New(instanceInfoStub.instanceIDError)
}

func (m *instanceInfoStub) Region() (string, error) {
	return instanceInfoStub.regionName, errors.New(instanceInfoStub.regionNameError)
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
	assert.Empty(t, getEnvVariableValue(command.Env, envVarInstanceId), fmt.Sprintf("%s is already defined", envVarInstanceId))
	assert.Empty(t, getEnvVariableValue(command.Env, envVarRegionName), fmt.Sprintf("%s is already defined", envVarRegionName))

	return command
}

func TestEnvironmentVariables_All(t *testing.T) {
	instance = &instanceInfoStub{instanceID: testInstanceId, regionName: testRegionName}

	command := getTestCommand(t)
	prepareEnvVariables(command)

	assert.Equal(t, getEnvVariableValue(command.Env, envVarInstanceId), testInstanceId)
	assert.Equal(t, getEnvVariableValue(command.Env, envVarRegionName), testRegionName)
}

func TestEnvironmentVariables_None(t *testing.T) {
	instance = &instanceInfoStub{"", testError, "", testError}

	command := getTestCommand(t)
	prepareEnvVariables(command)

	assert.Empty(t, getEnvVariableValue(command.Env, envVarInstanceId))
	assert.Empty(t, getEnvVariableValue(command.Env, envVarRegionName))
}
