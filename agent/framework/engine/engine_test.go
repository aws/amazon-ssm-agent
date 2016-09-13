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
	"github.com/aws/amazon-ssm-agent/agent/rebooter"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
)

// TestRunPlugins tests that RunPluginsWithRegistry calls all the expected plugins.
func TestRunPluginsWithRegistry(t *testing.T) {
	pluginNames := []string{"plugin1", "plugin2"}
	pluginConfigs := make(map[string]*contracts.Configuration)
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginInstances := make(map[string]*plugin.Mock)
	pluginRegistry := plugin.PluginRegistry{}
	documentID := "TestDocument"

	sendResponse := func(messageID string, pluginID string, results map[string]*contracts.PluginResult) {
	}

	var cancelFlag task.CancelFlag
	ctx := context.NewMockDefault()
	defaultTime := time.Now()
	for _, name := range pluginNames {

		// create an instance of our test object
		pluginInstances[name] = new(plugin.Mock)

		// setup expectations
		pluginConfigs[name] = &contracts.Configuration{}
		pluginResults[name] = &contracts.PluginResult{
			Output:        name,
			StartDateTime: defaultTime,
			EndDateTime:   defaultTime,
		}

		if name == "plugin2" {
			pluginResults[name].Status = contracts.ResultStatusSuccessAndReboot
		}

		pluginInstances[name].On("Execute", ctx, *pluginConfigs[name], cancelFlag).Return(*pluginResults[name])
		pluginRegistry[name] = pluginInstances[name]
	}

	// call the code we are testing
	outputs := RunPlugins(ctx, documentID, pluginConfigs, pluginRegistry, sendResponse, nil, cancelFlag)

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
	assert.Equal(t, pluginResults, outputs)
	time.Sleep(10 * time.Second)
	assert.Equal(t, true, rebooter.RebootRequested())
}
