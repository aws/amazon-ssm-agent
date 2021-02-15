// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
//
// +build freebsd linux netbsd openbsd darwin

// Package domainjoin implements the domain join plugin.
package domainjoin

import (
	"io"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	iohandlermocks "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler/mock"
	multiwritermock "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler/multiwriter/mock"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
	identityMocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type TestCase struct {
	Input          DomainJoinPluginInput
	Output         iohandler.DefaultIOHandler
	ExecuterErrors []error
	mark           bool
}

const (
	orchestrationDirectory = "OrchesDir"
	s3BucketName           = "bucket"
	s3KeyPrefix            = "key"
	testInstanceID         = "i-12345678"
	bucketRegionErrorMsg   = "AuthorizationHeaderMalformed: The authorization header is malformed; the region 'us-east-1' is wrong; expecting 'us-west-2' status code: 400, request id: []"
	testDirectoryName      = "corp.test.com"
	testDirectoryId        = "d-0123456789"
	testKeepHostName       = true
)

var TestCases = []TestCase{
	generateTestCaseOk(testDirectoryId, testDirectoryName, []string{"10.0.0.0", "10.0.1.0"}),
	generateTestCaseFail(testDirectoryId, testDirectoryName, []string{"10.0.0.2", "10.0.1.2"}),
}

func generateTestCaseOk(id string, name string, ipAddress []string) TestCase {

	testCase := TestCase{
		Input:  generateDomainJoinPluginInput(id, name, ipAddress),
		Output: iohandler.DefaultIOHandler{},
		mark:   true,
	}

	testCase.Output.SetStdout("")
	testCase.Output.SetStderr("")
	testCase.Output.ExitCode = 0
	testCase.Output.Status = "Success"

	return testCase
}

func generateTestCaseFail(id string, name string, ipAddress []string) TestCase {
	testCase := TestCase{
		Input:  generateDomainJoinPluginInput(id, name, ipAddress),
		Output: iohandler.DefaultIOHandler{},
		mark:   false,
	}

	testCase.Output.SetStdout("")
	testCase.Output.SetStderr("")
	testCase.Output.ExitCode = 1
	testCase.Output.Status = "Failed"

	return testCase
}

func generateDomainJoinPluginInput(id string, name string, ipAddress []string) DomainJoinPluginInput {
	return DomainJoinPluginInput{
		DirectoryId:    id,
		DirectoryName:  name,
		DnsIpAddresses: ipAddress,
	}
}

func generateDomainJoinPluginInputOptionalParamKeepHostName(id string, name string, ipAddress []string, keepHostName bool) DomainJoinPluginInput {
	return DomainJoinPluginInput{
		DirectoryId:    id,
		DirectoryName:  name,
		DnsIpAddresses: ipAddress,
		KeepHostName:   keepHostName,
	}
}

// TestRunCommands tests the runCommands and runCommandsRawInput methods, which run one set of commands.
func TestRunCommands(t *testing.T) {
	for _, testCase := range TestCases {
		testRunCommands(t, testCase, true)
		testRunCommands(t, testCase, false)
	}
}

// testRunCommands tests the runCommands or the runCommandsRawInput method for one testcase.
func testRunCommands(t *testing.T, testCase TestCase, rawInput bool) {
	var pluginString string = "domainjoin"

	if testCase.mark {
		utilExe = func(log.T, string, []string, string, string, io.Writer, io.Writer, bool) (string, error) {
			return "Success", nil
		}
	} else {
		utilExe = func(log.T, string, []string, string, string, io.Writer, io.Writer, bool) (string, error) {
			return "Success", nil
		}
	}

	makeDir = func(destinationDir string) (err error) {
		return nil
	}
	makeArgs = func(context context.T, scriptPath string, pluginInput DomainJoinPluginInput) (commandArguments string, err error) {
		return "cmd", err
	}
	createOrchesDir = func(log log.T, orchestrationDir string, pluginInput DomainJoinPluginInput) (scriptPath string, err error) {
		return "./aws_domainjoin.sh", nil
	}

	mockCancelFlag := new(task.MockCancelFlag)
	mockIOHandler := new(iohandlermocks.MockIOHandler)
	p := &Plugin{
		context: context.NewMockDefault(),
	}

	if rawInput {
		// prepare plugin input
		var rawPluginInput map[string]interface{}
		err := jsonutil.Remarshal(testCase.Input, &rawPluginInput)
		assert.Nil(t, err)

		mockIOHandler.On("SetStatus", mock.Anything).Return()
		mockIOHandler.On("GetStdoutWriter", mock.Anything).Return(new(multiwritermock.MockDocumentIOMultiWriter))
		mockIOHandler.On("GetStderrWriter", mock.Anything).Return(new(multiwritermock.MockDocumentIOMultiWriter))
		mockIOHandler.On("MarkAsSucceeded").Return()
		p.runCommandsRawInput(pluginString, rawPluginInput, orchestrationDirectory, mockCancelFlag, mockIOHandler, utilExe)
	} else {
		mockIOHandler.On("SetStatus", mock.Anything).Return()
		mockIOHandler.On("GetStdoutWriter", mock.Anything).Return(new(multiwritermock.MockDocumentIOMultiWriter))
		mockIOHandler.On("GetStderrWriter", mock.Anything).Return(new(multiwritermock.MockDocumentIOMultiWriter))
		mockIOHandler.On("MarkAsSucceeded").Return()
		p.runCommands(pluginString, testCase.Input, orchestrationDirectory, mockCancelFlag, mockIOHandler, utilExe)
	}
}

// TestMakeArguments tests the makeArguments methods, which build up the command for aws_domainjoin.sh
func TestMakeArguments(t *testing.T) {
	contextMock := &context.Mock{}
	context := context.NewMockDefault()
	domainJoinInput := generateDomainJoinPluginInput(testDirectoryId, testDirectoryName, []string{"172.31.4.141", "172.31.21.240"})
	commandRes, _ := makeArguments(context, "./aws_domainjoin.sh", domainJoinInput)
	expected := "./aws_domainjoin.sh --directory-id d-0123456789 --directory-name corp.test.com --instance-region us-east-1 --dns-addresses 172.31.4.141,172.31.21.240"
	assert.Equal(t, expected, commandRes)

	domainJoinInput = generateDomainJoinPluginInput(testDirectoryId, testDirectoryName, []string{"8.8.8.8", "8.8.8.8[[["})
	commandRes, _ = makeArguments(context, "./aws_domainjoin.sh", domainJoinInput)
	expected = ""
	assert.Equal(t, expected, commandRes)

	domainJoinInput = generateDomainJoinPluginInput(testDirectoryId, testDirectoryName, []string{"8.8.8.8[[[", "8.8.8.8"})
	commandRes, _ = makeArguments(context, "./aws_domainjoin.sh", domainJoinInput)
	expected = ""
	assert.Equal(t, expected, commandRes)

	domainJoinInput = generateDomainJoinPluginInput(testDirectoryId, testDirectoryName, []string{"Hello $(aws s3 ls)", "8.8.8.8"})
	commandRes, _ = makeArguments(context, "./aws_domainjoin.sh", domainJoinInput)
	expected = ""
	assert.Equal(t, expected, commandRes)

	domainJoinInput = generateDomainJoinPluginInput(testDirectoryId, testDirectoryName, []string{"Hello `aws s3 ls`", "8.8.8.8"})
	commandRes, _ = makeArguments(context, "./aws_domainjoin.sh", domainJoinInput)
	expected = ""
	assert.Equal(t, expected, commandRes)

	domainJoinInput = generateDomainJoinPluginInputOptionalParamKeepHostName(testDirectoryId, testDirectoryName, []string{"172.31.4.141", "172.31.21.240"}, testKeepHostName)
	commandRes, _ = makeArguments(context, "./aws_domainjoin.sh", domainJoinInput)
	expected = "./aws_domainjoin.sh --directory-id d-0123456789 --directory-name corp.test.com --instance-region us-east-1 --dns-addresses 172.31.4.141,172.31.21.240 --keep-hostname  "
	assert.Equal(t, expected, commandRes)

	// If region is not set, the domain join plugin must fail
	domainJoinInput = generateDomainJoinPluginInput(testDirectoryId, testDirectoryName, []string{"172.31.4.141", "172.31.21.240"})
	identityMock := &identityMocks.IAgentIdentity{}
	identityMock.On("Region").Return("", nil) // Remove region setting
	contextMock.On("Identity").Return(identityMock)
	contextMock.On("Log").Return(log.NewMockLog())
	commandRes, _ = makeArguments(contextMock, "./aws_domainjoin.sh", domainJoinInput)
	expected = ""
	assert.Equal(t, expected, commandRes)
}
