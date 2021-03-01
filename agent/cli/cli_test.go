// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package cli represents the entry point of the ssm agent cli.
package cli

import (
	"bytes"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/cli/clicommand"
	"github.com/aws/amazon-ssm-agent/agent/cli/cliutil"
	CliCommandMock "github.com/aws/amazon-ssm-agent/agent/cli/cliutil/mocks"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity"
	identityMocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCliUsage(t *testing.T) {
	var buffer bytes.Buffer
	args := []string{"ssm-cli"}
	exitCode := RunCommand(args, &buffer)
	assert.Contains(t, buffer.String(), "usage")
	assert.Equal(t, cliutil.CLI_PARSE_FAIL_EXITCODE, exitCode, "Command without enough arguments should return exit code 2")
}

func TestCliInvalidCommand(t *testing.T) {
	var buffer bytes.Buffer
	args := []string{"ssm-cli", "testCommand"}
	exitCode := RunCommand(args, &buffer)
	assert.Equal(t, cliutil.CLI_PARSE_FAIL_EXITCODE, exitCode, "Invalid command should return exit code 2")

}

func TestCliHelp(t *testing.T) {
	var buffer bytes.Buffer
	args := []string{"ssm-cli", "help"}
	exitCode := RunCommand(args, &buffer)
	assert.Equal(t, cliutil.CLI_SUCCESS_EXITCODE, exitCode, "help command should return exit code 0")
}

func TestCliCmdExecError(t *testing.T) {
	oldFunc := newAgentIdentity
	newAgentIdentity = func(logger.T, *appconfig.SsmagentConfig, identity.IAgentIdentitySelector) (identity.IAgentIdentity, error) {
		return identityMocks.NewDefaultMockAgentIdentity(), nil
	}
	defer func() { newAgentIdentity = oldFunc }()

	var buffer bytes.Buffer
	cliutil.Register(&clicommand.GetOfflineCommand{})
	args := []string{"ssm-cli", "get-offline-command-invocation", "--command-id", "d24d687d", "--details", "true"}
	exitCode := RunCommand(args, &buffer)
	assert.Equal(t, cliutil.CLI_COMMAND_FAIL_EXITCODE, exitCode, "command execution error return exit code 255")
}

func TestCliCmdExecSuccess(t *testing.T) {
	oldFunc := newAgentIdentity
	newAgentIdentity = func(logger.T, *appconfig.SsmagentConfig, identity.IAgentIdentitySelector) (identity.IAgentIdentity, error) {
		return identityMocks.NewDefaultMockAgentIdentity(), nil
	}
	defer func() { newAgentIdentity = oldFunc }()

	var buffer bytes.Buffer
	cliCmdMock := &CliCommandMock.CliCommand{}
	cliCmdMock.On("Name").Return("cli-command-mock").Once()
	cliCmdMock.On("Execute", mock.Anything, mock.AnythingOfType("[]string"), mock.AnythingOfType("map[string][]string")).Return(nil, "success").Once()
	cliutil.Register(cliCmdMock)

	args := []string{"ssm-cli", "cli-command-mock", "--content", "fakefile.json"}
	exitCode := RunCommand(args, &buffer)
	assert.Equal(t, cliutil.CLI_SUCCESS_EXITCODE, exitCode, "command execution success return exit code 0")
	cliCmdMock.AssertExpectations(t)
}
