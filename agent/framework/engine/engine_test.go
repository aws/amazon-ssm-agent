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
)

// TestRunPlugins tests that RunPluginsWithRegistry calls all the expected plugins.
func TestRunPluginsWithRegistry(t *testing.T) {
	pluginNames := []string{"plugin1", "plugin2"}
	pluginConfigs := make(map[string]model.PluginState)
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginInstances := make(map[string]*plugin.Mock)
	pluginRegistry := runpluginutil.PluginRegistry{}
	documentID := "TestDocument"

	var cancelFlag task.CancelFlag
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
	time.Sleep(10 * time.Second)
}
