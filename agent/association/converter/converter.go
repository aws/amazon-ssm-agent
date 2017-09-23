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

// Package converter converts the plugin information from version 1.0 and 1.2 to version 2.0
package converter

import "github.com/aws/amazon-ssm-agent/agent/contracts"

// ConvertPluginsInformation converts plugin information from map to array to fit the requirements of document v2 schema
func ConvertPluginsInformation(pluginsInformation map[string]contracts.PluginState) []contracts.PluginState {

	instancePluginsInformation := make([]contracts.PluginState, len(pluginsInformation))
	index := 0
	for pluginID, pluginState := range pluginsInformation {
		pluginState.Name = pluginID
		pluginState.Id = pluginID
		instancePluginsInformation[index] = pluginState
		index++
	}
	return instancePluginsInformation
}
