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

package engine

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/mock"
)

// Note: This code is used in the test files. However, this code is not in a _test.go file
// because then we would have to copy it in every test package that needs the mock.

// MockedPluginRunner stands for a mock plugin runner.
type MockedPluginRunner struct {
	mock.Mock
}

// RunPlugins mocks the RunPlugins method.
func (runnerMock *MockedPluginRunner) RunPlugins(context context.T, plugins map[string]model.PluginState, cancelFlag task.CancelFlag) (pluginOutputs map[string]*contracts.PluginResult) {
	args := runnerMock.Called(plugins, cancelFlag)
	return args.Get(0).(map[string]*contracts.PluginResult)
}
