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
	testKeepHostNameBool           = true
	testKeepHostNameString         = "true"
	testNotKeepHostNameString      = "FALSE"
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

func generateDomainJoinPluginInputOptionalParamKeepHostName(id string, name string, ou string, ipAddress []string, keepHostName interface{}) DomainJoinPluginInput {
	return DomainJoinPluginInput{
		DirectoryId:    id,
		DirectoryName:  name,
		DirectoryOU:    ou,
		DnsIpAddresses: ipAddress,
		KeepHostName:   keepHostName,
	}
}

func generateDomainJoinPluginInputOptionalParamKeepHostNameNoIPs(id string, name string, ou string, keepHostName string) DomainJoinPluginInput {
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
	expectedCommandLine := "./aws_domainjoin.sh --directory-id 'd-0123456789' --directory-name 'corp.test.com' --instance-region us-east-1"
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
	expectedCommandLine = "./aws_domainjoin.sh --directory-id 'd-0123456789' --directory-name 'corp.test.com' --instance-region us-east-1 --directory-ou 'OU=test,OU=corp,DC=test,DC=com' --dns-addresses 172.31.4.141,172.31.21.240"
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
	expectedCommandLine = "./aws_domainjoin.sh --directory-id 'd-0123456789' --directory-name 'corp.test.com' --instance-region us-east-1 --directory-ou 'OU=test with space,OU=corp,DC=test,DC=com' --dns-addresses 172.31.4.141,172.31.21.240"
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
	expectedCommandLine = "./aws_domainjoin.sh --directory-id 'd-0123456789' --directory-name 'corp.test.com' --instance-region us-east-1 --directory-ou 'OU=test with space,OU=corp,DC=test,DC=com' --dns-addresses 172.31.4.141,172.31.21.240"
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
	expectedCommandLine = "./aws_domainjoin.sh --directory-id 'd-0123456789' --directory-name 'corp.test.com' --instance-region us-east-1 --set-hostname 'my_hostname' --dns-addresses 172.31.4.141,172.31.21.240"
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
	expectedCommandLine = "./aws_domainjoin.sh --directory-id 'd-0123456789' --directory-name 'corp.test.com' --instance-region us-east-1 --set-hostname 'my_hostname' --set-hostname-append-num-digits 5 --dns-addresses 172.31.4.141,172.31.21.240"
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

	domainJoinInput = generateDomainJoinPluginInputOptionalParamKeepHostName(testDirectoryId, testDirectoryName, "", []string{"172.31.4.141", "172.31.21.240"}, testKeepHostNameBool)
	commandLine, _ = makeArguments(context, "./aws_domainjoin.sh", domainJoinInput)
	expectedCommandLine = "./aws_domainjoin.sh --directory-id 'd-0123456789' --directory-name 'corp.test.com' --instance-region us-east-1 --keep-hostname   --dns-addresses 172.31.4.141,172.31.21.240"
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
		"--dns-addresses",
		"172.31.4.141,172.31.21.240",
	}
	assert.Equal(t, expectedCommandParts, commandParts)

	domainJoinInput = generateDomainJoinPluginInputOptionalParamKeepHostName(testDirectoryId, testDirectoryName, "", []string{"172.31.4.141", "172.31.21.240"}, testNotKeepHostNameString)
	commandLine, _ = makeArguments(context, "./aws_domainjoin.sh", domainJoinInput)
	expectedCommandLine = "./aws_domainjoin.sh --directory-id 'd-0123456789' --directory-name 'corp.test.com' --instance-region us-east-1 --dns-addresses 172.31.4.141,172.31.21.240"
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
		"--dns-addresses",
		"172.31.4.141,172.31.21.240",
	}
	assert.Equal(t, expectedCommandParts, commandParts)

	domainJoinInput = generateDomainJoinPluginInputOptionalParamKeepHostNameNoIPs(testDirectoryId, testDirectoryName, "", testKeepHostNameString)
	commandLine, _ = makeArguments(context, "./aws_domainjoin.sh", domainJoinInput)
	expectedCommandLine = "./aws_domainjoin.sh --directory-id 'd-0123456789' --directory-name 'corp.test.com' --instance-region us-east-1 --keep-hostname  "
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

// invalidParameterCharacters returns the characters that must never appear in a
// domain-join parameter value: the shell metacharacters already covered by the
// denylist, the single quote used to wrap the value for single-token parsing,
// and the newline / carriage return that would introduce an extra token when
// the command line is re-parsed. These are character-class probes, not working
// commands.
func invalidParameterCharacters() []string {
	return []string{"$", "`", ";", "|", "&", "'", "\n", "\r"}
}

// TestIsShellInjection_RejectsQuoteAndNewline locks the denylist contract at the
// function level: the single quote and the newline / carriage return must be
// rejected in addition to the previously covered shell metacharacters.
func TestIsShellInjection_RejectsQuoteAndNewline(t *testing.T) {
	assert.True(t, isShellInjection("abc'def"), "single quote must be rejected")
	assert.True(t, isShellInjection("abc\ndef"), "newline must be rejected")
	assert.True(t, isShellInjection("abc\rdef"), "carriage return must be rejected")
	// A legitimate value with none of these characters must still be accepted.
	assert.False(t, isShellInjection("OU=My Servers,DC=corp,DC=example,DC=com"))
}

// TestMakeArguments_RejectsInvalidCharactersInDirectoryId proves makeArguments
// rejects a DirectoryId that contains any disallowed character.
func TestMakeArguments_RejectsInvalidCharactersInDirectoryId(t *testing.T) {
	context := contextmocks.NewMockDefault()
	for _, bad := range invalidParameterCharacters() {
		input := generateDomainJoinPluginInput("d-012"+bad, testDirectoryName, "", []string{"10.0.0.1"})
		commandLine, err := makeArguments(context, "./aws_domainjoin.sh", input)
		assert.Empty(t, commandLine, "expected rejection for DirectoryId containing %q", bad)
		assert.Error(t, err, "expected error for DirectoryId containing %q", bad)
	}
}

// TestMakeArguments_RejectsInvalidCharactersInDirectoryName proves makeArguments
// rejects a DirectoryName that contains any disallowed character.
func TestMakeArguments_RejectsInvalidCharactersInDirectoryName(t *testing.T) {
	context := contextmocks.NewMockDefault()
	for _, bad := range invalidParameterCharacters() {
		input := generateDomainJoinPluginInput(testDirectoryId, "corp.test.com"+bad, "", []string{"10.0.0.1"})
		commandLine, err := makeArguments(context, "./aws_domainjoin.sh", input)
		assert.Empty(t, commandLine, "expected rejection for DirectoryName containing %q", bad)
		assert.Error(t, err, "expected error for DirectoryName containing %q", bad)
	}
}

// TestMakeArguments_RejectsInvalidCharactersInDirectoryOU proves makeArguments
// rejects a DirectoryOU that contains any disallowed character. DirectoryOU is
// single-quote wrapped for single-token parsing, so a single quote in the value
// would otherwise terminate the wrapping.
func TestMakeArguments_RejectsInvalidCharactersInDirectoryOU(t *testing.T) {
	context := contextmocks.NewMockDefault()
	for _, bad := range invalidParameterCharacters() {
		input := generateDomainJoinPluginInput(testDirectoryId, testDirectoryName, "OU=test"+bad+",DC=corp", []string{"10.0.0.1"})
		commandLine, err := makeArguments(context, "./aws_domainjoin.sh", input)
		assert.Empty(t, commandLine, "expected rejection for DirectoryOU containing %q", bad)
		assert.Error(t, err, "expected error for DirectoryOU containing %q", bad)
	}
}

// TestMakeArguments_RejectsInvalidCharactersInHostName proves makeArguments
// rejects a HostName that contains any disallowed character.
func TestMakeArguments_RejectsInvalidCharactersInHostName(t *testing.T) {
	context := contextmocks.NewMockDefault()
	for _, bad := range invalidParameterCharacters() {
		input := generateDomainJoinPluginInputOptionalParamSetHostName(testDirectoryId, testDirectoryName, "", []string{"10.0.0.1"}, "host"+bad+"name")
		commandLine, err := makeArguments(context, "./aws_domainjoin.sh", input)
		assert.Empty(t, commandLine, "expected rejection for HostName containing %q", bad)
		assert.Error(t, err, "expected error for HostName containing %q", bad)
	}
}

// TestMakeArguments_LegitimateValuesParseAsSingleTokens proves the token-integrity
// contract: legitimate values (including an OU and a hostname that contain
// spaces) survive the re-parse as single tokens rather than being split into
// extra arguments to the helper script.
func TestMakeArguments_LegitimateValuesParseAsSingleTokens(t *testing.T) {
	context := contextmocks.NewMockDefault()
	input := generateDomainJoinPluginInputOptionalParamSetHostName(
		"d-0123456789",
		"corp.example.com",
		"OU=My Servers,DC=corp,DC=example,DC=com",
		[]string{"172.31.4.141", "172.31.21.240"},
		"my_hostname",
	)
	commandLine, err := makeArguments(context, "./aws_domainjoin.sh", input)
	assert.NoError(t, err)
	assert.NotEmpty(t, commandLine)

	parts, err := makeCommandParts(commandLine)
	assert.NoError(t, err)
	assert.Contains(t, parts, "d-0123456789")
	assert.Contains(t, parts, "corp.example.com")
	// The OU with a space must be a single token, not two.
	assert.Contains(t, parts, "OU=My Servers,DC=corp,DC=example,DC=com")
	assert.Contains(t, parts, "my_hostname")
	assert.Contains(t, parts, "172.31.4.141,172.31.21.240")
}

// TestRunCommandsRawInput_RejectsInvalidDocumentParameters is an integration
// test over the full document-parameter path: parameters arrive as the raw
// map that runCommandsRawInput receives from the document, flow through
// runCommands into the real argument builder, and a value carrying a character
// that is not permitted in a domain-join value must be rejected end-to-end -
// the command is marked failed and is never handed to the executer. This guards
// against a regression that re-opens the validation at any layer of that path.
func TestRunCommandsRawInput_RejectsInvalidDocumentParameters(t *testing.T) {
	// Use the real argument builder, not the stub used elsewhere in this file.
	makeArgs = makeArguments
	makeDir = func(destinationDir string) (err error) { return nil }
	createOrchesDir = func(log log.T, orchestrationDir string, pluginInput DomainJoinPluginInput) (scriptPath string, err error) {
		return "./aws_domainjoin.sh", nil
	}

	executed := false
	exe := func(log.T, string, []string, string, string, io.Writer, io.Writer, bool) (string, error) {
		executed = true
		return "Success", nil
	}

	cases := []struct {
		name  string
		input DomainJoinPluginInput
	}{
		{"single-quote in DirectoryOU", generateDomainJoinPluginInput(testDirectoryId, testDirectoryName, "OU=test',DC=corp", []string{"10.0.0.1"})},
		{"newline in DirectoryName", generateDomainJoinPluginInput(testDirectoryId, "corp.test.com\nextra", "", []string{"10.0.0.1"})},
		{"metacharacter in DirectoryId", generateDomainJoinPluginInput("d-012;id", testDirectoryName, "", []string{"10.0.0.1"})},
		{"carriage-return in HostName", generateDomainJoinPluginInputOptionalParamSetHostName(testDirectoryId, testDirectoryName, "", []string{"10.0.0.1"}, "host\rname")},
	}

	for _, tc := range cases {
		executed = false

		var rawPluginInput map[string]interface{}
		err := jsonutil.Remarshal(tc.input, &rawPluginInput)
		assert.Nil(t, err)

		p := &Plugin{context: contextmocks.NewMockDefault()}
		mockCancelFlag := new(task.MockCancelFlag)
		mockIOHandler := new(iohandlermocks.MockIOHandler)
		mockIOHandler.On("MarkAsFailed", mock.Anything).Return()

		p.runCommandsRawInput("domainjoin", rawPluginInput, orchestrationDirectory, mockCancelFlag, mockIOHandler, exe)

		mockIOHandler.AssertCalled(t, "MarkAsFailed", mock.Anything)
		mockIOHandler.AssertNotCalled(t, "MarkAsSucceeded")
		assert.False(t, executed, "executer must not run for rejected document input: %s", tc.name)
	}
}

// TestRunCommandsRawInput_AcceptsLegitimateDocumentParameters is the positive
// half of the integration path: a legitimate set of document parameters
// (including an OU with a space) must still complete successfully, proving the
// validation does not reject valid domain-join requests.
func TestRunCommandsRawInput_AcceptsLegitimateDocumentParameters(t *testing.T) {
	makeArgs = makeArguments
	makeDir = func(destinationDir string) (err error) { return nil }
	createOrchesDir = func(log log.T, orchestrationDir string, pluginInput DomainJoinPluginInput) (scriptPath string, err error) {
		return "./aws_domainjoin.sh", nil
	}
	exe := func(log.T, string, []string, string, string, io.Writer, io.Writer, bool) (string, error) {
		return "Success", nil
	}

	input := generateDomainJoinPluginInput(testDirectoryId, testDirectoryName, testDirectoryOUWithSpace, []string{"172.31.4.141", "172.31.21.240"})
	var rawPluginInput map[string]interface{}
	err := jsonutil.Remarshal(input, &rawPluginInput)
	assert.Nil(t, err)

	p := &Plugin{context: contextmocks.NewMockDefault()}
	mockCancelFlag := new(task.MockCancelFlag)
	mockIOHandler := new(iohandlermocks.MockIOHandler)
	mockIOHandler.On("SetStatus", mock.Anything).Return()
	mockIOHandler.On("GetStdoutWriter", mock.Anything).Return(new(multiwritermock.MockDocumentIOMultiWriter))
	mockIOHandler.On("GetStderrWriter", mock.Anything).Return(new(multiwritermock.MockDocumentIOMultiWriter))
	mockIOHandler.On("MarkAsSucceeded").Return()

	p.runCommandsRawInput("domainjoin", rawPluginInput, orchestrationDirectory, mockCancelFlag, mockIOHandler, exe)

	mockIOHandler.AssertCalled(t, "MarkAsSucceeded")
	mockIOHandler.AssertNotCalled(t, "MarkAsFailed", mock.Anything)
}

// TestIsValidDirectoryId tests that isValidDirectoryId accepts only the
// canonical d-xxxxxxxxxx form (ten lowercase hex chars).
func TestIsValidDirectoryId(t *testing.T) {
	valid := []string{"d-0123456789", "d-abcdef0123", "d-00000000aa"}
	for _, v := range valid {
		assert.True(t, isValidDirectoryId(v), "expected %q to be a valid directory id", v)
	}

	invalid := []string{
		"",
		"d-0123456789 --endpoint-url=https://actor", // space + injected flag
		"d-0123456789 ",   // trailing space
		"d-012345678",     // too short
		"d-01234567890",   // too long
		"d-ABCDEF0123",    // uppercase hex not allowed
		"d-01234567zz",    // non-hex
		"x-0123456789",    // wrong prefix
		"d-0123456789;id", // metacharacter
	}
	for _, v := range invalid {
		assert.False(t, isValidDirectoryId(v), "expected %q to be rejected", v)
	}
}

// TestIsValidDirectoryName tests that isValidDirectoryName accepts only
// DNS-name characters (letters, digits, dot, hyphen).
func TestIsValidDirectoryName(t *testing.T) {
	valid := []string{"corp.test.com", "example.com", "my-corp.example.co.uk", "a.b"}
	for _, v := range valid {
		assert.True(t, isValidDirectoryName(v), "expected %q to be a valid directory name", v)
	}

	invalid := []string{
		"",
		"evil.example -H ldap://actor", // space + injected flag
		"corp.test.com ",               // trailing space
		"corp.test.com\nextra",         // newline
		"corp.test.com;id",             // metacharacter
		"corp_test_com",                // underscore not in DNS label set
		"-foo",                         // leading hyphen (parsed as a flag)
		"--version",                    // leading double hyphen (parsed as a flag)
		"corp.test.com-",               // trailing hyphen
		".corp.test.com",               // leading dot
	}
	for _, v := range invalid {
		assert.False(t, isValidDirectoryName(v), "expected %q to be rejected", v)
	}
}

// TestMakeArguments_RejectsSpaceInjectedDirectoryId tests that makeArguments
// rejects a DirectoryId that smuggles an extra argument using a space.
func TestMakeArguments_RejectsSpaceInjectedDirectoryId(t *testing.T) {
	context := contextmocks.NewMockDefault()
	input := generateDomainJoinPluginInput("d-0123456789 --endpoint-url=https://actor", testDirectoryName, "", []string{"10.0.0.1"})
	commandLine, err := makeArguments(context, "./aws_domainjoin.sh", input)
	assert.Empty(t, commandLine)
	assert.Error(t, err)
}

// TestMakeArguments_RejectsSpaceInjectedDirectoryName tests that makeArguments
// rejects a DirectoryName that smuggles an extra argument using a space.
func TestMakeArguments_RejectsSpaceInjectedDirectoryName(t *testing.T) {
	context := contextmocks.NewMockDefault()
	input := generateDomainJoinPluginInput(testDirectoryId, "evil.example -H ldap://actor", "", []string{"10.0.0.1"})
	commandLine, err := makeArguments(context, "./aws_domainjoin.sh", input)
	assert.Empty(t, commandLine)
	assert.Error(t, err)
}

// TestRunCommandsRawInput_RejectsSpaceInjectedDocumentParameters tests that a
// DirectoryId or DirectoryName carrying a space and an extra argument is
// rejected end-to-end (command marked failed, executer never invoked).
func TestRunCommandsRawInput_RejectsSpaceInjectedDocumentParameters(t *testing.T) {
	makeArgs = makeArguments
	makeDir = func(destinationDir string) (err error) { return nil }
	createOrchesDir = func(log log.T, orchestrationDir string, pluginInput DomainJoinPluginInput) (scriptPath string, err error) {
		return "./aws_domainjoin.sh", nil
	}

	executed := false
	exe := func(log.T, string, []string, string, string, io.Writer, io.Writer, bool) (string, error) {
		executed = true
		return "Success", nil
	}

	cases := []struct {
		name  string
		input DomainJoinPluginInput
	}{
		{"space-injected DirectoryId", generateDomainJoinPluginInput("d-0123456789 --endpoint-url=https://actor", testDirectoryName, "", []string{"10.0.0.1"})},
		{"space-injected DirectoryName", generateDomainJoinPluginInput(testDirectoryId, "evil.example -H ldap://actor", "", []string{"10.0.0.1"})},
	}

	for _, tc := range cases {
		executed = false

		var rawPluginInput map[string]interface{}
		err := jsonutil.Remarshal(tc.input, &rawPluginInput)
		assert.Nil(t, err)

		p := &Plugin{context: contextmocks.NewMockDefault()}
		mockCancelFlag := new(task.MockCancelFlag)
		mockIOHandler := new(iohandlermocks.MockIOHandler)
		mockIOHandler.On("MarkAsFailed", mock.Anything).Return()

		p.runCommandsRawInput("domainjoin", rawPluginInput, orchestrationDirectory, mockCancelFlag, mockIOHandler, exe)

		mockIOHandler.AssertCalled(t, "MarkAsFailed", mock.Anything)
		mockIOHandler.AssertNotCalled(t, "MarkAsSucceeded")
		assert.False(t, executed, "executer must not run for rejected document input: %s", tc.name)
	}
}
