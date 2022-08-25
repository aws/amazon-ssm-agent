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

//go:build freebsd || linux || netbsd || openbsd
// +build freebsd linux netbsd openbsd

// Package runpluginutil run plugin utility functions without referencing the actually plugin impl packages
package runpluginutil

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	testPlugin0           = "plugin0"
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

		pluginInstances[name].On("Execute", pluginConfigs[name].Configuration, cancelFlag, mock.Anything).Return()

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

// TODO cancelFlag should not fail subsequent plugins
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
			plugins[name].On("Execute", pluginState.Configuration, cancelFlag, mock.Anything).Run(func(args mock.Arguments) {
				flag := args.Get(1).(task.CancelFlag)
				flag.Set(task.ShutDown)
			}).Return()

		} else {
			plugins[name].On("Execute", pluginState.Configuration, cancelFlag, mock.Anything).Return()
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
	ioConfig := contracts.IOConfiguration{
		OrchestrationDirectory: "test",
	}

	var cancelFlag task.CancelFlag = task.NewChanneledCancelFlag()
	config := appconfig.SsmagentConfig{}
	config.Ssm.OrchestrationDirectoryCleanup = appconfig.OrchestrationDirCleanupForSuccessFailedCommand
	var ctx = context.NewMockDefaultWithConfig(config)
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
			plugins[name].On("Execute", pluginState.Configuration, cancelFlag, mock.Anything).Return()
		}
		pluginStates[index] = pluginState
		pluginFactory := new(PluginFactoryMock)
		pluginFactory.On("Create", mock.Anything).Return(plugins[name], nil)
		pluginRegistry[name] = pluginFactory
	}

	ch := make(chan contracts.PluginResult, 2)

	// Not Deletion case - ResultStatusNotStarted
	var deleteDirectoryFlag bool
	deleteDirectoryRef = func(dirName string) (err error) {
		deleteDirectoryFlag = true
		return nil
	}
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
	assert.False(t, deleteDirectoryFlag)
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
		plugin.On("Execute", pluginConfigs[name].Configuration, cancelFlag, mock.Anything).Return(*pluginResults[name])
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
func TestRunPluginsWithCompatiblePlatformPrecondition(t *testing.T) {
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

	// initial precondition: "StringEquals": ["platformType", "Linux"]
	parsedPreconditions := map[string][]contracts.PreconditionArgument{
		"StringEquals": {
			contracts.PreconditionArgument{
				InitialArgumentValue:  "platformType",
				ResolvedArgumentValue: "platformType",
			},
			contracts.PreconditionArgument{
				InitialArgumentValue:  "Linux",
				ResolvedArgumentValue: "Linux",
			},
		},
	}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         parsedPreconditions,
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
		pluginInstances[name].On("Execute", pluginConfigs[name].Configuration, cancelFlag, mock.Anything).Return()

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
func TestRunPluginsWithCompatiblePlatformPreconditionWithValueFirst(t *testing.T) {
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

	// initial precondition: "StringEquals": ["Linux", "platformType"]
	parsedPreconditions := map[string][]contracts.PreconditionArgument{
		"StringEquals": {
			contracts.PreconditionArgument{
				InitialArgumentValue:  "Linux",
				ResolvedArgumentValue: "Linux",
			},
			contracts.PreconditionArgument{
				InitialArgumentValue:  "platformType",
				ResolvedArgumentValue: "platformType",
			},
		},
	}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         parsedPreconditions,
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
		pluginInstances[name].On("Execute", pluginConfigs[name].Configuration, cancelFlag, mock.Anything).Return()

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

// Crossplatform document with incompatible platform precondition, steps must be skipped
// Precondition = "StringEquals": ["platformType", "Windows"]
func TestRunPluginsWithIncompatiblePlatformPrecondition(t *testing.T) {
	setIsSupportedMock()
	defer restoreIsSupported()
	pluginNames := []string{testPlugin1, testPlugin2}
	pluginConfigs := make(map[string]contracts.PluginState)
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginInstances := make(map[string]*PluginMock)
	pluginRegistry := PluginRegistry{}
	ioConfig := contracts.IOConfiguration{
		OrchestrationDirectory: "test",
	}

	var cancelFlag task.CancelFlag = task.NewChanneledCancelFlag()

	config := appconfig.SsmagentConfig{}
	config.Ssm.OrchestrationDirectoryCleanup = appconfig.OrchestrationDirCleanupForSuccessFailedCommand
	var ctx = context.NewMockDefaultWithConfig(config)

	defaultTime := time.Now()
	defaultOutput := ""
	pluginConfigs2 := make([]contracts.PluginState, len(pluginNames))

	// initial precondition: "StringEquals": ["platformType", "Windows"]
	parsedPreconditions := map[string][]contracts.PreconditionArgument{
		"StringEquals": {
			contracts.PreconditionArgument{
				InitialArgumentValue:  "platformType",
				ResolvedArgumentValue: "platformType",
			},
			contracts.PreconditionArgument{
				InitialArgumentValue:  "Windows",
				ResolvedArgumentValue: "Windows",
			},
		},
	}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         parsedPreconditions,
		}

		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}
		pluginResults[name] = &contracts.PluginResult{
			Output:         "Step execution skipped due to unsatisfied preconditions: '\"StringEquals\": [platformType, Windows]'. Step name: " + name,
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

	// Deletion case - ResultStatusSkipped
	var deleteDirectoryFlag bool
	deleteDirectoryRef = func(dirName string) (err error) {
		deleteDirectoryFlag = true
		return nil
	}

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
	assert.True(t, deleteDirectoryFlag)
	assert.Equal(t, pluginResults[testPlugin1], outputs[testPlugin1])
	assert.Equal(t, pluginResults[testPlugin2], outputs[testPlugin2])

	assert.Equal(t, pluginResults, outputs)
}

// Crossplatform document preconditions must be forward-compatible for future platform types
// Preconditions with such OS types will be skipped for now
// Precondition = "StringEquals": ["platformType", "FutureOS"]
func TestRunPluginsWithFuturePlatformPrecondition(t *testing.T) {
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

	// initial precondition: "StringEquals": ["platformType", "FutureOS"]
	parsedPreconditions := map[string][]contracts.PreconditionArgument{
		"StringEquals": {
			contracts.PreconditionArgument{
				InitialArgumentValue:  "platformType",
				ResolvedArgumentValue: "platformType",
			},
			contracts.PreconditionArgument{
				InitialArgumentValue:  "FutureOS",
				ResolvedArgumentValue: "FutureOS",
			},
		},
	}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         parsedPreconditions,
		}

		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}
		pluginResults[name] = &contracts.PluginResult{
			Output:         "Step execution skipped due to unsatisfied preconditions: '\"StringEquals\": [platformType, FutureOS]'. Step name: " + name,
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
	pluginNames := []string{testPlugin1, testUnsupportedPlugin, testPlugin2}
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

	// initial precondition: "StringEquals": ["platformType", "Linux"]
	parsedPreconditions := map[string][]contracts.PreconditionArgument{
		"StringEquals": {
			contracts.PreconditionArgument{
				InitialArgumentValue:  "platformType",
				ResolvedArgumentValue: "platformType",
			},
			contracts.PreconditionArgument{
				InitialArgumentValue:  "Linux",
				ResolvedArgumentValue: "Linux",
			},
		},
	}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         parsedPreconditions,
		}

		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}

		if name == testUnsupportedPlugin {
			pluginResults[name] = &contracts.PluginResult{
				Output:         "Step execution skipped due to unsupported plugin: " + name + ". Step name: " + name,
				PluginName:     name,
				PluginID:       name,
				StartDateTime:  defaultTime,
				EndDateTime:    defaultTime,
				StandardOutput: defaultOutput,
				StandardError:  defaultOutput,
				Status:         contracts.ResultStatusSkipped,
			}
		} else {
			pluginResults[name] = &contracts.PluginResult{
				Output:        "",
				PluginID:      name,
				PluginName:    name,
				StartDateTime: defaultTime,
				EndDateTime:   defaultTime,
			}
			pluginInstances[name].On("Execute", pluginConfigs[name].Configuration, cancelFlag, mock.Anything).Return(*pluginResults[name])
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
	assert.Equal(t, pluginResults[testUnsupportedPlugin], outputs[testUnsupportedPlugin])
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
	ioConfig := contracts.IOConfiguration{
		OrchestrationDirectory: "test",
	}

	var cancelFlag task.CancelFlag = task.NewChanneledCancelFlag()

	config := appconfig.SsmagentConfig{}
	config.Ssm.OrchestrationDirectoryCleanup = appconfig.OrchestrationDirCleanupForSuccessFailedCommand
	var ctx = context.NewMockDefaultWithConfig(config)

	defaultTime := time.Now()
	defaultOutput := ""
	pluginConfigs2 := make([]contracts.PluginState, len(pluginNames))

	// initial precondition: {"StringEquals": ["Linux", "platformType"], "foo": ["operand1", "operand2"]}
	parsedPreconditions := map[string][]contracts.PreconditionArgument{
		"StringEquals": {
			contracts.PreconditionArgument{
				InitialArgumentValue:  "Linux",
				ResolvedArgumentValue: "Linux",
			},
			contracts.PreconditionArgument{
				InitialArgumentValue:  "platformType",
				ResolvedArgumentValue: "platformType",
			},
		},
		"foo": {
			contracts.PreconditionArgument{
				InitialArgumentValue:  "operand1",
				ResolvedArgumentValue: "operand1",
			},
			contracts.PreconditionArgument{
				InitialArgumentValue:  "operand2",
				ResolvedArgumentValue: "operand2",
			},
		},
	}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         parsedPreconditions,
		}

		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}

		pluginError := fmt.Sprintf(
			"Unrecognized precondition(s): 'unrecognized operator: \"foo\"', please update agent to latest version. Step name: %s",
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
	// Deletion case - ResultStatusFailed
	var deleteDirectoryFlag bool
	deleteDirectoryRef = func(dirName string) (err error) {
		deleteDirectoryFlag = true
		return nil
	}
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

	assert.True(t, deleteDirectoryFlag)
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

	// initial precondition: "foo": ["platformType", "Linux"]
	parsedPreconditions := map[string][]contracts.PreconditionArgument{
		"foo": {
			contracts.PreconditionArgument{
				InitialArgumentValue:  "platformType",
				ResolvedArgumentValue: "platformType",
			},
			contracts.PreconditionArgument{
				InitialArgumentValue:  "Linux",
				ResolvedArgumentValue: "Linux",
			},
		},
	}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         parsedPreconditions,
		}

		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}

		pluginError := fmt.Sprintf(
			"Unrecognized precondition(s): 'unrecognized operator: \"foo\"', please update agent to latest version. Step name: %s",
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

	// initial precondition: "StringEquals": ["platformType", "platformType"]
	parsedPreconditions := map[string][]contracts.PreconditionArgument{
		"StringEquals": {
			contracts.PreconditionArgument{
				InitialArgumentValue:  "platformType",
				ResolvedArgumentValue: "platformType",
			},
			contracts.PreconditionArgument{
				InitialArgumentValue:  "platformType",
				ResolvedArgumentValue: "platformType",
			},
		},
	}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         parsedPreconditions,
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

// Crossplatform document with invalid precondition, steps must fail
// Precondition: "StringEquals": ["{{ paramName }}", "{{ paramName }}"]
func TestRunPluginsWithUnrecognizedPreconditionDuplicateParameter(t *testing.T) {
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

	// initial precondition: "StringEquals": ["{{ paramName }}", "{{ paramName }}"]
	parsedPreconditions := map[string][]contracts.PreconditionArgument{
		"StringEquals": {
			contracts.PreconditionArgument{
				InitialArgumentValue:  "{{ paramName }}",
				ResolvedArgumentValue: "paramValue",
			},
			contracts.PreconditionArgument{
				InitialArgumentValue:  "{{ paramName }}",
				ResolvedArgumentValue: "paramValue",
			},
		},
	}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         parsedPreconditions,
		}

		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}

		pluginError := fmt.Sprintf(
			"Unrecognized precondition(s): '\"StringEquals\": operator's arguments can't be identical', please update agent to latest version. Step name: %s",
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
// Precondition: "StringEquals": ["foo", "foo"]
func TestRunPluginsWithUnrecognizedPreconditionDuplicateConstant(t *testing.T) {
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

	// initial precondition: "StringEquals": ["foo", "foo"]
	parsedPreconditions := map[string][]contracts.PreconditionArgument{
		"StringEquals": {
			contracts.PreconditionArgument{
				InitialArgumentValue:  "foo",
				ResolvedArgumentValue: "foo",
			},
			contracts.PreconditionArgument{
				InitialArgumentValue:  "foo",
				ResolvedArgumentValue: "foo",
			},
		},
	}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         parsedPreconditions,
		}

		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}

		pluginError := fmt.Sprintf(
			"Unrecognized precondition(s): '\"StringEquals\": operator's arguments can't be identical', please update agent to latest version. Step name: %s",
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
// Precondition: "StringEquals": ["{{ ssm:parameter }}", "bar"]
func TestRunPluginsWithUnrecognizedPreconditionSSMParameter(t *testing.T) {
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

	// initial precondition: "StringEquals": ["{{ ssm:parameter }}", "bar"]
	parsedPreconditions := map[string][]contracts.PreconditionArgument{
		"StringEquals": {
			contracts.PreconditionArgument{
				InitialArgumentValue:  "{{ ssm:parameter }}",
				ResolvedArgumentValue: "{{ ssm:parameter }}",
			},
			contracts.PreconditionArgument{
				InitialArgumentValue:  "bar",
				ResolvedArgumentValue: "bar",
			},
		},
	}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         parsedPreconditions,
		}

		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}

		pluginError := fmt.Sprintf(
			"Unrecognized precondition(s): '\"StringEquals\": operator's arguments can't contain SSM parameters', please update agent to latest version. Step name: %s",
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
// Precondition: "StringEquals": ["{{ ssm-secure:parameter }}", "bar"]
func TestRunPluginsWithUnrecognizedPreconditionSecureSSMParameter(t *testing.T) {
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

	// initial precondition: "StringEquals": ["{{ ssm-secure:parameter }}", "bar"]
	parsedPreconditions := map[string][]contracts.PreconditionArgument{
		"StringEquals": {
			contracts.PreconditionArgument{
				InitialArgumentValue:  "{{ ssm-secure:parameter }}",
				ResolvedArgumentValue: "{{ ssm-secure:parameter }}",
			},
			contracts.PreconditionArgument{
				InitialArgumentValue:  "bar",
				ResolvedArgumentValue: "bar",
			},
		},
	}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         parsedPreconditions,
		}

		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}

		pluginError := fmt.Sprintf(
			"Unrecognized precondition(s): '\"StringEquals\": operator's arguments can't contain secure SSM parameters', please update agent to latest version. Step name: %s",
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
// Precondition: "StringEquals": ["foo", "bar"]
func TestRunPluginsWithUnrecognizedPreconditionNoDocumentParameters(t *testing.T) {
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

	// initial precondition: "StringEquals": ["foo", "bar"]
	parsedPreconditions := map[string][]contracts.PreconditionArgument{
		"StringEquals": {
			contracts.PreconditionArgument{
				InitialArgumentValue:  "foo",
				ResolvedArgumentValue: "foo",
			},
			contracts.PreconditionArgument{
				InitialArgumentValue:  "bar",
				ResolvedArgumentValue: "bar",
			},
		},
	}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         parsedPreconditions,
		}

		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}

		pluginError := fmt.Sprintf(
			"Unrecognized precondition(s): '\"StringEquals\": at least one of operator's arguments must contain a valid document parameter', please update agent to latest version. Step name: %s",
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
// Precondition: "StringEquals": ["{{ unknown }} foo", "{{ unknown }} foo"]
func TestRunPluginsWithUnrecognizedPreconditionUnrecognizedParameter(t *testing.T) {
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

	// initial precondition: "StringEquals": ["{{ unknown }}", "bar"]
	parsedPreconditions := map[string][]contracts.PreconditionArgument{
		"StringEquals": {
			contracts.PreconditionArgument{
				InitialArgumentValue:  "{{ unknown }}",
				ResolvedArgumentValue: "{{ unknown }}",
			},
			contracts.PreconditionArgument{
				InitialArgumentValue:  "bar",
				ResolvedArgumentValue: "bar",
			},
		},
	}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         parsedPreconditions,
		}

		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}

		pluginError := fmt.Sprintf(
			"Unrecognized precondition(s): '\"StringEquals\": at least one of operator's arguments must contain a valid document parameter', please update agent to latest version. Step name: %s",
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

// Crossplatform document with platformType and document parameters should fail
// Precondition: "StringEquals": ["platformType", "{{ platformValue }}=Linux"]
func TestRunPluginsPlatformPreconditionWithDocumentParameters(t *testing.T) {
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

	// initial precondition: "StringEquals": ["platformType", "{{ platformValue }}"]
	parsedPreconditions := map[string][]contracts.PreconditionArgument{
		"StringEquals": {
			contracts.PreconditionArgument{
				InitialArgumentValue:  "platformType",
				ResolvedArgumentValue: "platformType",
			},
			contracts.PreconditionArgument{
				InitialArgumentValue:  "{{ platformValue }}",
				ResolvedArgumentValue: "Linux",
			},
		},
	}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         parsedPreconditions,
		}

		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}

		pluginError := fmt.Sprintf(
			"Unrecognized precondition(s): '\"StringEquals\": the second argument for the platformType variable can't contain document parameters', please update agent to latest version. Step name: %s",
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

// Crossplatform document with compatible precondition, steps must be executed
// Precondition = "StringEquals": ["{{ param1 }}", "{{ param2 }}"]
func TestRunPluginsWithCompatibleParamParamPrecondition(t *testing.T) {
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

	// initial precondition: "StringEquals": ["{{ param1 }}", "{{ param2 }}"]
	parsedPreconditions := map[string][]contracts.PreconditionArgument{
		"StringEquals": {
			contracts.PreconditionArgument{
				InitialArgumentValue:  "{{ param1 }}",
				ResolvedArgumentValue: "foo",
			},
			contracts.PreconditionArgument{
				InitialArgumentValue:  "{{ param2 }}",
				ResolvedArgumentValue: "foo",
			},
		},
	}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         parsedPreconditions,
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
		pluginInstances[name].On("Execute", pluginConfigs[name].Configuration, cancelFlag, mock.Anything).Return()

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

// Crossplatform document with incompatible platform precondition, steps must be skipped
// Precondition = "StringEquals": ["{{ param1 }}", "{{ param2 }}"]
func TestRunPluginsWithIncompatibleParamParamPrecondition(t *testing.T) {
	setIsSupportedMock()
	defer restoreIsSupported()
	pluginNames := []string{testPlugin1, testPlugin2}
	pluginConfigs := make(map[string]contracts.PluginState)
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginInstances := make(map[string]*PluginMock)
	pluginRegistry := PluginRegistry{}
	ioConfig := contracts.IOConfiguration{
		OrchestrationDirectory: "test",
	}

	var cancelFlag task.CancelFlag = task.NewChanneledCancelFlag()

	config := appconfig.SsmagentConfig{}
	config.Ssm.OrchestrationDirectoryCleanup = appconfig.OrchestrationDirCleanupForSuccessFailedCommand
	var ctx = context.NewMockDefaultWithConfig(config)
	defaultTime := time.Now()
	defaultOutput := ""
	pluginConfigs2 := make([]contracts.PluginState, len(pluginNames))

	// initial precondition: "StringEquals": ["{{ param1 }}", "{{ param2 }}"]
	parsedPreconditions := map[string][]contracts.PreconditionArgument{
		"StringEquals": {
			contracts.PreconditionArgument{
				InitialArgumentValue:  "{{ param1 }}",
				ResolvedArgumentValue: "foo",
			},
			contracts.PreconditionArgument{
				InitialArgumentValue:  "{{ param2 }}",
				ResolvedArgumentValue: "bar",
			},
		},
	}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         parsedPreconditions,
		}

		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}
		pluginResults[name] = &contracts.PluginResult{
			Output:         "Step execution skipped due to unsatisfied preconditions: '\"StringEquals\": [{{ param1 }}, {{ param2 }}]'. Step name: " + name,
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
	// Deletion case - ResultStatusSkipped
	var deleteDirectoryFlag bool
	deleteDirectoryRef = func(dirName string) (err error) {
		deleteDirectoryFlag = true
		return nil
	}
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
	assert.True(t, deleteDirectoryFlag)
	assert.Equal(t, pluginResults[testPlugin1], outputs[testPlugin1])
	assert.Equal(t, pluginResults[testPlugin2], outputs[testPlugin2])

	assert.Equal(t, pluginResults, outputs)
}

// Crossplatform document with compatible precondition, steps must be executed
// Precondition = "StringEquals": ["{{ param }}", "foo"]
func TestRunPluginsWithCompatibleParamValuePrecondition(t *testing.T) {
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

	// initial precondition: "StringEquals": ["{{ param }}", "foo"]
	parsedPreconditions := map[string][]contracts.PreconditionArgument{
		"StringEquals": {
			contracts.PreconditionArgument{
				InitialArgumentValue:  "{{ param }}",
				ResolvedArgumentValue: "",
			},
			contracts.PreconditionArgument{
				InitialArgumentValue:  "",
				ResolvedArgumentValue: "",
			},
		},
	}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         parsedPreconditions,
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
		pluginInstances[name].On("Execute", pluginConfigs[name].Configuration, cancelFlag, mock.Anything).Return()

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
// Precondition = "StringEquals": ["foo", "{{ param }}"]
func TestRunPluginsWithCompatibleValueParamPrecondition(t *testing.T) {
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

	// initial precondition: "StringEquals": ["foo", "{{ param }}"]
	parsedPreconditions := map[string][]contracts.PreconditionArgument{
		"StringEquals": {
			contracts.PreconditionArgument{
				InitialArgumentValue:  "foo",
				ResolvedArgumentValue: "foo",
			},
			contracts.PreconditionArgument{
				InitialArgumentValue:  "{{ param1 }}",
				ResolvedArgumentValue: "foo",
			},
		},
	}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         parsedPreconditions,
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
		pluginInstances[name].On("Execute", pluginConfigs[name].Configuration, cancelFlag, mock.Anything).Return()

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

// Crossplatform document with incompatible platform precondition, steps must be skipped
// Precondition = "StringEquals": ["{{ param }}", "bar}"]
func TestRunPluginsWithIncompatibleParamValuePrecondition(t *testing.T) {
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

	// initial precondition: "StringEquals": ["{{ param }}", "bar"]
	parsedPreconditions := map[string][]contracts.PreconditionArgument{
		"StringEquals": {
			contracts.PreconditionArgument{
				InitialArgumentValue:  "{{ param }}",
				ResolvedArgumentValue: "foo",
			},
			contracts.PreconditionArgument{
				InitialArgumentValue:  "bar",
				ResolvedArgumentValue: "bar",
			},
		},
	}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         parsedPreconditions,
		}

		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}
		pluginResults[name] = &contracts.PluginResult{
			Output:         "Step execution skipped due to unsatisfied preconditions: '\"StringEquals\": [{{ param }}, bar]'. Step name: " + name,
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

// Crossplatform document with compatible precondition, steps must be executed
// Precondition = "StringEquals": ["{{ param1 }} bar {{ not a param }}", "foo {{ param2 }} {{ not a param }}"]
func TestRunPluginsWithCompatibleMixedPrecondition(t *testing.T) {
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

	// initial precondition: "StringEquals": ["{{ param1 }} bar {{ not a param }}", "foo {{ param2 }} {{ not a param }}"]
	parsedPreconditions := map[string][]contracts.PreconditionArgument{
		"StringEquals": {
			contracts.PreconditionArgument{
				InitialArgumentValue:  "{{ param1 }} bar {{ not a param }}",
				ResolvedArgumentValue: "foo bar {{ not a param }}",
			},
			contracts.PreconditionArgument{
				InitialArgumentValue:  "foo {{ param2 }} {{ not a param }}",
				ResolvedArgumentValue: "foo bar {{ not a param }}",
			},
		},
	}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         parsedPreconditions,
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
		pluginInstances[name].On("Execute", pluginConfigs[name].Configuration, cancelFlag, mock.Anything).Return()

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
// Precondition = "StringEquals": ["{{ param1 }} bar {{ not a param }}", "foo {{ param2 }} {{ wrong not a param }}"]
func TestRunPluginsWithIncompatibleMixedPrecondition(t *testing.T) {
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

	// initial precondition: "StringEquals": ["{{ param1 }} bar {{ not a param }}", "{{ param2 }}"]
	parsedPreconditions := map[string][]contracts.PreconditionArgument{
		"StringEquals": {
			contracts.PreconditionArgument{
				InitialArgumentValue:  "{{ param1 }} bar {{ not a param }}",
				ResolvedArgumentValue: "foo bar {{ not a param }}",
			},
			contracts.PreconditionArgument{
				InitialArgumentValue:  "foo {{ param2 }} {{ wrong not a param }}",
				ResolvedArgumentValue: "foo bar {{ wrong not a param }}",
			},
		},
	}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         parsedPreconditions,
		}

		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}
		pluginResults[name] = &contracts.PluginResult{
			Output:         "Step execution skipped due to unsatisfied preconditions: '\"StringEquals\": [{{ param1 }} bar {{ not a param }}, foo {{ param2 }} {{ wrong not a param }}]'. Step name: " + name,
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

	// initial precondition: "StringEquals": ["platformType", "Linux", "foo"]
	parsedPreconditions := map[string][]contracts.PreconditionArgument{
		"StringEquals": {
			contracts.PreconditionArgument{
				InitialArgumentValue:  "platformType",
				ResolvedArgumentValue: "platformType",
			},
			contracts.PreconditionArgument{
				InitialArgumentValue:  "Linux",
				ResolvedArgumentValue: "Linux",
			},
			contracts.PreconditionArgument{
				InitialArgumentValue:  "foo",
				ResolvedArgumentValue: "foo",
			},
		},
	}

	for index, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(PluginMock)

		// create configuration for execution
		config := contracts.Configuration{
			PluginID:              name,
			PluginName:            name,
			IsPreconditionEnabled: true,
			Preconditions:         parsedPreconditions,
		}

		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}

		pluginError := fmt.Sprintf(
			"Unrecognized precondition(s): '\"StringEquals\": operator accepts exactly 2 arguments', please update agent to latest version. Step name: %s",
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

	// initial precondition: "StringEquals": ["platformType", "Linux"]
	parsedPreconditions := map[string][]contracts.PreconditionArgument{
		"StringEquals": {
			contracts.PreconditionArgument{
				InitialArgumentValue:  "platformType",
				ResolvedArgumentValue: "platformType",
			},
			contracts.PreconditionArgument{
				InitialArgumentValue:  "Linux",
				ResolvedArgumentValue: "Linux",
			},
		},
	}

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
			Preconditions:         parsedPreconditions,
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
			pluginInstances[name].On("Execute", pluginConfigs[name].Configuration, cancelFlag, mock.Anything).Return(*pluginResults[name])
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

		pluginInstances[name].On("Execute", pluginConfigs[name].Configuration, cancelFlag, mock.Anything).Return()

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

func TestGetShouldPluginSkipBasedOnControlFlow(t *testing.T) {
	pluginNames := []string{testPlugin0, testPlugin1, testPlugin2, testUnknownPlugin}
	plugins := make([]contracts.PluginState, len(pluginNames))
	pluginResults := make(map[string]*contracts.PluginResult)
	testProperties := make([]interface{}, len(pluginNames))
	config := make([]contracts.Configuration, len(pluginNames))
	ctx := context.NewMockDefault()
	defaultStatus := contracts.ResultStatusSuccess
	defaultStatus2 := contracts.ResultStatusSuccess
	pluginCode := []int{0, 0, 1, 168, 169}
	pluginCode2 := []int{1, 0, 168, 169, 0}
	// plugin config properties
	testProperties[0] = map[string]interface{}{
		contracts.OnFailureModifier: "exit",
	}
	testProperties[1] = map[string]interface{}{
		contracts.OnSuccessModifier:   "exit",
		contracts.FinallyStepModifier: "false",
	}
	testProperties[2] = map[string]interface{}{
		contracts.OnFailureModifier:   "exit",
		contracts.FinallyStepModifier: "true",
	}
	testProperties[3] = map[string]interface{}{
		contracts.OnFailureModifier:   "exit",
		contracts.FinallyStepModifier: "true",
	}
	for index, name := range pluginNames {
		config[index] = contracts.Configuration{
			PluginID:   name,
			PluginName: name,
			Properties: testProperties[index],
		}
		plugins[index] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config[index],
		}
	}

	for index, code := range pluginCode {
		if code == 1 || code == 169 {
			defaultStatus = contracts.ResultStatusFailed
		} else {
			defaultStatus = contracts.ResultStatusSuccess
		}
		pluginResults[testPlugin0] = &contracts.PluginResult{
			PluginID:   testPlugin0,
			PluginName: testPlugin0,
			Status:     defaultStatus,
			Code:       code,
		}
		// plugin 1
		if pluginCode2[index] == 1 || pluginCode2[index] == 169 {
			defaultStatus2 = contracts.ResultStatusFailed
		} else {
			defaultStatus2 = contracts.ResultStatusSuccess
		}
		pluginResults[testPlugin1] = &contracts.PluginResult{
			PluginID:   testPlugin1,
			PluginName: testPlugin1,
			Status:     defaultStatus2,
			Code:       pluginCode2[index],
		}
		// plugin 2
		pluginResults[testPlugin2] = &contracts.PluginResult{
			PluginID:   testPlugin2,
			PluginName: testPlugin2,
			Status:     contracts.ResultStatusSuccess,
			Code:       0,
		}
		// plugin testUnknownPlugin
		pluginResults[testUnknownPlugin] = &contracts.PluginResult{
			PluginID:   testUnknownPlugin,
			PluginName: testUnknownPlugin,
			Status:     contracts.ResultStatusSuccess,
			Code:       0,
		}

		shouldPluginSkipBasedOnControlFlow0 := getShouldPluginSkipBasedOnControlFlow(ctx, plugins, 0, pluginResults)
		shouldPluginSkipBasedOnControlFlow1 := getShouldPluginSkipBasedOnControlFlow(ctx, plugins, 1, pluginResults)
		shouldPluginSkipBasedOnControlFlow2 := getShouldPluginSkipBasedOnControlFlow(ctx, plugins, 2, pluginResults)
		shouldPluginSkipBasedOnControlFlow3 := getShouldPluginSkipBasedOnControlFlow(ctx, plugins, 3, pluginResults)
		assert.Equal(t, shouldPluginSkipBasedOnControlFlow0, false)
		assert.Equal(t, shouldPluginSkipBasedOnControlFlow3, false)
		// test getShouldPluginSkipBasedOnControlFlow
		if index == 0 {
			assert.Equal(t, shouldPluginSkipBasedOnControlFlow1, false)
			assert.Equal(t, shouldPluginSkipBasedOnControlFlow2, false)
		} else if index == 1 {
			assert.Equal(t, shouldPluginSkipBasedOnControlFlow1, false)
			assert.Equal(t, shouldPluginSkipBasedOnControlFlow2, true)
		} else {
			assert.Equal(t, shouldPluginSkipBasedOnControlFlow1, true)
			assert.Equal(t, shouldPluginSkipBasedOnControlFlow2, true)
		}
	}
}

func TestGetStringPropByName(t *testing.T) {
	testProperties := map[string]interface{}{
		contracts.OnFailureModifier:   "exit",
		contracts.OnSuccessModifier:   "exit",
		contracts.FinallyStepModifier: "true",
	}
	config := contracts.Configuration{
		PluginID:   testPlugin1,
		PluginName: testPlugin1,
		Properties: testProperties,
	}
	pluginState := contracts.PluginState{
		Name:          testPlugin1,
		Id:            testPlugin1,
		Configuration: config,
	}
	// test getStringPropByName
	onFailureOutput := getStringPropByName(config.Properties, contracts.OnFailureModifier)
	onSuccessOutput := getStringPropByName(config.Properties, contracts.OnSuccessModifier)
	finallyStepOutput := getStringPropByName(pluginState.Configuration.Properties, contracts.FinallyStepModifier)
	assert.Equal(t, onFailureOutput, "exit")
	assert.Equal(t, onSuccessOutput, "exit")
	assert.Equal(t, finallyStepOutput, "true")
}

func TestRunPluginWithOnFailureProperty168(t *testing.T) {
	setIsSupportedMock()
	defer restoreIsSupported()
	pluginNames := []string{testPlugin1}
	pluginConfigs := make(map[string]contracts.PluginState)
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginInstances := make(map[string]*PluginMock)
	pluginRegistry := PluginRegistry{}
	var cancelFlag task.CancelFlag
	ctx := context.NewMockDefault()
	defaultTime := time.Now()
	ioConfig := contracts.IOConfiguration{}
	pluginConfigs2 := make([]contracts.PluginState, len(pluginNames))

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
		standardOutput := "\nStep exited with code 168. Therefore, marking step as succeeded. Further document steps will be skipped."
		standardError := ""
		defaultCode := contracts.ExitWithSuccess
		defaultStatus := contracts.ResultStatusSuccess
		outputMessage := ""

		pluginResults[name] = &contracts.PluginResult{
			PluginID:       name,
			PluginName:     name,
			StartDateTime:  defaultTime,
			EndDateTime:    defaultTime,
			Status:         defaultStatus,
			Code:           defaultCode,
			StandardOutput: standardOutput,
			StandardError:  standardError,
			Output:         outputMessage,
		}

		oldRunPlugin := runPlugin
		runPlugin = func(context context.T,
			factory PluginFactory,
			pluginName string,
			config contracts.Configuration,
			cancelFlag task.CancelFlag,
			ioConfig contracts.IOConfiguration,
		) (res contracts.PluginResult) {
			res.Code = defaultCode
			res.Status = defaultStatus
			res.Output = ""
			return
		}
		defer func() { runPlugin = oldRunPlugin }()

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

	ctx.AssertCalled(t, "Log")
	assert.Equal(t, pluginResults[testPlugin1], outputs[testPlugin1])
}

func TestRunPluginWithOnFailureProperty169(t *testing.T) {
	setIsSupportedMock()
	defer restoreIsSupported()
	pluginNames := []string{testPlugin1}
	pluginConfigs := make(map[string]contracts.PluginState)
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginInstances := make(map[string]*PluginMock)
	pluginRegistry := PluginRegistry{}
	var cancelFlag task.CancelFlag
	ctx := context.NewMockDefault()
	defaultTime := time.Now()
	ioConfig := contracts.IOConfiguration{}
	pluginConfigs2 := make([]contracts.PluginState, len(pluginNames))

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
		standardOutput := "\nStep exited with code 169. Therefore, marking step as Failed. Further document steps will be skipped."
		standardError := standardOutput
		defaultCode := contracts.ExitWithFailure
		defaultStatus := contracts.ResultStatusFailed
		outputMessage := ""

		pluginResults[name] = &contracts.PluginResult{
			PluginID:       name,
			PluginName:     name,
			StartDateTime:  defaultTime,
			EndDateTime:    defaultTime,
			Status:         defaultStatus,
			Code:           defaultCode,
			StandardOutput: standardOutput,
			StandardError:  standardError,
			Output:         outputMessage,
		}

		oldRunPlugin := runPlugin
		runPlugin = func(context context.T,
			factory PluginFactory,
			pluginName string,
			config contracts.Configuration,
			cancelFlag task.CancelFlag,
			ioConfig contracts.IOConfiguration,
		) (res contracts.PluginResult) {
			res.Code = defaultCode
			res.Status = defaultStatus
			res.Output = ""
			return
		}
		defer func() { runPlugin = oldRunPlugin }()

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

	ctx.AssertCalled(t, "Log")
	assert.Equal(t, pluginResults[testPlugin1], outputs[testPlugin1])
}

func TestRunPluginWithOnFailureProperty1(t *testing.T) {
	setIsSupportedMock()
	defer restoreIsSupported()
	pluginNames := []string{testPlugin1}
	pluginConfigs := make(map[string]contracts.PluginState)
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginInstances := make(map[string]*PluginMock)
	pluginRegistry := PluginRegistry{}
	var cancelFlag task.CancelFlag
	ctx := context.NewMockDefault()
	defaultTime := time.Now()
	ioConfig := contracts.IOConfiguration{}
	pluginConfigs2 := make([]contracts.PluginState, len(pluginNames))

	for index, name := range pluginNames {
		// create mock plugin instance for testing
		pluginInstances[name] = new(PluginMock)

		// create properties
		testProperties := map[string]interface{}{
			contracts.OnFailureModifier: contracts.ModifierValueSuccessAndExit,
		}
		// create configuration for execution
		config := contracts.Configuration{
			PluginID:   name,
			PluginName: name,
			Properties: testProperties,
		}
		// setup expectations
		pluginConfigs[name] = contracts.PluginState{
			Name:          name,
			Id:            name,
			Configuration: config,
		}
		standardOutput := "\nStep was found to have onFailure property. Further document steps will be skipped."
		standardError := standardOutput
		defaultCode := 1
		defaultStatus := contracts.ResultStatusFailed
		outputMessage := ""

		pluginResults[name] = &contracts.PluginResult{
			PluginID:       name,
			PluginName:     name,
			StartDateTime:  defaultTime,
			EndDateTime:    defaultTime,
			Status:         contracts.ResultStatusSuccess,
			Code:           contracts.ExitWithSuccess,
			StandardOutput: standardOutput,
			StandardError:  standardError,
			Output:         outputMessage,
		}

		oldRunPlugin := runPlugin
		runPlugin = func(context context.T,
			factory PluginFactory,
			pluginName string,
			config contracts.Configuration,
			cancelFlag task.CancelFlag,
			ioConfig contracts.IOConfiguration,
		) (res contracts.PluginResult) {
			res.Code = defaultCode
			res.Status = defaultStatus
			res.Output = ""
			return
		}
		defer func() { runPlugin = oldRunPlugin }()

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

	ctx.AssertCalled(t, "Log")
	assert.Equal(t, pluginResults[testPlugin1], outputs[testPlugin1])
}
