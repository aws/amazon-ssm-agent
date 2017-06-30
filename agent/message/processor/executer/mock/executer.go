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

// Package executer provides interfaces as document execution logic
package executermocks

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	docModel "github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/message/contracts"
	"github.com/aws/amazon-ssm-agent/agent/message/processor/executer"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/mock"
)

//TODO once we replace the callback with channel, we dont need to mock replyBuilder and MDS anymore
// MockedPluginRunner stands for a mock plugin runner.
type MockedPluginRunner struct {
	mock.Mock
}

// RunPlugins mocks a PluginRunner (which is a func).
func (runnerMock *MockedPluginRunner) RunPlugins(context context.T, documentID string, plugins []docModel.PluginState, sendResponse runpluginutil.SendResponse, cancelFlag task.CancelFlag) (pluginOutputs map[string]*contracts.PluginResult) {
	args := runnerMock.Called(context, documentID, plugins, sendResponse, cancelFlag)
	return args.Get(0).(map[string]*contracts.PluginResult)
}

// MockedReplyBuilder stands for a mock reply builder.
type MockedReplyBuilder struct {
	mock.Mock
}

// BuildReply mocks a ReplyBuilder (which is a func).
func (replyBuilderMock *MockedReplyBuilder) BuildReply(pluginID string, pluginResults map[string]*contracts.PluginResult) model.SendReplyPayload {
	args := replyBuilderMock.Called(pluginID, pluginResults)
	return args.Get(0).(model.SendReplyPayload)
}

type MockedExecuter struct {
	mock.Mock
}

func (executerMock MockedExecuter) Run(context context.T,
	cancelFlag task.CancelFlag,
	buildReply executer.ReplyBuilder,
	updateAssoc runpluginutil.UpdateAssociation,
	sendResponse runpluginutil.SendResponse,
	docStore executer.DocumentStore) {
	executerMock.Called(context, cancelFlag, buildReply, sendResponse, docStore)
	return
}

type MockDocumentStore struct {
	mock.Mock
}

func (m MockDocumentStore) Save() {
	m.Called()
	return
}

func (m MockDocumentStore) Load() *docModel.DocumentState {
	args := m.Called()
	return args.Get(0).(*docModel.DocumentState)
}
