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
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/plugin"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/statemanager/model"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestRunPlugins tests that RunPluginsWithRegistry calls all the expected plugins.
func TestRunPluginsWithNewDocument(t *testing.T) {
	pluginNames := []string{"plugin1", "plugin2"}
	pluginConfigs := make(map[string]model.PluginState)
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginInstances := make(map[string]*plugin.Mock)
	pluginRegistry := runpluginutil.PluginRegistry{}
	documentID := "TestDocument"

	var cancelFlag task.CancelFlag = task.NewChanneledCancelFlag()

	ctx := context.NewMockDefault()
	defaultTime := time.Now()
	defaultOutput := "output"
	pluginConfigs2 := make([]model.PluginState, len(pluginNames))

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(plugin.Mock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID: name,
		}

		// setup expectations
		pluginConfigs[name] = model.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}
		pluginResults[name] = &contracts.PluginResult{
			Output:         name,
			PluginName:     name,
			StartDateTime:  defaultTime,
			EndDateTime:    defaultTime,
			StandardOutput: defaultOutput,
			StandardError:  defaultOutput,
		}

		if name == "plugin2" {
			pluginResults[name].Status = contracts.ResultStatusSuccessAndReboot
		}

		pluginInstances[name].On("Execute", ctx, pluginConfigs[name].Configuration, cancelFlag).Return(*pluginResults[name])
		pluginRegistry[name] = pluginInstances[name]

		pluginConfigs2[index] = pluginConfigs[name]
	}
	called := 0
	sendResponse := func(messageID string, pluginID string, results map[string]*contracts.PluginResult) {
		for _, result := range results {
			result.EndDateTime = defaultTime
			result.StartDateTime = defaultTime
		}
		if called == 0 {
			assert.Equal(t, results["plugin1"], pluginResults["plugin1"])
		} else if called == 1 {
			assert.Equal(t, results, pluginResults)
		} else {
			assert.Fail(t, "sendreply shouldn't been called more than twice")
		}
		called++
	}
	// call the code we are testing
	outputs := RunPlugins(ctx, documentID, "", pluginConfigs2, pluginRegistry, sendResponse, nil, cancelFlag)

	// fix the times expectation.
	for _, result := range outputs {
		result.EndDateTime = defaultTime
		result.StartDateTime = defaultTime
	}

	// assert that the expectations were met
	for _, mockPlugin := range pluginInstances {
		mockPlugin.AssertExpectations(t)
	}
	ctx.AssertCalled(t, "Log")
	assert.Equal(t, pluginResults["plugin1"], outputs["plugin1"])
	assert.Equal(t, pluginResults["plugin2"], outputs["plugin2"])

	assert.Equal(t, pluginResults, outputs)

}

//TODO cancelFlag should not fail subsequent plugins
func TestRunPluginsWithCancelFlagShutdown(t *testing.T) {
	pluginNames := []string{"plugin1", "plugin2"}
	pluginStates := make([]model.PluginState, 2)
	pluginResults := make(map[string]*contracts.PluginResult)
	plugins := make(map[string]*plugin.Mock)
	pluginRegistry := runpluginutil.PluginRegistry{}
	documentID := "TestDocument2"

	sendResponse := func(messageID string, pluginID string, results map[string]*contracts.PluginResult) {
	}

	var cancelFlag task.CancelFlag = task.NewChanneledCancelFlag()
	ctx := context.NewMockDefault()
	defaultTime := time.Now()

	for index, name := range pluginNames {
		plugins[name] = new(plugin.Mock)
		config := contracts.Configuration{
			PluginID: name,
		}
		pluginState := model.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}

		pluginResults[name] = &contracts.PluginResult{
			Output:        name,
			PluginName:    name,
			StartDateTime: defaultTime,
			EndDateTime:   defaultTime,
		}
		if name == "plugin1" {
			pluginResults[name].Status = contracts.ResultStatusSuccess
			plugins[name].On("Execute", ctx, pluginState.Configuration, cancelFlag).Run(func(args mock.Arguments) {
				flag := args.Get(2).(task.CancelFlag)
				flag.Set(task.ShutDown)
			}).Return(*pluginResults[name])

		} else {
			pluginResults[name].Status = contracts.ResultStatusFailed
			plugins[name].On("Execute", ctx, pluginState.Configuration, cancelFlag).Return(*pluginResults[name])
		}
		pluginStates[index] = pluginState
		pluginRegistry[name] = plugins[name]
	}

	outputs := RunPlugins(ctx, documentID, "", pluginStates, pluginRegistry, sendResponse, nil, cancelFlag)

	// fix the times expectation.
	for _, result := range outputs {
		result.EndDateTime = defaultTime
		result.StartDateTime = defaultTime
	}
	for _, mockPlugin := range plugins {
		mockPlugin.AssertExpectations(t)
	}
	ctx.AssertCalled(t, "Log")
	assert.Equal(t, pluginResults["plugin1"], outputs["plugin1"])
	//empty struct
	assert.Equal(t, pluginResults["plugin2"], outputs["plugin2"])
}

func TestRunPluginsWithInProgressDocuments(t *testing.T) {
	pluginNames := []string{"plugin1", "plugin2"}
	pluginStates := make([]model.PluginState, 2)
	pluginResults := make(map[string]*contracts.PluginResult)
	plugins := make(map[string]*plugin.Mock)
	pluginRegistry := runpluginutil.PluginRegistry{}
	documentID := "TestDocument2"

	sendResponse := func(messageID string, pluginID string, results map[string]*contracts.PluginResult) {
	}

	var cancelFlag task.CancelFlag = task.NewChanneledCancelFlag()
	ctx := context.NewMockDefault()
	defaultTime := time.Now()

	for index, name := range pluginNames {
		plugins[name] = new(plugin.Mock)
		config := contracts.Configuration{
			PluginID: name,
		}
		pluginState := model.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}

		pluginResults[name] = &contracts.PluginResult{
			Output:        name,
			Status:        contracts.ResultStatusSuccess,
			PluginName:    name,
			StartDateTime: defaultTime,
			EndDateTime:   defaultTime,
		}
		//plugin1 has already been executed, plugin2 has not started yet
		if name == "plugin1" {
			pluginState.Result = *pluginResults[name]
		} else {
			pluginState.Result.Status = contracts.ResultStatusNotStarted
			plugins[name].On("Execute", ctx, pluginState.Configuration, cancelFlag).Return(*pluginResults[name])
		}
		pluginStates[index] = pluginState
		pluginRegistry[name] = plugins[name]
	}

	outputs := RunPlugins(ctx, documentID, "", pluginStates, pluginRegistry, sendResponse, nil, cancelFlag)
	// fix the times expectation.
	for _, result := range outputs {
		result.EndDateTime = defaultTime
		result.StartDateTime = defaultTime
	}
	for _, mockPlugin := range plugins {
		mockPlugin.AssertExpectations(t)
	}
	assert.Equal(t, pluginResults["plugin1"], outputs["plugin1"])
	assert.Equal(t, pluginResults["plugin2"], outputs["plugin2"])

}

//TODO this test wont work cuz we don't have a good way to mock lib functions
//func TestEngineUnhandledPlugins(t *testing.T) {
//	pluginName := "nonexited_plugin"
//	pluginStates := make([]model.PluginState, 1)
//	pluginResults := make(map[string]*contracts.PluginResult)
//	plugins := make(map[string]*plugin.Mock)
//	pluginRegistry := runpluginutil.PluginRegistry{}
//	documentID := "TestDocument3"
//
//	sendResponse := func(messageID string, pluginID string, results map[string]*contracts.PluginResult) {
//	}
//
//	var cancelFlag task.CancelFlag = task.NewChanneledCancelFlag()
//	ctx := context.NewMockDefault()
//
//	plugins[pluginName] = new(plugin.Mock)
//	config := contracts.Configuration{
//		PluginID: pluginName,
//	}
//	pluginState := model.PluginState{
//		Name:          pluginName,
//		Id:            pluginName,
//		Configuration: config,
//	}
//
//	pluginResults[pluginName] = &contracts.PluginResult{
//		Status:     contracts.ResultStatusFailed,
//		PluginName: pluginName,
//	}
//	pluginStates[0] = pluginState
//	outputs := RunPlugins(ctx, documentID, "", pluginStates, pluginRegistry, sendResponse, nil, cancelFlag)
//	plugins[pluginName].AssertExpectations(t)
//	assert.Equal(t, pluginResults[pluginName], outputs[pluginName])
//}

func TestRunPluginsWithDuplicatePluginType(t *testing.T) {
	pluginType := "aws:runShellScript"
	pluginNames := []string{"plugin1", "plugin2"}
	pluginConfigs := make(map[string]model.PluginState)
	pluginResults := make(map[string]*contracts.PluginResult)
	// create an instance of our test object
	plugin := new(plugin.Mock)
	pluginRegistry := runpluginutil.PluginRegistry{}
	documentID := "TestDocument"

	var cancelFlag task.CancelFlag
	ctx := context.NewMockDefault()
	defaultTime := time.Now()
	defaultOutput := "output"
	pluginConfigs2 := make([]model.PluginState, len(pluginNames))

	for index, name := range pluginNames {

		// create configuration for execution
		config := contracts.Configuration{
			PluginID: name,
		}

		// setup expectations
		pluginConfigs[name] = model.PluginState{
			Name:          pluginType,
			Id:            name,
			Configuration: config,
		}

		pluginResults[name] = &contracts.PluginResult{
			Output:         name,
			PluginName:     pluginType,
			StartDateTime:  defaultTime,
			EndDateTime:    defaultTime,
			StandardOutput: defaultOutput,
			StandardError:  defaultOutput,
		}

		if name == "plugin2" {
			pluginResults[name].Status = contracts.ResultStatusSuccessAndReboot
		}

		plugin.On("Execute", ctx, pluginConfigs[name].Configuration, cancelFlag).Return(*pluginResults[name])
		pluginRegistry[pluginType] = plugin

		pluginConfigs2[index] = pluginConfigs[name]
	}
	called := 0
	sendResponse := func(messageID string, pluginID string, results map[string]*contracts.PluginResult) {
		for _, result := range results {
			result.EndDateTime = defaultTime
			result.StartDateTime = defaultTime
		}
		if called == 0 {
			assert.Equal(t, results["plugin1"], pluginResults["plugin1"])
		} else if called == 1 {
			assert.Equal(t, results, pluginResults)
		} else {
			assert.Fail(t, "sendreply shouldn't been called more than twice")
		}
		called++
	}
	// call the code we are testing
	outputs := RunPlugins(ctx, documentID, "", pluginConfigs2, pluginRegistry, sendResponse, nil, cancelFlag)

	// fix the times expectation.
	for _, result := range outputs {
		result.EndDateTime = defaultTime
		result.StartDateTime = defaultTime
	}

	// assert that the expectations were met
	plugin.AssertExpectations(t)

	ctx.AssertCalled(t, "Log")
	assert.Equal(t, pluginResults["plugin1"], outputs["plugin1"])
	assert.Equal(t, pluginResults["plugin2"], outputs["plugin2"])

	assert.Equal(t, pluginType, outputs["plugin1"].PluginName)
	assert.Equal(t, pluginType, outputs["plugin2"].PluginName)

	assert.Equal(t, pluginResults, outputs)
}
