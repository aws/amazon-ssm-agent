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
//go:build freebsd || linux || netbsd || openbsd || darwin
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
	contextmocks "github.com/aws/amazon-ssm-agent/agent/mocks/context"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/agent/mocks/task"
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
	orchestrationDirectory         = "OrchesDir"
	s3BucketName                   = "bucket"
	s3KeyPrefix                    = "key"
	testInstanceID                 = "i-12345678"
	bucketRegionErrorMsg           = "AuthorizationHeaderMalformed: The authorization header is malformed; the region 'us-east-1' is wrong; expecting 'us-west-2' status code: 400, request id: []"
	testDirectoryName              = "corp.test.com"
	testDirectoryId                = "d-0123456789"
	testDirectoryOU                = "OU=test,OU=corp,DC=test,DC=com"
	testDirectoryOUWithSpace       = "OU=test with space,OU=corp,DC=test,DC=com"
	testDirectoryOUWithSpaceQuoted = "\"OU=test with space,OU=corp,DC=test,DC=com\""
	testKeepHostName               = true
	testSetHostName                = "my_hostname"
	testSetHostNameNumAppendDigits = "5"
)

var TestCases = []TestCase{
	generateTestCaseOk(testDirectoryId, testDirectoryName, "", []string{"10.0.0.0", "10.0.1.0"}),
	generateTestCaseFail(testDirectoryId, testDirectoryName, "", []string{"10.0.0.2", "10.0.1.2"}),
}

func generateTestCaseOk(id string, name string, ou string, ipAddress []string) TestCase {

	testCase := TestCase{
		Input:  generateDomainJoinPluginInput(id, name, ou, ipAddress),
		Output: iohandler.DefaultIOHandler{},
		mark:   true,
	}

	testCase.Output.SetStdout("")
	testCase.Output.SetStderr("")
	testCase.Output.ExitCode = 0
	testCase.Output.Status = "Success"

	return testCase
}

func generateTestCaseFail(id string, name string, ou string, ipAddress []string) TestCase {
	testCase := TestCase{
		Input:  generateDomainJoinPluginInput(id, name, ou, ipAddress),
		Output: iohandler.DefaultIOHandler{},
		mark:   false,
	}

	testCase.Output.SetStdout("")
	testCase.Output.SetStderr("")
	testCase.Output.ExitCode = 1
	testCase.Output.Status = "Failed"

	return testCase
}

func generateDomainJoinPluginInput(id string, name string, ou string, ipAddress []string) DomainJoinPluginInput {
	return DomainJoinPluginInput{
		DirectoryId:    id,
		DirectoryName:  name,
		DirectoryOU:    ou,
		DnsIpAddresses: ipAddress,
	}
}

func generateDomainJoinPluginInputOptionalParamSetHostName(id string, name string, ou string, ipAddress []string, setHostName string) DomainJoinPluginInput {
	return DomainJoinPluginInput{
		DirectoryId:    id,
		DirectoryName:  name,
		DirectoryOU:    ou,
		DnsIpAddresses: ipAddress,
		HostName:       setHostName,
	}
}

func generateDomainJoinPluginInputOptionalParamSetHostNameWithAppendDigits(id string, name string, ou string, ipAddress []string, setHostName string, setHostNameNumAppendDigits string) DomainJoinPluginInput {
	return DomainJoinPluginInput{
		DirectoryId:             id,
		DirectoryName:           name,
		DirectoryOU:             ou,
		DnsIpAddresses:          ipAddress,
		HostName:                setHostName,
		HostNameNumAppendDigits: setHostNameNumAppendDigits,
	}
}

func generateDomainJoinPluginInputOptionalParamKeepHostName(id string, name string, ou string, ipAddress []string, keepHostName bool) DomainJoinPluginInput {
	return DomainJoinPluginInput{
		DirectoryId:    id,
		DirectoryName:  name,
		DirectoryOU:    ou,
		DnsIpAddresses: ipAddress,
		KeepHostName:   keepHostName,
	}
}

func generateDomainJoinPluginInputOptionalParamKeepHostNameNoIPs(id string, name string, ou string, keepHostName bool) DomainJoinPluginInput {
	return DomainJoinPluginInput{
		DirectoryId:   id,
		DirectoryName: name,
		DirectoryOU:   ou,
		KeepHostName:  keepHostName,
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
		context: contextmocks.NewMockDefault(),
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

// TestMakeArgumentsAndCommandParts tests the makeArguments and makeCommandParts methods, which build up the command for aws_domainjoin.sh
func TestMakeArgumentsAndCommandParts(t *testing.T) {
	contextMock := &contextmocks.Mock{}
	context := contextmocks.NewMockDefault()
	domainJoinInput := generateDomainJoinPluginInput(testDirectoryId, testDirectoryName, "", []string{})
	commandLine, _ := makeArguments(context, "./aws_domainjoin.sh", domainJoinInput)
	expectedCommandLine := "./aws_domainjoin.sh --directory-id d-0123456789 --directory-name corp.test.com --instance-region us-east-1"
	assert.Equal(t, expectedCommandLine, commandLine)
	commandParts, _ := makeCommandParts(commandLine)
	expectedCommandParts := []string{
		"./aws_domainjoin.sh",
		"--directory-id",
		"d-0123456789",
		"--directory-name",
		"corp.test.com",
		"--instance-region",
		"us-east-1",
	}
	assert.Equal(t, expectedCommandParts, commandParts)

	domainJoinInput = generateDomainJoinPluginInput(testDirectoryId, testDirectoryName, testDirectoryOU, []string{"172.31.4.141", "172.31.21.240"})
	commandLine, _ = makeArguments(context, "./aws_domainjoin.sh", domainJoinInput)
	expectedCommandLine = "./aws_domainjoin.sh --directory-id d-0123456789 --directory-name corp.test.com --instance-region us-east-1 --directory-ou 'OU=test,OU=corp,DC=test,DC=com' --dns-addresses 172.31.4.141,172.31.21.240"
	assert.Equal(t, expectedCommandLine, commandLine)
	commandParts, _ = makeCommandParts(commandLine)
	expectedCommandParts = []string{
		"./aws_domainjoin.sh",
		"--directory-id",
		"d-0123456789",
		"--directory-name",
		"corp.test.com",
		"--instance-region",
		"us-east-1",
		"--directory-ou",
		"OU=test,OU=corp,DC=test,DC=com",
		"--dns-addresses",
		"172.31.4.141,172.31.21.240",
	}
	assert.Equal(t, expectedCommandParts, commandParts)

	domainJoinInput = generateDomainJoinPluginInput(testDirectoryId, testDirectoryName, testDirectoryOUWithSpace, []string{"172.31.4.141", "172.31.21.240"})
	commandLine, _ = makeArguments(context, "./aws_domainjoin.sh", domainJoinInput)
	expectedCommandLine = "./aws_domainjoin.sh --directory-id d-0123456789 --directory-name corp.test.com --instance-region us-east-1 --directory-ou 'OU=test with space,OU=corp,DC=test,DC=com' --dns-addresses 172.31.4.141,172.31.21.240"
	assert.Equal(t, expectedCommandLine, commandLine)
	commandParts, _ = makeCommandParts(commandLine)
	expectedCommandParts = []string{
		"./aws_domainjoin.sh",
		"--directory-id",
		"d-0123456789",
		"--directory-name",
		"corp.test.com",
		"--instance-region",
		"us-east-1",
		"--directory-ou",
		"OU=test with space,OU=corp,DC=test,DC=com",
		"--dns-addresses",
		"172.31.4.141,172.31.21.240",
	}
	assert.Equal(t, expectedCommandParts, commandParts)

	domainJoinInput = generateDomainJoinPluginInput(testDirectoryId, testDirectoryName, testDirectoryOUWithSpaceQuoted, []string{"172.31.4.141", "172.31.21.240"})
	commandLine, _ = makeArguments(context, "./aws_domainjoin.sh", domainJoinInput)
	expectedCommandLine = "./aws_domainjoin.sh --directory-id d-0123456789 --directory-name corp.test.com --instance-region us-east-1 --directory-ou 'OU=test with space,OU=corp,DC=test,DC=com' --dns-addresses 172.31.4.141,172.31.21.240"
	assert.Equal(t, expectedCommandLine, commandLine)
	commandParts, _ = makeCommandParts(commandLine)
	expectedCommandParts = []string{
		"./aws_domainjoin.sh",
		"--directory-id",
		"d-0123456789",
		"--directory-name",
		"corp.test.com",
		"--instance-region",
		"us-east-1",
		"--directory-ou",
		"OU=test with space,OU=corp,DC=test,DC=com",
		"--dns-addresses",
		"172.31.4.141,172.31.21.240",
	}
	assert.Equal(t, expectedCommandParts, commandParts)

	domainJoinInput = generateDomainJoinPluginInput(testDirectoryId, testDirectoryName, "", []string{"8.8.8.8", "8.8.8.8[[["})
	commandLine, _ = makeArguments(context, "./aws_domainjoin.sh", domainJoinInput)
	expectedCommandLine = ""
	assert.Equal(t, expectedCommandLine, commandLine)

	domainJoinInput = generateDomainJoinPluginInput(testDirectoryId, testDirectoryName, "", []string{"8.8.8.8[[[", "8.8.8.8"})
	commandLine, _ = makeArguments(context, "./aws_domainjoin.sh", domainJoinInput)
	expectedCommandLine = ""
	assert.Equal(t, expectedCommandLine, commandLine)

	domainJoinInput = generateDomainJoinPluginInput(testDirectoryId, testDirectoryName, "", []string{"Hello $(aws s3 ls)", "8.8.8.8"})
	commandLine, _ = makeArguments(context, "./aws_domainjoin.sh", domainJoinInput)
	expectedCommandLine = ""
	assert.Equal(t, expectedCommandLine, commandLine)

	domainJoinInput = generateDomainJoinPluginInput(testDirectoryId, testDirectoryName, "", []string{"Hello `aws s3 ls`", "8.8.8.8"})
	commandLine, _ = makeArguments(context, "./aws_domainjoin.sh", domainJoinInput)
	expectedCommandLine = ""
	assert.Equal(t, expectedCommandLine, commandLine)

	domainJoinInput = generateDomainJoinPluginInputOptionalParamSetHostName(testDirectoryId, testDirectoryName, "", []string{"172.31.4.141", "172.31.21.240"}, testSetHostName)
	commandLine, _ = makeArguments(context, "./aws_domainjoin.sh", domainJoinInput)
	expectedCommandLine = "./aws_domainjoin.sh --directory-id d-0123456789 --directory-name corp.test.com --instance-region us-east-1 --set-hostname my_hostname --dns-addresses 172.31.4.141,172.31.21.240"
	assert.Equal(t, expectedCommandLine, commandLine)
	commandParts, _ = makeCommandParts(commandLine)
	expectedCommandParts = []string{
		"./aws_domainjoin.sh",
		"--directory-id",
		"d-0123456789",
		"--directory-name",
		"corp.test.com",
		"--instance-region",
		"us-east-1",
		"--set-hostname",
		"my_hostname",
		"--dns-addresses",
		"172.31.4.141,172.31.21.240",
	}
	assert.Equal(t, expectedCommandParts, commandParts)

	domainJoinInput = generateDomainJoinPluginInputOptionalParamSetHostNameWithAppendDigits(testDirectoryId, testDirectoryName, "", []string{"172.31.4.141", "172.31.21.240"}, testSetHostName, testSetHostNameNumAppendDigits)
	commandLine, _ = makeArguments(context, "./aws_domainjoin.sh", domainJoinInput)
	expectedCommandLine = "./aws_domainjoin.sh --directory-id d-0123456789 --directory-name corp.test.com --instance-region us-east-1 --set-hostname my_hostname --set-hostname-append-num-digits 5 --dns-addresses 172.31.4.141,172.31.21.240"
	assert.Equal(t, expectedCommandLine, commandLine)
	commandParts, _ = makeCommandParts(commandLine)
	expectedCommandParts = []string{
		"./aws_domainjoin.sh",
		"--directory-id",
		"d-0123456789",
		"--directory-name",
		"corp.test.com",
		"--instance-region",
		"us-east-1",
		"--set-hostname",
		"my_hostname",
		"--set-hostname-append-num-digits",
		"5",
		"--dns-addresses",
		"172.31.4.141,172.31.21.240",
	}
	assert.Equal(t, expectedCommandParts, commandParts)

	// If num append digits is set to a non-number, the plugin must fail
	domainJoinInput = generateDomainJoinPluginInputOptionalParamSetHostNameWithAppendDigits(testDirectoryId, testDirectoryName, "", []string{"172.31.4.141", "172.31.21.240"}, testSetHostName, "1I")
	commandLine, _ = makeArguments(context, "./aws_domainjoin.sh", domainJoinInput)
	expectedCommandLine = ""
	assert.Equal(t, expectedCommandLine, commandLine)

	// If region is not set, the domain join plugin must fail
	domainJoinInput = generateDomainJoinPluginInput(testDirectoryId, testDirectoryName, "", []string{"172.31.4.141", "172.31.21.240"})
	identityMock := &identityMocks.IAgentIdentity{}
	identityMock.On("Region").Return("", nil) // Remove region setting
	contextMock.On("Identity").Return(identityMock)
	contextMock.On("Log").Return(logmocks.NewMockLog())
	commandLine, _ = makeArguments(contextMock, "./aws_domainjoin.sh", domainJoinInput)
	expectedCommandLine = ""
	assert.Equal(t, expectedCommandLine, commandLine)

	domainJoinInput = generateDomainJoinPluginInputOptionalParamKeepHostName(testDirectoryId, testDirectoryName, "", []string{"172.31.4.141", "172.31.21.240"}, testKeepHostName)
	commandLine, _ = makeArguments(context, "./aws_domainjoin.sh", domainJoinInput)
	expectedCommandLine = "./aws_domainjoin.sh --directory-id d-0123456789 --directory-name corp.test.com --instance-region us-east-1 --keep-hostname   --dns-addresses 172.31.4.141,172.31.21.240"
	assert.Equal(t, expectedCommandLine, commandLine)
	commandParts, _ = makeCommandParts(commandLine)
	expectedCommandParts = []string{
		"./aws_domainjoin.sh",
		"--directory-id",
		"d-0123456789",
		"--directory-name",
		"corp.test.com",
		"--instance-region",
		"us-east-1",
		"--keep-hostname",
		"my_hostname",
		"--dns-addresses",
		"172.31.4.141,172.31.21.240",
	}

	domainJoinInput = generateDomainJoinPluginInputOptionalParamKeepHostNameNoIPs(testDirectoryId, testDirectoryName, "", testKeepHostName)
	commandLine, _ = makeArguments(context, "./aws_domainjoin.sh", domainJoinInput)
	expectedCommandLine = "./aws_domainjoin.sh --directory-id d-0123456789 --directory-name corp.test.com --instance-region us-east-1 --keep-hostname  "
	assert.Equal(t, expectedCommandLine, commandLine)
	commandParts, _ = makeCommandParts(commandLine)
	expectedCommandParts = []string{
		"./aws_domainjoin.sh",
		"--directory-id",
		"d-0123456789",
		"--directory-name",
		"corp.test.com",
		"--instance-region",
		"us-east-1",
		"--keep-hostname",
	}
	assert.Equal(t, expectedCommandParts, commandParts)

	var shellInjectionCheck = isShellInjection("$(rm *)")
	assert.Equal(t, shellInjectionCheck, true, "test failed for $(rm *)")
	shellInjectionCheck = isShellInjection("`rm *`")
	assert.Equal(t, shellInjectionCheck, true, "test failed for `rm *`")
	shellInjectionCheck = isShellInjection("echo abc && rm *")
	assert.Equal(t, shellInjectionCheck, true, "test failed for echo abc && rm *")
	shellInjectionCheck = isShellInjection("echo abc || rm *")
	assert.Equal(t, shellInjectionCheck, true, "test failed for echo abc || rm *")
	shellInjectionCheck = isShellInjection("echo abc ; rm *")
	assert.Equal(t, shellInjectionCheck, true, "test failed for echo abc ; rm *")
}
