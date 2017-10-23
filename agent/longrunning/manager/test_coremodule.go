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

// Package manager encapsulates everything related to long running plugin manager that starts, stops & configures long running plugins
package manager

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	managerContracts "github.com/aws/amazon-ssm-agent/agent/longrunning/plugin"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/plugin/cloudwatch"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/mock"
)

// Note: This code is used in the test files. However, this code is not in a _test.go file
// because then we would have to copy it in every test package that needs the mock.

// Mock stands for a mocked manager.
type Mock struct {
	mock.Mock
}

// CloudWatchId represents the ID of cloud watch plugin
const CloudWatchId = "aws:cloudWatch"

// NewMockDefault returns an instance of Mock with default expectations set.
func NewMockDefault() *Mock {
	mgr := new(Mock)
	var pluginsMap = make(map[string]managerContracts.Plugin)
	var cwPlugin = managerContracts.Plugin{
		Handler: cloudwatch.NewMockDefault(),
	}
	pluginsMap[CloudWatchId] = cwPlugin

	mgr.On("GetRegisteredPlugins").Return(pluginsMap)
	mgr.On("Name").Return(CloudWatchId)
	mgr.On("Execute", mock.AnythingOfType("context.T")).Return(nil)
	mgr.On("RequestStop", mock.AnythingOfType("string")).Return(nil)
	mgr.On("StopPlugin", mock.AnythingOfType("string"), mock.Anything).Return(nil)
	mgr.On("StartPlugin", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("task.CancelFlag")).Return(nil)
	return mgr
}

// GetRegisteredPlugins returns a map of all registered long running plugins - return the specified plugin map for testing here
func (m *Mock) GetRegisteredPlugins() map[string]managerContracts.Plugin {
	args := m.Called()
	return args.Get(0).(map[string]managerContracts.Plugin)
}

// Name returns the module name
func (m *Mock) ModuleName() string {
	args := m.Called()
	return args.Get(0).(string)
}

// Execute starts long running plugin manager and returns encountered error - returns nil here for testing
func (m *Mock) ModuleExecute(context context.T) (err error) {
	args := m.Called(context)
	return args.Get(0).(error)
}

// RequestStop handles the termination of the message processor plugin job and returns encountered error - returns nil here for testing
func (m *Mock) ModuleRequestStop(stopType contracts.StopType) (err error) {
	return nil
}

// StopPlugin stops a given plugin from executing and returns encountered error - returns nil here for testing
func (m *Mock) StopPlugin(name string, cancelFlag task.CancelFlag) (err error) {
	return nil
}

// StartPlugin starts the given plugin with the given configuration and returns encountered error - returns nil here for testing
func (m *Mock) StartPlugin(name, configuration, orchestrationDir string, cancelFlag task.CancelFlag, out iohandler.IOHandler) (err error) {
	return nil
}

// EnsurePluginRegistered adds a long-running plugin if it is not already in the registry
func (m *Mock) EnsurePluginRegistered(name string, plugin managerContracts.Plugin) (err error) {
	return nil
}
