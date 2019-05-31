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

// Package shell is a common library that implements session manager shell.
package shell

import (
	context "github.com/aws/amazon-ssm-agent/agent/context"
	contracts "github.com/aws/amazon-ssm-agent/agent/contracts"
	iohandler "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	log "github.com/aws/amazon-ssm-agent/agent/log"
	sessioncontracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	datachannel "github.com/aws/amazon-ssm-agent/agent/session/datachannel"
	mock "github.com/stretchr/testify/mock"

	task "github.com/aws/amazon-ssm-agent/agent/task"
)

// Mock stands for a mocked context.
type IShellPluginMock struct {
	mock.Mock
}

// Execute provides a mock function with given fields: _a0, config, cancelFlag, output, dataChannel
func (_m *IShellPluginMock) Execute(_a0 context.T, config contracts.Configuration, cancelFlag task.CancelFlag, output iohandler.IOHandler, dataChannel datachannel.IDataChannel, shellProps sessioncontracts.ShellProperties) {
	_m.Called(_a0, config, cancelFlag, output, dataChannel, shellProps)
}

// InputStreamMessageHandler provides a mock function with given fields: _a0, streamDataMessage
func (_m *IShellPluginMock) InputStreamMessageHandler(_a0 log.T, streamDataMessage sessioncontracts.AgentMessage) error {
	ret := _m.Called(_a0, streamDataMessage)

	var r0 error
	if rf, ok := ret.Get(0).(func(log.T, sessioncontracts.AgentMessage) error); ok {
		r0 = rf(_a0, streamDataMessage)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
