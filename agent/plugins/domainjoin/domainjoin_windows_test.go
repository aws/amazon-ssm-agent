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
//
//go:build windows
// +build windows

// Package domainjoin implements the domain join plugin.
package domainjoin

import (
	"errors"
	"io"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	contextmocks "github.com/aws/amazon-ssm-agent/agent/mocks/context"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/agent/mocks/task"
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
	testSetHostName        = "my_hostname"
)

var TestCases = []TestCase{
	generateTestCaseOk(testDirectoryId, testDirectoryName, []string{"10.0.0.0", "10.0.1.0"}),
	generateTestCaseFail(testDirectoryId, testDirectoryName, []string{"10.0.0.2", "10.0.1.2"}),
}

var logger = logmocks.NewMockLog()

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

func generateDomainJoinPluginInputOptionalParamSetHostName(id string, name string, ipAddress []string, setHostName string) DomainJoinPluginInput {
	return DomainJoinPluginInput{
		DirectoryId:    id,
		DirectoryName:  name,
		DnsIpAddresses: ipAddress,
		HostName:       setHostName,
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
	logger.On("Error", mock.Anything).Return(nil)
	logger.Infof("test run commands %v", testCase)

	if testCase.mark {
		utilExe = func(log.T, string, []string, string, string, io.Writer, io.Writer, bool) (string, error) {
			return "", nil
		}
	} else {
		utilExe = func(log.T, string, []string, string, string, io.Writer, io.Writer, bool) (string, error) {
			return "", errors.New("err here")
		}
	}

	makeDir = func(destinationDir string) (err error) {
		return nil
	}
	makeArgs = func(context.T, DomainJoinPluginInput) (string, error) {
		return "cmd", nil
	}
	iohandler.DefaultOutputConfig()
	mockCancelFlag := new(task.MockCancelFlag)
	p := &Plugin{
		context: contextmocks.NewMockDefault(),
	}
	if rawInput {
		// prepare plugin input
		var rawPluginInput map[string]interface{}
		err := jsonutil.Remarshal(testCase.Input, &rawPluginInput)
		assert.Nil(t, err)

		p.runCommandsRawInput("-", rawPluginInput, orchestrationDirectory, mockCancelFlag, iohandler.NewDefaultIOHandler(p.context, contracts.IOConfiguration{}), utilExe)
	} else {
		p.runCommands("-", testCase.Input, orchestrationDirectory, mockCancelFlag, iohandler.NewDefaultIOHandler(p.context, contracts.IOConfiguration{}), utilExe)
	}
}

// TestMakeArguments tests the makeArguments methods, which build up the command for domainJoin.exe
func TestMakeArguments(t *testing.T) {
	context := contextmocks.NewMockDefault()

	domainJoinInput := generateDomainJoinPluginInput(testDirectoryId, testDirectoryName, []string{"172.31.4.141", "172.31.21.240"})
	commandRes, _ := makeArguments(context, domainJoinInput)
	expected := "./" + DomainJoinPluginExecutableName + " --directory-id d-0123456789 --directory-name corp.test.com --instance-region us-east-1 --dns-addresses 172.31.4.141 172.31.21.240"

	assert.Equal(t, expected, commandRes)

	domainJoinInput = generateDomainJoinPluginInputOptionalParamSetHostName(testDirectoryId, testDirectoryName, []string{"172.31.4.141", "172.31.21.240"}, testSetHostName)
	commandRes, _ = makeArguments(context, domainJoinInput)
	expected = "./" + DomainJoinPluginExecutableName + " --directory-id d-0123456789 --directory-name corp.test.com --instance-region us-east-1 --set-hostname my_hostname --dns-addresses 172.31.4.141 172.31.21.240"
	assert.Equal(t, expected, commandRes)

	shellInjectionCheck := isShellInjection("`del /Q *`")
	assert.Equal(t, shellInjectionCheck, true, "test failed for `del /Q *`")
	shellInjectionCheck = isShellInjection("echo abc && del /Q *")
	assert.Equal(t, shellInjectionCheck, true, "test failed for echo abc && del /Q *")
	shellInjectionCheck = isShellInjection("echo abc || del /Q *")
	assert.Equal(t, shellInjectionCheck, true, "test failed for echo abc || del /Q *")
	shellInjectionCheck = isShellInjection("echo abc ; del /Q *")
	assert.Equal(t, shellInjectionCheck, true, "test failed for echo abc ; del /Q *")
}
