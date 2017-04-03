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
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/statemanager/model"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
)

var mockCancelFlag task.CancelFlag
var mockContext context.T
var mockSendResponse func(messageID string, pluginID string, results map[string]*contracts.PluginResult)
var mockTime time.Time

func init() {
	mockCancel := task.NewMockDefault()
	mockCancel.On("Canceled").Return(false)
	mockCancel.On("ShutDown").Return(false)
	mockCancelFlag = mockCancel

	mockContext = context.NewMockDefault()
	mockTime = time.Now()
	mockSendResponse = func(messageID string, pluginID string, results map[string]*contracts.PluginResult) {}
}

// TestConfigurePackageOutput tests that the engine produces the expected PluginResult from the output of the configurePackage plugin
func TestConfigurePackageOutput(t *testing.T) {
	documentID := "TestDocument"
	name := configurepackage.Name()

	// Set configurePackage dependency stubs for success
	stubs := configurepackage.SetStubs()
	defer stubs.Clear()

	// Build plugin registry with configurePackage
	pluginRegistry := runpluginutil.PluginRegistry{}
	configurePackagePlugin, _ := configurepackage.NewPlugin(pluginutil.DefaultPluginConfig())
	pluginRegistry[name] = configurePackagePlugin

	// Build request pluginConfigs
	input := configurepackage.ConfigurePackagePluginInput{
		Name:    "PVDriver",
		Version: "1.0.0",
		Action:  "Install",
	}
	config := contracts.Configuration{
		PluginID:   name,
		Properties: input,
	}
	pluginConfigs := make([]model.PluginState, 1)
	pluginConfigs[0] = model.PluginState{
		Name:          name,
		Id:            name,
		Configuration: config,
	}

	pluginResults := make(map[string]*contracts.PluginResult)
	pluginResults[name] = &contracts.PluginResult{
		PluginName:    name,
		StartDateTime: mockTime,
		EndDateTime:   mockTime,
		Output:        "Initiating PVDriver 1.0.0 install\nInitiating PVDriver 1.0.0 validate\nSuccessfully installed PVDriver 1.0.0",
		Status:        contracts.ResultStatusSuccess,
	}

	// Call RunPlugins
	outputs := RunPlugins(mockContext, documentID, "", pluginConfigs, pluginRegistry, mockSendResponse, nil, mockCancelFlag)

	// fix the times expectation.
	for _, result := range outputs {
		result.EndDateTime = mockTime
		result.StartDateTime = mockTime
	}

	// Verify output
	assert.Equal(t, pluginResults[name], outputs[name])
}
