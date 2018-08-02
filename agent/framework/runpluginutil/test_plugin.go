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

package runpluginutil

import (
	"github.com/aws/amazon-ssm-agent/agent/agentlogstocloudwatch/cloudwatchlogspublisher/cloudwatchlogsinterface"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/s3util"
	"github.com/aws/amazon-ssm-agent/agent/session/datachannel"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/mock"
)

// Note: This code is used in the test files. However, this code is not in a _test.go file
// because then we would have to copy it in every test package that needs the mock.

// Mock stands for a mocked plugin.
type PluginMock struct {
	mock.Mock
}

func (m *PluginMock) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, output iohandler.IOHandler) {
	_ = m.Called(context, config, cancelFlag, output)
	return
}

type PluginFactoryMock struct {
	mock.Mock
}

func (m *PluginFactoryMock) Create(context context.T) (T, error) {
	args := m.Called(context)
	return args.Get(0).(T), args.Error(1)
}

type ISessionPlugin struct {
	mock.Mock
}

// Execute provides a mock function with given fields: _a0, config, cancelFlag, output, dataChannel
func (_m *ISessionPlugin) Execute(_a0 context.T, config contracts.Configuration, cancelFlag task.CancelFlag, output iohandler.IOHandler, dataChannel datachannel.IDataChannel) {
	_ = _m.Called(_a0, config, cancelFlag, output, dataChannel)
	return
}

// GetOnMessageHandler provides a mock function with given fields: _a0, cancelFlag
func (_m *ISessionPlugin) GetOnMessageHandler(_a0 log.T, cancelFlag task.CancelFlag) func([]byte) {
	ret := _m.Called(_a0, cancelFlag)

	var r0 func([]byte)
	if rf, ok := ret.Get(0).(func(log.T, task.CancelFlag) func([]byte)); ok {
		r0 = rf(_a0, cancelFlag)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(func([]byte))
		}
	}

	return r0
}

// Name provides a mock function with given fields:
func (_m *ISessionPlugin) Name() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// Validate provides a mock function with given fields: _a0, config, cwl, s3UploaderUtil
func (_m *ISessionPlugin) Validate(_a0 context.T, config contracts.Configuration, cwl cloudwatchlogsinterface.ICloudWatchLogsService, s3UploaderUtil s3util.IAmazonS3Util) error {
	ret := _m.Called(_a0, config, cwl, s3UploaderUtil)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.T, contracts.Configuration, cloudwatchlogsinterface.ICloudWatchLogsService, s3util.IAmazonS3Util) error); ok {
		r0 = rf(_a0, config, cwl, s3UploaderUtil)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type ISessionPluginFactory struct {
	mock.Mock
}

// Create provides a mock function with given fields: _a0
func (_m *ISessionPluginFactory) Create(_a0 context.T) (SessionPlugin, error) {
	args := _m.Called(_a0)
	return args.Get(0).(SessionPlugin), args.Error(1)
}
