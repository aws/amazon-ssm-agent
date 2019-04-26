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

// Package runpluginutil run plugin utility functions without referencing the actually plugin impl packages
package runpluginutil

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	testPlugin1           = "plugin1"
	testPlugin2           = "plugin2"
	testUnknownPlugin     = "plugin3"
	testUnsupportedPlugin = "plugin4"
)

var origIsSupported func(log log.T, pluginName string) (isKnown bool, isSupported bool, message string)

func setIsSupportedMock() {
	origIsSupported = isSupportedPlugin
	isSupportedPlugin = func(log log.T, pluginName string) (isKnown bool, isSupported bool, message string) {
		switch pluginName {
		case testUnknownPlugin:
			return false, true, ""
		case testUnsupportedPlugin:
			return true, false, ""
		default:
			return true, true, ""
		}
	}
}

func restoreIsSupported() {
	isSupportedPlugin = origIsSupported
}

// TestRunPlugins tests that RunPluginsWithRegistry calls all the expected plugins.
func TestRunPluginsWithNewDocument(t *testing.T) {
	setIsSupportedMock()
	defer restoreIsSupported()
	pluginNames := []string{testPlugin1, testPlugin2}
	pluginConfigs := make(map[string]contracts.PluginState)
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginInstances := make(map[string]*PluginMock)
	pluginRegistry := PluginRegistry{}
	var cancelFlag task.CancelFlag = task.NewChanneledCancelFlag()

	ctx := context.NewMockDefault()
	defaultTime := time.Now()
	pluginConfigs2 := make([]contracts.PluginState, len(pluginNames))
	ioConfig := contracts.IOConfiguration{}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:   name,
			PluginName: name,
		}

		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}
		pluginResults[name] = &contracts.PluginResult{
			PluginID:      name,
			PluginName:    name,
			StartDateTime: defaultTime,
			EndDateTime:   defaultTime,
			Output:        "",
		}

		pluginInstances[name].On("Execute", ctx, pluginConfigs[name].Configuration, cancelFlag, mock.Anything).Return()

		pluginFactory := new(PluginFactoryMock)
		pluginFactory.On("Create", mock.Anything).Return(pluginInstances[name], nil)
		pluginRegistry[name] = pluginFactory

		pluginConfigs2[index] = pluginConfigs[name]
	}
	called := 0

	ch := make(chan contracts.PluginResult)
	go func() {
		for result := range ch {
			result.EndDateTime = defaultTime
			result.StartDateTime = defaultTime
			if called == 0 {
				assert.Equal(t, result, *pluginResults[testPlugin1])
			} else if called == 1 {
				assert.Equal(t, result, *pluginResults[testPlugin2])
			} else {
				assert.Fail(t, "there shouldn't be more than 2 update")
			}
			called++
		}
	}()
	// call the code we are testing
	outputs := RunPlugins(ctx, pluginConfigs2, ioConfig, pluginRegistry, ch, cancelFlag)
	close(ch)

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
	assert.Equal(t, pluginResults[testPlugin1], outputs[testPlugin1])
	assert.Equal(t, pluginResults[testPlugin2], outputs[testPlugin2])

	assert.Equal(t, pluginResults, outputs)

}

// Document with steps containing unknown plugin (i.e. when plugin handler is not found), steps must fail
func TestRunPluginsWithMissingPluginHandler(t *testing.T) {
	setIsSupportedMock()
	defer restoreIsSupported()
	pluginNames := []string{testPlugin1, testPlugin2}
	pluginConfigs := make(map[string]contracts.PluginState)
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginInstances := make(map[string]*PluginMock)
	pluginRegistry := PluginRegistry{}

	var cancelFlag task.CancelFlag = task.NewChanneledCancelFlag()

	ctx := context.NewMockDefault()
	defaultTime := time.Now()
	defaultOutput := ""
	pluginConfigs2 := make([]contracts.PluginState, len(pluginNames))
	ioConfig := contracts.IOConfiguration{}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:   name,
			PluginName: name,
		}

		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}

		pluginError := fmt.Sprintf("Plugin with name %s not found. Step name: %s", name, name)

		pluginResults[name] = &contracts.PluginResult{
			PluginName:     name,
			PluginID:       name,
			StartDateTime:  defaultTime,
			EndDateTime:    defaultTime,
			StandardOutput: defaultOutput,
			StandardError:  defaultOutput,
			Status:         contracts.ResultStatusFailed,
			Error:          pluginError,
		}

		pluginConfigs2[index] = pluginConfigs[name]
	}
	called := 0

	ch := make(chan contracts.PluginResult)
	go func() {
		for result := range ch {
			result.EndDateTime = defaultTime
			result.StartDateTime = defaultTime
			if called == 0 {
				assert.Equal(t, result, *pluginResults[testPlugin1])
			} else if called == 1 {
				assert.Equal(t, result, *pluginResults[testPlugin2])
			} else {
				assert.Fail(t, "there shouldn't be more than 2 update")
			}
			called++
		}
	}()
	// call the code we are testing
	outputs := RunPlugins(ctx, pluginConfigs2, ioConfig, pluginRegistry, ch, cancelFlag)

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
	assert.Equal(t, pluginResults[testPlugin1], outputs[testPlugin1])
	assert.Equal(t, pluginResults[testPlugin2], outputs[testPlugin2])

	assert.Equal(t, pluginResults, outputs)

}

//TODO cancelFlag should not fail subsequent plugins
func TestRunPluginsWithCancelFlagShutdown(t *testing.T) {
	setIsSupportedMock()
	defer restoreIsSupported()
	pluginNames := []string{testPlugin1, testPlugin2}
	pluginStates := make([]contracts.PluginState, 2)
	pluginResults := make(map[string]*contracts.PluginResult)
	plugins := make(map[string]*PluginMock)
	pluginRegistry := PluginRegistry{}

	var cancelFlag task.CancelFlag = task.NewChanneledCancelFlag()
	ctx := context.NewMockDefault()
	defaultTime := time.Now()
	ioConfig := contracts.IOConfiguration{}

	for index, name := range pluginNames {
		plugins[name] = new(PluginMock)
		config := contracts.Configuration{
			PluginID:   name,
			PluginName: name,
		}
		pluginState := contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}

		pluginResults[name] = &contracts.PluginResult{
			Output:        "",
			PluginName:    name,
			PluginID:      name,
			StartDateTime: defaultTime,
			EndDateTime:   defaultTime,
		}
		if name == testPlugin1 {
			plugins[name].On("Execute", ctx, pluginState.Configuration, cancelFlag, mock.Anything).Run(func(args mock.Arguments) {
				flag := args.Get(2).(task.CancelFlag)
				flag.Set(task.ShutDown)
			}).Return()

		} else {
			plugins[name].On("Execute", ctx, pluginState.Configuration, cancelFlag, mock.Anything).Return()
		}
		pluginStates[index] = pluginState
		pluginFactory := new(PluginFactoryMock)
		pluginFactory.On("Create", mock.Anything).Return(plugins[name], nil)
		pluginRegistry[name] = pluginFactory
	}

	ch := make(chan contracts.PluginResult, 2)

	outputs := RunPlugins(ctx, pluginStates, ioConfig, pluginRegistry, ch, cancelFlag)

	close(ch)

	// fix the times expectation.
	for _, result := range outputs {
		result.EndDateTime = defaultTime
		result.StartDateTime = defaultTime
	}
	for _, mockPlugin := range plugins {
		mockPlugin.AssertExpectations(t)
	}
	ctx.AssertCalled(t, "Log")
	assert.Equal(t, pluginResults[testPlugin1], outputs[testPlugin1])
	//empty struct
	assert.Equal(t, pluginResults[testPlugin2], outputs[testPlugin2])
}

func TestRunPluginsWithInProgressDocuments(t *testing.T) {
	setIsSupportedMock()
	defer restoreIsSupported()
	pluginNames := []string{testPlugin1, testPlugin2}
	pluginStates := make([]contracts.PluginState, 2)
	pluginResults := make(map[string]*contracts.PluginResult)
	plugins := make(map[string]*PluginMock)
	pluginRegistry := PluginRegistry{}
	ioConfig := contracts.IOConfiguration{}

	var cancelFlag task.CancelFlag = task.NewChanneledCancelFlag()
	ctx := context.NewMockDefault()
	defaultTime := time.Now()

	for index, name := range pluginNames {
		plugins[name] = new(PluginMock)
		config := contracts.Configuration{
			PluginID:   name,
			PluginName: name,
		}
		pluginState := contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}

		pluginResults[name] = &contracts.PluginResult{
			Output:        "",
			Status:        contracts.ResultStatusSuccess,
			PluginName:    name,
			PluginID:      name,
			StartDateTime: defaultTime,
			EndDateTime:   defaultTime,
		}
		//plugin1 has already been executed, plugin2 has not started yet
		if name == testPlugin1 {
			pluginState.Result = *pluginResults[name]
		} else {
			pluginState.Result.Status = contracts.ResultStatusNotStarted
			plugins[name].On("Execute", ctx, pluginState.Configuration, cancelFlag, mock.Anything).Return()
		}
		pluginStates[index] = pluginState
		pluginFactory := new(PluginFactoryMock)
		pluginFactory.On("Create", mock.Anything).Return(plugins[name], nil)
		pluginRegistry[name] = pluginFactory
	}

	ch := make(chan contracts.PluginResult, 2)
	outputs := RunPlugins(ctx, pluginStates, ioConfig, pluginRegistry, ch, cancelFlag)
	close(ch)
	// fix the times expectation.
	for _, result := range outputs {
		result.EndDateTime = defaultTime
		result.StartDateTime = defaultTime
	}
	for _, mockPlugin := range plugins {
		mockPlugin.AssertExpectations(t)
	}
	pluginResults[testPlugin2].Status = ""
	assert.Equal(t, pluginResults[testPlugin1], outputs[testPlugin1])
	assert.Equal(t, pluginResults[testPlugin2], outputs[testPlugin2])
}

//TODO this test wont work cuz we don't have a good way to mock lib functions
//func TestEngineUnhandledPlugins(t *testing.T) {
//	pluginName := "nonexited_plugin"
//	pluginStates := make([]contracts.PluginState, 1)
//	pluginResults := make(map[string]*contracts.PluginResult)
//	plugins := make(map[string]*PluginMock)
//	pluginRegistry := PluginRegistry{}
//
//	var cancelFlag task.CancelFlag = task.NewChanneledCancelFlag()
//	ctx := context.NewMockDefault()
//	defaultTime := time.Now()
//
//	for index, name := range pluginNames {
//		plugins[name] = new(PluginMock)
//		config := contracts.Configuration{
//			PluginID: name,
//		}
//		pluginState := contracts.PluginState{
//			Name:          name,
//			Id:            name,
//			Configuration: config,
//		}
//
//	pluginResults[pluginName] = &contracts.PluginResult{
//		Status:     contracts.ResultStatusFailed,
//		PluginName: pluginName,
//	}
//	pluginStates[0] = pluginState
//	outputs := (ctx, pluginStates, pluginRegistry, sendResponse, nil, cancelFlag)
//	plugins[pluginName].AssertExpectations(t)
//	assert.Equal(t, pluginResults[pluginName], outputs[pluginName])
//}

func TestRunPluginsWithDuplicatePluginType(t *testing.T) {
	setIsSupportedMock()
	defer restoreIsSupported()
	pluginType := "aws:runShellScript"
	pluginNames := []string{testPlugin1, testPlugin2}
	pluginConfigs := make(map[string]contracts.PluginState)
	pluginResults := make(map[string]*contracts.PluginResult)
	// create an instance of our test object
	plugin := new(PluginMock)
	pluginRegistry := PluginRegistry{}
	ioConfig := contracts.IOConfiguration{}

	var cancelFlag task.CancelFlag
	ctx := context.NewMockDefault()
	defaultTime := time.Now()
	pluginConfigs2 := make([]contracts.PluginState, len(pluginNames))

	for index, name := range pluginNames {

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:   name,
			PluginName: name,
		}

		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          pluginType,
			Id:            name,
			Configuration: config,
		}

		pluginResults[name] = &contracts.PluginResult{
			Output:        "",
			PluginID:      name,
			PluginName:    pluginType,
			StartDateTime: defaultTime,
			EndDateTime:   defaultTime,
		}

		pluginFactory := new(PluginFactoryMock)
		pluginFactory.On("Create", mock.Anything).Return(plugin, nil)
		plugin.On("Execute", ctx, pluginConfigs[name].Configuration, cancelFlag, mock.Anything).Return(*pluginResults[name])
		pluginRegistry[pluginType] = pluginFactory

		pluginConfigs2[index] = pluginConfigs[name]
	}
	called := 0
	ch := make(chan contracts.PluginResult)
	go func() {
		for result := range ch {
			result.EndDateTime = defaultTime
			result.StartDateTime = defaultTime
			if called == 0 {
				assert.Equal(t, result, *pluginResults[testPlugin1])
			} else if called == 1 {
				assert.Equal(t, result, *pluginResults[testPlugin2])
			} else {
				assert.Fail(t, "there shouldn't be more than 2 update")
			}
			called++
		}
	}()
	// call the code we are testing
	outputs := RunPlugins(ctx, pluginConfigs2, ioConfig, pluginRegistry, ch, cancelFlag)

	// fix the times expectation.
	for _, result := range outputs {
		result.EndDateTime = defaultTime
		result.StartDateTime = defaultTime
	}

	// assert that the expectations were met
	plugin.AssertExpectations(t)

	ctx.AssertCalled(t, "Log")
	assert.Equal(t, pluginResults[testPlugin1], outputs[testPlugin1])
	assert.Equal(t, pluginResults[testPlugin2], outputs[testPlugin2])

	assert.Equal(t, pluginType, outputs[testPlugin1].PluginName)
	assert.Equal(t, pluginType, outputs[testPlugin2].PluginName)

	assert.Equal(t, pluginResults, outputs)
}

// Crossplatform document with compatible precondition, steps must be executed
// Precondition = "StringEquals": ["platformType", "Linux"]
func TestRunPluginsWithCompatiblePrecondition(t *testing.T) {
	setIsSupportedMock()
	defer restoreIsSupported()
	pluginNames := []string{testPlugin1, testPlugin2}
	pluginConfigs := make(map[string]contracts.PluginState)
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginInstances := make(map[string]*PluginMock)
	pluginRegistry := PluginRegistry{}
	ioConfig := contracts.IOConfiguration{}

	var cancelFlag task.CancelFlag = task.NewChanneledCancelFlag()

	ctx := context.NewMockDefault()
	defaultTime := time.Now()
	pluginConfigs2 := make([]contracts.PluginState, len(pluginNames))

	preconditions := map[string][]string{"StringEquals": []string{"platformType", "Linux"}}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         preconditions,
		}

		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}
		pluginResults[name] = &contracts.PluginResult{
			Output:        "",
			PluginName:    name,
			PluginID:      name,
			StartDateTime: defaultTime,
			EndDateTime:   defaultTime,
		}

		pluginFactory := new(PluginFactoryMock)
		pluginFactory.On("Create", mock.Anything).Return(pluginInstances[name], nil)
		pluginRegistry[name] = pluginFactory
		pluginInstances[name].On("Execute", ctx, pluginConfigs[name].Configuration, cancelFlag, mock.Anything).Return()

		pluginConfigs2[index] = pluginConfigs[name]
	}
	called := 0
	ch := make(chan contracts.PluginResult)
	go func() {
		for result := range ch {
			result.EndDateTime = defaultTime
			result.StartDateTime = defaultTime
			if called == 0 {
				assert.Equal(t, result, *pluginResults[testPlugin1])
			} else if called == 1 {
				assert.Equal(t, result, *pluginResults[testPlugin2])
			} else {
				assert.Fail(t, "there shouldn't be more than 2 update")
			}
			called++
		}
	}()
	// call the code we are testing
	outputs := RunPlugins(ctx, pluginConfigs2, ioConfig, pluginRegistry, ch, cancelFlag)

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
	assert.Equal(t, pluginResults[testPlugin1], outputs[testPlugin1])
	assert.Equal(t, pluginResults[testPlugin2], outputs[testPlugin2])

	assert.Equal(t, pluginResults, outputs)
}

// Crossplatform document with compatible precondition, steps must be executed
// Precondition = "StringEquals": ["Linux", "platformType"]
func TestRunPluginsWithCompatiblePreconditionWithValueFirst(t *testing.T) {
	setIsSupportedMock()
	defer restoreIsSupported()
	pluginNames := []string{testPlugin1, testPlugin2}
	pluginConfigs := make(map[string]contracts.PluginState)
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginInstances := make(map[string]*PluginMock)
	pluginRegistry := PluginRegistry{}
	ioConfig := contracts.IOConfiguration{}

	var cancelFlag task.CancelFlag = task.NewChanneledCancelFlag()

	ctx := context.NewMockDefault()
	defaultTime := time.Now()
	pluginConfigs2 := make([]contracts.PluginState, len(pluginNames))

	preconditions := map[string][]string{"StringEquals": []string{"Linux", "platformType"}}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         preconditions,
		}

		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}
		pluginResults[name] = &contracts.PluginResult{
			Output:        "",
			PluginID:      name,
			PluginName:    name,
			StartDateTime: defaultTime,
			EndDateTime:   defaultTime,
		}

		pluginFactory := new(PluginFactoryMock)
		pluginFactory.On("Create", mock.Anything).Return(pluginInstances[name], nil)
		pluginRegistry[name] = pluginFactory
		pluginInstances[name].On("Execute", ctx, pluginConfigs[name].Configuration, cancelFlag, mock.Anything).Return()

		pluginConfigs2[index] = pluginConfigs[name]
	}
	called := 0
	ch := make(chan contracts.PluginResult)
	go func() {
		for result := range ch {
			result.EndDateTime = defaultTime
			result.StartDateTime = defaultTime
			if called == 0 {
				assert.Equal(t, result, *pluginResults[testPlugin1])
			} else if called == 1 {
				assert.Equal(t, result, *pluginResults[testPlugin2])
			} else {
				assert.Fail(t, "there shouldn't be more than 2 update")
			}
			called++
		}
	}()
	// call the code we are testing
	outputs := RunPlugins(ctx, pluginConfigs2, ioConfig, pluginRegistry, ch, cancelFlag)
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
	assert.Equal(t, pluginResults[testPlugin1], outputs[testPlugin1])
	assert.Equal(t, pluginResults[testPlugin2], outputs[testPlugin2])

	assert.Equal(t, pluginResults, outputs)
}

// Crossplatform document with incompatible precondition, steps must be skipped
func TestRunPluginsWithIncompatiblePrecondition(t *testing.T) {
	setIsSupportedMock()
	defer restoreIsSupported()
	pluginNames := []string{testPlugin1, testPlugin2}
	pluginConfigs := make(map[string]contracts.PluginState)
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginInstances := make(map[string]*PluginMock)
	pluginRegistry := PluginRegistry{}
	ioConfig := contracts.IOConfiguration{}

	var cancelFlag task.CancelFlag = task.NewChanneledCancelFlag()

	ctx := context.NewMockDefault()
	defaultTime := time.Now()
	defaultOutput := ""
	pluginConfigs2 := make([]contracts.PluginState, len(pluginNames))

	preconditions := map[string][]string{"StringEquals": []string{"platformType", "Windows"}}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         preconditions,
		}

		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}
		pluginResults[name] = &contracts.PluginResult{
			Output:         "Step execution skipped due to incompatible platform. Step name: " + name,
			PluginName:     name,
			PluginID:       name,
			StartDateTime:  defaultTime,
			EndDateTime:    defaultTime,
			StandardOutput: defaultOutput,
			StandardError:  defaultOutput,
			Status:         contracts.ResultStatusSkipped,
		}
		pluginFactory := new(PluginFactoryMock)
		pluginFactory.On("Create", mock.Anything).Return(pluginInstances[name], nil)
		pluginRegistry[name] = pluginFactory
		pluginConfigs2[index] = pluginConfigs[name]
	}
	called := 0
	ch := make(chan contracts.PluginResult)
	go func() {
		for result := range ch {
			result.EndDateTime = defaultTime
			result.StartDateTime = defaultTime
			if called == 0 {
				assert.Equal(t, result, *pluginResults[testPlugin1])
			} else if called == 1 {
				assert.Equal(t, result, *pluginResults[testPlugin2])
			} else {
				assert.Fail(t, "there shouldn't be more than 2 update")
			}
			called++
		}
	}()
	// call the code we are testing
	outputs := RunPlugins(ctx, pluginConfigs2, ioConfig, pluginRegistry, ch, cancelFlag)
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
	assert.Equal(t, pluginResults[testPlugin1], outputs[testPlugin1])
	assert.Equal(t, pluginResults[testPlugin2], outputs[testPlugin2])

	assert.Equal(t, pluginResults, outputs)
}

// Crossplatform document with unknown plugin (i.e. when plugin handler is not found), steps must be skipped
func TestRunPluginsWithCompatiblePreconditionButMissingPluginHandler(t *testing.T) {
	setIsSupportedMock()
	defer restoreIsSupported()
	pluginNames := []string{testPlugin1, testPlugin2}
	pluginConfigs := make(map[string]contracts.PluginState)
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginInstances := make(map[string]*PluginMock)
	pluginRegistry := PluginRegistry{}
	ioConfig := contracts.IOConfiguration{}

	var cancelFlag task.CancelFlag = task.NewChanneledCancelFlag()

	ctx := context.NewMockDefault()
	defaultTime := time.Now()
	defaultOutput := ""
	pluginConfigs2 := make([]contracts.PluginState, len(pluginNames))

	preconditions := map[string][]string{"StringEquals": []string{"platformType", "Linux"}}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         preconditions,
		}

		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}
		pluginResults[name] = &contracts.PluginResult{
			Output:         "Step execution skipped due to incompatible platform. Step name: " + name,
			PluginName:     name,
			PluginID:       name,
			StartDateTime:  defaultTime,
			EndDateTime:    defaultTime,
			StandardOutput: defaultOutput,
			StandardError:  defaultOutput,
			Status:         contracts.ResultStatusSkipped,
		}

		pluginConfigs2[index] = pluginConfigs[name]
	}
	called := 0
	ch := make(chan contracts.PluginResult)
	go func() {
		for result := range ch {
			result.EndDateTime = defaultTime
			result.StartDateTime = defaultTime
			if called == 0 {
				assert.Equal(t, result, *pluginResults[testPlugin1])
			} else if called == 1 {
				assert.Equal(t, result, *pluginResults[testPlugin2])
			} else {
				assert.Fail(t, "there shouldn't be more than 2 update")
			}
			called++
		}
	}()
	// call the code we are testing
	outputs := RunPlugins(ctx, pluginConfigs2, ioConfig, pluginRegistry, ch, cancelFlag)
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
	assert.Equal(t, pluginResults[testPlugin1], outputs[testPlugin1])
	assert.Equal(t, pluginResults[testPlugin2], outputs[testPlugin2])

	assert.Equal(t, pluginResults, outputs)
}

// Crossplatform document with more than 1 precondition, steps must fail
func TestRunPluginsWithMoreThanOnePrecondition(t *testing.T) {
	setIsSupportedMock()
	defer restoreIsSupported()
	pluginNames := []string{testPlugin1, testPlugin2}
	pluginConfigs := make(map[string]contracts.PluginState)
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginInstances := make(map[string]*PluginMock)
	pluginRegistry := PluginRegistry{}
	ioConfig := contracts.IOConfiguration{}

	var cancelFlag task.CancelFlag = task.NewChanneledCancelFlag()

	ctx := context.NewMockDefault()
	defaultTime := time.Now()
	defaultOutput := ""
	pluginConfigs2 := make([]contracts.PluginState, len(pluginNames))

	preconditions := map[string][]string{
		"StringEquals": []string{"platformType", "Linux"},
		"foo":          []string{"operand1", "operand2"},
	}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         preconditions,
		}

		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}

		pluginError := fmt.Sprintf(
			"Unrecognized precondition(s): '\"foo\": [operand1 operand2]', please update agent to latest version. Step name: %s",
			name)

		pluginResults[name] = &contracts.PluginResult{
			PluginName:     name,
			PluginID:       name,
			StartDateTime:  defaultTime,
			EndDateTime:    defaultTime,
			StandardOutput: defaultOutput,
			StandardError:  defaultOutput,
			Status:         contracts.ResultStatusFailed,
			Error:          pluginError,
		}
		pluginFactory := new(PluginFactoryMock)
		pluginFactory.On("Create", mock.Anything).Return(pluginInstances[name], nil)
		pluginRegistry[name] = pluginFactory

		pluginConfigs2[index] = pluginConfigs[name]
	}
	called := 0
	ch := make(chan contracts.PluginResult)
	go func() {
		for result := range ch {
			result.EndDateTime = defaultTime
			result.StartDateTime = defaultTime
			if called == 0 {
				assert.Equal(t, result, *pluginResults[testPlugin1])
			} else if called == 1 {
				assert.Equal(t, result, *pluginResults[testPlugin2])
			} else {
				assert.Fail(t, "there shouldn't be more than 2 update")
			}
			called++
		}
	}()
	// call the code we are testing
	outputs := RunPlugins(ctx, pluginConfigs2, ioConfig, pluginRegistry, ch, cancelFlag)
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
	assert.Equal(t, pluginResults[testPlugin1], outputs[testPlugin1])
	assert.Equal(t, pluginResults[testPlugin2], outputs[testPlugin2])

	assert.Equal(t, pluginResults, outputs)
}

// Crossplatform document with unrecognized precondition operator, steps must fail
func TestRunPluginsWithUnrecognizedPreconditionOperator(t *testing.T) {
	setIsSupportedMock()
	defer restoreIsSupported()
	pluginNames := []string{testPlugin1, testPlugin2}
	pluginConfigs := make(map[string]contracts.PluginState)
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginInstances := make(map[string]*PluginMock)
	pluginRegistry := PluginRegistry{}
	ioConfig := contracts.IOConfiguration{}

	var cancelFlag task.CancelFlag = task.NewChanneledCancelFlag()

	ctx := context.NewMockDefault()
	defaultTime := time.Now()
	defaultOutput := ""
	pluginConfigs2 := make([]contracts.PluginState, len(pluginNames))

	preconditions := map[string][]string{"foo": []string{"platformType", "Linux"}}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         preconditions,
		}

		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}

		pluginError := fmt.Sprintf(
			"Unrecognized precondition(s): '\"foo\": [platformType Linux]', please update agent to latest version. Step name: %s",
			name)

		pluginResults[name] = &contracts.PluginResult{
			PluginName:     name,
			PluginID:       name,
			StartDateTime:  defaultTime,
			EndDateTime:    defaultTime,
			StandardOutput: defaultOutput,
			StandardError:  defaultOutput,
			Status:         contracts.ResultStatusFailed,
			Error:          pluginError,
		}
		pluginFactory := new(PluginFactoryMock)
		pluginFactory.On("Create", mock.Anything).Return(pluginInstances[name], nil)
		pluginRegistry[name] = pluginFactory

		pluginConfigs2[index] = pluginConfigs[name]
	}
	called := 0
	ch := make(chan contracts.PluginResult)
	go func() {
		for result := range ch {
			result.EndDateTime = defaultTime
			result.StartDateTime = defaultTime
			if called == 0 {
				assert.Equal(t, result, *pluginResults[testPlugin1])
			} else if called == 1 {
				assert.Equal(t, result, *pluginResults[testPlugin2])
			} else {
				assert.Fail(t, "there shouldn't be more than 2 update")
			}
			called++
		}
	}()
	// call the code we are testing
	outputs := RunPlugins(ctx, pluginConfigs2, ioConfig, pluginRegistry, ch, cancelFlag)

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
	assert.Equal(t, pluginResults[testPlugin1], outputs[testPlugin1])
	assert.Equal(t, pluginResults[testPlugin2], outputs[testPlugin2])

	assert.Equal(t, pluginResults, outputs)
}

// Crossplatform document with unrecognized precondition operand, steps must fail
func TestRunPluginsWithUnrecognizedPreconditionOperand(t *testing.T) {
	setIsSupportedMock()
	defer restoreIsSupported()
	pluginNames := []string{testPlugin1, testPlugin2}
	pluginConfigs := make(map[string]contracts.PluginState)
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginInstances := make(map[string]*PluginMock)
	pluginRegistry := PluginRegistry{}
	ioConfig := contracts.IOConfiguration{}

	var cancelFlag task.CancelFlag = task.NewChanneledCancelFlag()

	ctx := context.NewMockDefault()
	defaultTime := time.Now()
	defaultOutput := ""
	pluginConfigs2 := make([]contracts.PluginState, len(pluginNames))

	preconditions := map[string][]string{"StringEquals": []string{"foo", "Linux"}}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         preconditions,
		}

		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}

		pluginError := fmt.Sprintf(
			"Unrecognized precondition(s): '\"StringEquals\": [foo Linux]', please update agent to latest version. Step name: %s",
			name)

		pluginResults[name] = &contracts.PluginResult{
			PluginName:     name,
			PluginID:       name,
			StartDateTime:  defaultTime,
			EndDateTime:    defaultTime,
			StandardOutput: defaultOutput,
			StandardError:  defaultOutput,
			Status:         contracts.ResultStatusFailed,
			Error:          pluginError,
		}

		pluginFactory := new(PluginFactoryMock)
		pluginFactory.On("Create", mock.Anything).Return(pluginInstances[name], nil)
		pluginRegistry[name] = pluginFactory
		pluginConfigs2[index] = pluginConfigs[name]
	}
	called := 0
	ch := make(chan contracts.PluginResult)
	go func() {
		for result := range ch {
			result.EndDateTime = defaultTime
			result.StartDateTime = defaultTime
			if called == 0 {
				assert.Equal(t, result, *pluginResults[testPlugin1])
			} else if called == 1 {
				assert.Equal(t, result, *pluginResults[testPlugin2])
			} else {
				assert.Fail(t, "there shouldn't be more than 2 update")
			}
			called++
		}
	}()
	// call the code we are testing
	outputs := RunPlugins(ctx, pluginConfigs2, ioConfig, pluginRegistry, ch, cancelFlag)

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
	assert.Equal(t, pluginResults[testPlugin1], outputs[testPlugin1])
	assert.Equal(t, pluginResults[testPlugin2], outputs[testPlugin2])

	assert.Equal(t, pluginResults, outputs)
}

// Crossplatform document with invalid precondition, steps must fail
// Precondition: "StringEquals": ["platformType", "platformType"]
func TestRunPluginsWithUnrecognizedPreconditionDuplicateVariable(t *testing.T) {
	setIsSupportedMock()
	defer restoreIsSupported()
	pluginNames := []string{testPlugin1, testPlugin2}
	pluginConfigs := make(map[string]contracts.PluginState)
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginInstances := make(map[string]*PluginMock)
	pluginRegistry := PluginRegistry{}
	ioConfig := contracts.IOConfiguration{}

	var cancelFlag task.CancelFlag = task.NewChanneledCancelFlag()

	ctx := context.NewMockDefault()
	defaultTime := time.Now()
	defaultOutput := ""
	pluginConfigs2 := make([]contracts.PluginState, len(pluginNames))

	preconditions := map[string][]string{"StringEquals": []string{"platformType", "platformType"}}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         preconditions,
		}

		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}

		pluginError := fmt.Sprintf(
			"Unrecognized precondition(s): '\"StringEquals\": [platformType platformType]', please update agent to latest version. Step name: %s",
			name)

		pluginResults[name] = &contracts.PluginResult{
			PluginName:     name,
			PluginID:       name,
			StartDateTime:  defaultTime,
			EndDateTime:    defaultTime,
			StandardOutput: defaultOutput,
			StandardError:  defaultOutput,
			Status:         contracts.ResultStatusFailed,
			Error:          pluginError,
		}

		pluginFactory := new(PluginFactoryMock)
		pluginFactory.On("Create", mock.Anything).Return(pluginInstances[name], nil)
		pluginRegistry[name] = pluginFactory

		pluginConfigs2[index] = pluginConfigs[name]
	}
	called := 0
	ch := make(chan contracts.PluginResult)
	go func() {
		for result := range ch {
			result.EndDateTime = defaultTime
			result.StartDateTime = defaultTime
			if called == 0 {
				assert.Equal(t, result, *pluginResults[testPlugin1])
			} else if called == 1 {
				assert.Equal(t, result, *pluginResults[testPlugin2])
			} else {
				assert.Fail(t, "there shouldn't be more than 2 update")
			}
			called++
		}
	}()
	// call the code we are testing
	outputs := RunPlugins(ctx, pluginConfigs2, ioConfig, pluginRegistry, ch, cancelFlag)

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
	assert.Equal(t, pluginResults[testPlugin1], outputs[testPlugin1])
	assert.Equal(t, pluginResults[testPlugin2], outputs[testPlugin2])

	assert.Equal(t, pluginResults, outputs)
}

// Crossplatform document with more than 2 precondition operands, steps must fail
func TestRunPluginsWithMoreThanTwoPreconditionOperands(t *testing.T) {
	setIsSupportedMock()
	defer restoreIsSupported()
	pluginNames := []string{testPlugin1, testPlugin2}
	pluginConfigs := make(map[string]contracts.PluginState)
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginInstances := make(map[string]*PluginMock)
	pluginRegistry := PluginRegistry{}
	ioConfig := contracts.IOConfiguration{}

	var cancelFlag task.CancelFlag = task.NewChanneledCancelFlag()

	ctx := context.NewMockDefault()
	defaultTime := time.Now()
	defaultOutput := ""
	pluginConfigs2 := make([]contracts.PluginState, len(pluginNames))

	preconditions := map[string][]string{"StringEquals": []string{"platformType", "Linux", "foo"}}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         preconditions,
		}

		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}

		pluginError := fmt.Sprintf(
			"Unrecognized precondition(s): '\"StringEquals\": [platformType Linux foo]', please update agent to latest version. Step name: %s",
			name)

		pluginResults[name] = &contracts.PluginResult{
			PluginName:     name,
			PluginID:       name,
			StartDateTime:  defaultTime,
			EndDateTime:    defaultTime,
			StandardOutput: defaultOutput,
			StandardError:  defaultOutput,
			Status:         contracts.ResultStatusFailed,
			Error:          pluginError,
		}

		pluginFactory := new(PluginFactoryMock)
		pluginFactory.On("Create", mock.Anything).Return(pluginInstances[name], nil)
		pluginRegistry[name] = pluginFactory

		pluginConfigs2[index] = pluginConfigs[name]
	}
	called := 0
	ch := make(chan contracts.PluginResult)
	go func() {
		for result := range ch {
			result.EndDateTime = defaultTime
			result.StartDateTime = defaultTime
			if called == 0 {
				assert.Equal(t, result, *pluginResults[testPlugin1])
			} else if called == 1 {
				assert.Equal(t, result, *pluginResults[testPlugin2])
			} else {
				assert.Fail(t, "there shouldn't be more than 2 update")
			}
			called++
		}
	}()
	// call the code we are testing
	outputs := RunPlugins(ctx, pluginConfigs2, ioConfig, pluginRegistry, ch, cancelFlag)

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
	assert.Equal(t, pluginResults[testPlugin1], outputs[testPlugin1])
	assert.Equal(t, pluginResults[testPlugin2], outputs[testPlugin2])

	assert.Equal(t, pluginResults, outputs)
}

// Crossplatform document with unknown plugin, steps must fail
func TestRunPluginsWithUnknownPlugin(t *testing.T) {
	setIsSupportedMock()
	defer restoreIsSupported()
	pluginNames := []string{testPlugin1, testUnknownPlugin, testPlugin2}
	pluginConfigs := make(map[string]contracts.PluginState)
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginInstances := make(map[string]*PluginMock)
	pluginRegistry := PluginRegistry{}
	ioConfig := contracts.IOConfiguration{}

	var cancelFlag task.CancelFlag = task.NewChanneledCancelFlag()

	ctx := context.NewMockDefault()
	defaultTime := time.Now()
	defaultOutput := ""
	pluginConfigs2 := make([]contracts.PluginState, len(pluginNames))

	preconditions := map[string][]string{"StringEquals": []string{"platformType", "Linux"}}

	for index, name := range pluginNames {

		// create an instance of our test object, but not if it is an unknown plugin
		if name != testUnknownPlugin {
			pluginInstances[name] = new(PluginMock)
		}

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         preconditions,
		}

		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}

		if name == testUnknownPlugin {
			pluginError := fmt.Sprintf(
				"Plugin with name %s is not supported by this version of ssm agent, please update to latest version. Step name: %s",
				name,
				name)

			pluginResults[name] = &contracts.PluginResult{
				PluginName:     name,
				PluginID:       name,
				StartDateTime:  defaultTime,
				EndDateTime:    defaultTime,
				StandardOutput: defaultOutput,
				StandardError:  defaultOutput,
				Status:         contracts.ResultStatusFailed,
				Error:          pluginError,
			}
		} else {
			pluginResults[name] = &contracts.PluginResult{
				Output:        "",
				PluginID:      name,
				PluginName:    name,
				StartDateTime: defaultTime,
				EndDateTime:   defaultTime,
			}
			pluginInstances[name].On("Execute", ctx, pluginConfigs[name].Configuration, cancelFlag, mock.Anything).Return(*pluginResults[name])
		}
		pluginFactory := new(PluginFactoryMock)
		pluginFactory.On("Create", mock.Anything).Return(pluginInstances[name], nil)
		pluginRegistry[name] = pluginFactory

		pluginConfigs2[index] = pluginConfigs[name]
	}
	called := 0
	ch := make(chan contracts.PluginResult)
	go func() {
		for result := range ch {
			result.EndDateTime = defaultTime
			result.StartDateTime = defaultTime
			if called > 2 {
				assert.Fail(t, "there shouldn't be more than 3 update")
			}
			called++
		}
	}()
	// call the code we are testing
	outputs := RunPlugins(ctx, pluginConfigs2, ioConfig, pluginRegistry, ch, cancelFlag)

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
	assert.Equal(t, pluginResults[testPlugin1], outputs[testPlugin1])
	assert.Equal(t, pluginResults[testUnknownPlugin], outputs[testUnknownPlugin])
	assert.Equal(t, pluginResults[testPlugin2], outputs[testPlugin2])

	assert.Equal(t, pluginResults, outputs)
}

func TestRunPluginSuccessWithNonTruncatedResult(t *testing.T) {
	setIsSupportedMock()
	defer restoreIsSupported()
	pluginNames := []string{testPlugin1, testPlugin2}
	pluginConfigs := make(map[string]contracts.PluginState)
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginInstances := make(map[string]*PluginMock)
	pluginRegistry := PluginRegistry{}
	var cancelFlag task.CancelFlag = task.NewChanneledCancelFlag()

	ctx := context.NewMockDefault()
	defaultTime := time.Now()
	pluginConfigs2 := make([]contracts.PluginState, len(pluginNames))
	ioConfig := contracts.IOConfiguration{}

	for index, name := range pluginNames {

		// create mock plugin instance for testing
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:   name,
			PluginName: name,
		}

		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}
		pluginResults[name] = &contracts.PluginResult{
			PluginID:       name,
			PluginName:     name,
			StartDateTime:  defaultTime,
			EndDateTime:    defaultTime,
			StandardOutput: "",
		}

		pluginInstances[name].On("Execute", ctx, pluginConfigs[name].Configuration, cancelFlag, mock.Anything).Return()

		pluginFactory := new(PluginFactoryMock)
		pluginFactory.On("Create", mock.Anything).Return(pluginInstances[name], nil)
		pluginRegistry[name] = pluginFactory

		pluginConfigs2[index] = pluginConfigs[name]
	}

	ch := make(chan contracts.PluginResult)
	defer func() {
		close(ch)
	}()
	go func() {
		for result := range ch {
			result.EndDateTime = defaultTime
			result.StartDateTime = defaultTime
		}
	}()
	// call the code we are testing
	outputs := RunPlugins(ctx, pluginConfigs2, ioConfig, pluginRegistry, ch, cancelFlag)

	for _, result := range outputs {
		result.EndDateTime = defaultTime
		result.StartDateTime = defaultTime
	}

	// assert that the expectations were met
	for _, mockPlugin := range pluginInstances {
		mockPlugin.AssertExpectations(t)
	}
	for pluginID, output := range outputs {
		assert.Equal(t, pluginResults[pluginID].StandardOutput, output.StandardOutput)
	}
}

func TestGetStepNameV1Documents(t *testing.T) {
	inputPluginName := "testPluginName1"
	testProperties := make(map[string]string)
	PropID := "0.aws:plugin1"
	testProperties["id"] = PropID
	config := contracts.Configuration{
		PluginID:   testPlugin1,
		PluginName: testPlugin1,
		Properties: testProperties,
	}
	output, err := getStepName(inputPluginName, config)
	assert.Equal(t, err, nil)
	assert.Equal(t, output, PropID)
}

func TestGetStepNameV2Documents(t *testing.T) {
	inputPluginName := "testPluginName1"
	config := contracts.Configuration{
		PluginID:   "PluginID",
		PluginName: testPlugin1,
	}
	output, err := getStepName(inputPluginName, config)
	assert.Equal(t, err, nil)
	assert.Equal(t, output, config.PluginID)
}

func TestGetStepNameCloudWatchDocument(t *testing.T) {
	inputPluginName := "aws:cloudWatch"
	config := contracts.Configuration{
		PluginID:   testPlugin1,
		PluginName: testPlugin1,
	}
	output, err := getStepName(inputPluginName, config)
	assert.Equal(t, err, nil)
	assert.Equal(t, output, inputPluginName)
}

func TestGetStepNameError(t *testing.T) {
	inputPluginName := "error test"
	config := contracts.Configuration{
		PluginID:   testPlugin1,
		PluginName: testPlugin1,
	}
	_, err := getStepName(inputPluginName, config)
	assert.Nil(t, err)
}
