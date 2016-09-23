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

// Package lrpminvoker contains implementation of lrpm-invoker plugin. (lrpm - long running plugin manager)
// lrpminvoker is an ondemand worker plugin - which can be called by SSM config or SSM Command.
package lrpminvoker

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/manager"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
)

type TestCase struct {
	Config        contracts.Configuration
	Context       context.T
	ContextResult string
	ContextErr    error
	Message       string
	Result        contracts.PluginResult
	Status        contracts.ResultStatus
}

var TestCases = []TestCase{
	generateTestCaseSuccess("Success_0", "aws:cloudWatch"),
	generateTestCaseSuccess("Success_1", "aws:domainjoin"),
	generateTestCaseFail("Failed_2", "pluginID=<aws:cloudWatch>"),
	generateTestCaseFail("Failed_3", "pluginID=awscloudWatch"),
}

var config = contracts.Configuration{}

var p = new(Plugin)

func generateTestCaseSuccess(msg, id string) TestCase {
	var status = contracts.ResultStatusSuccess
	var res = contracts.PluginResult{
		Status: status,
		Output: msg,
		Code:   0,
	}
	var contextCase = context.NewMockDefaultWithContext([]string{"pluginID=" + id})
	var config = contracts.Configuration{
		Settings: "enable",
	}

	return TestCase{
		Message:       msg,
		Result:        res,
		Status:        status,
		Config:        config,
		Context:       contextCase,
		ContextResult: id,
		ContextErr:    nil,
	}
}

func generateTestCaseFail(msg, id string) TestCase {
	var status = contracts.ResultStatusFailed
	var res = contracts.PluginResult{
		Status: status,
		Output: msg,
		Code:   1,
	}
	var contextCase = context.NewMockDefaultWithContext([]string{id})
	var config = contracts.Configuration{
		Settings: "enable",
	}

	return TestCase{
		Message:       msg,
		Result:        res,
		Status:        status,
		Config:        config,
		Context:       contextCase,
		ContextResult: "",
		ContextErr:    errors.New("unable to parse pluginName from context"),
	}
}

// TestExecuteSuccess tests the cloud watch invoker plugin's Execute method with correct input.
func TestExecuteSuccess(t *testing.T) {
	testCase := TestCases[0]
	pluginPersister = func(log log.T, pluginName string, config contracts.Configuration, res contracts.PluginResult) {}

	//mockS3Uploader := pluginutil.NewMockDefault()
	p.lrpm = manager.NewMockDefault()

	p.ExecuteUploadOutputToS3Bucket = func(log log.T, pluginID string, orchestrationDir string, outputS3BucketName string, outputS3KeyPrefix string, useTempDirectory bool, tempDir string, Stdout string, Stderr string) []string {
		return []string{}
	}

	var cancelFlag = task.NewMockDefault()
	cancelFlag.On("Canceled").Return(false)
	cancelFlag.On("ShutDown").Return(false)

	//var context = context.NewMockDefault()
	var enabledConfig = contracts.Configuration{
		Settings: LongRunningPluginSettings{
			StartType: "Enabled",
		},
	}

	readFile = func(filename string) ([]byte, error) {
		return []byte{}, nil
	}

	res := p.Execute(testCase.Context, enabledConfig, cancelFlag)
	expectRes := p.CreateResult("success", contracts.ResultStatusSuccess)
	assert.Equal(t, expectRes, res)
}

// TestExecuteFailWithContext tests the cloud watch invoker plugin's Execute method with incorrect context.
func TestExecuteFailWithContext(t *testing.T) {
	testCase := TestCases[2]
	pluginPersister = func(log log.T, pluginName string, config contracts.Configuration, res contracts.PluginResult) {}

	var cancelFlag = task.NewMockDefault()
	cancelFlag.On("Canceled").Return(false)
	cancelFlag.On("ShutDown").Return(false)

	res := p.Execute(testCase.Context, config, cancelFlag)
	expectRes := p.CreateResult("Unable to get plugin name because of unsupported plugin name format",
		contracts.ResultStatusFailed)
	assert.Equal(t, expectRes, res)
}

// TestExecuteFailWithStartType tests the cloud watch invoker plugin's Execute method with incorrect start type.
func TestExecuteFailWithStartType(t *testing.T) {
	testCase := TestCases[0]
	pluginPersister = func(log log.T, pluginName string, config contracts.Configuration, res contracts.PluginResult) {}

	var cancelFlag = task.NewMockDefault()
	cancelFlag.On("Canceled").Return(false)
	cancelFlag.On("ShutDown").Return(false)

	res := p.Execute(testCase.Context, config, cancelFlag)
	expectRes := p.CreateResult(fmt.Sprintf("Allowed Values of StartType: Enabled | Disabled"),
		contracts.ResultStatusFailed)
	assert.Equal(t, expectRes, res)
}

// TestExecuteFailWithConfig tests the cloud watch invoker plugin's Execute method with incorrect config.
func TestExecuteFailWithConfig(t *testing.T) {
	testCase := TestCases[0]
	pluginPersister = func(log log.T, pluginName string, config contracts.Configuration, res contracts.PluginResult) {}

	var cancelFlag = task.NewMockDefault()
	cancelFlag.On("Canceled").Return(false)
	cancelFlag.On("ShutDown").Return(false)

	var enabledConfig = contracts.Configuration{
		Settings: "Enabled",
	}
	res := p.Execute(testCase.Context, enabledConfig, cancelFlag)
	expectRes := p.CreateResult(fmt.Sprintf("Unable to parse Settings for %s", testCase.ContextResult),
		contracts.ResultStatusFailed)
	assert.Equal(t, expectRes, res)
}

// TestGetPluginIdFromContext tests the TestGetPluginIdFromContext methods
func TestGetPluginIdFromContext(t *testing.T) {
	for _, testCase := range TestCases {
		res, err := p.GetPluginIdFromContext(testCase.Context)
		assert.Equal(t, testCase.ContextResult, res)
		assert.Equal(t, testCase.ContextErr, err)
	}
}

// TestCreateResult tests the CreateResult method
func TestCreateResult(t *testing.T) {
	for _, testCase := range TestCases {
		var res = p.CreateResult(testCase.Message, testCase.Status)
		assert.Equal(t, testCase.Result, res)
	}

}
