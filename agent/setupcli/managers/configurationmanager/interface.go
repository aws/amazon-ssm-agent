// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package configurationmanager

type IConfigurationManager interface {
	// IsAgentAlreadyConfigured returns true if agent already has a configuration file
	IsAgentAlreadyConfigured() bool
	// IsConfigAvailable returns true if config file is available in folder to configure the agent with
	IsConfigAvailable(folderPath string) bool
	// ConfigureAgent copies the config in the folder to the applicable location to configure the agent
	ConfigureAgent(folderPath string) error
}
