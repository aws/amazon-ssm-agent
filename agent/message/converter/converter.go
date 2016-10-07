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

// Package converter converts the plugin state from version 1.0 and 1.2 to version 2.0
package converter

import (
	"github.com/aws/amazon-ssm-agent/agent/statemanager/model"
)

// ConvertPluginState converts plugin state from map to array to fit the requirement of association document v2 schema
func ConvertPluginState(pluginstateMap map[string]model.PluginState) []model.PluginState {

	pluginStates := make([]model.PluginState, len(pluginstateMap))
	index := 0
	for pluginId, pluginState := range pluginstateMap {
		pluginState.Id = pluginId
		pluginState.Name = pluginId
		pluginStates[index] = pluginState
		index++
	}
	return pluginStates
}
