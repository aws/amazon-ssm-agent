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

import "github.com/aws/amazon-ssm-agent/agent/log"

// ConfigureAgent verifies the agent is not already configured, checks if configuration is available and configures the agent
func ConfigureAgent(log log.T, manager IConfigurationManager, folderPath string) error {
	if manager.IsAgentAlreadyConfigured() {
		log.Infof("Skipping configuration, agent is already configured")
		return nil
	}

	if !manager.IsConfigAvailable(folderPath) {
		log.Infof("Skipping configuration, No default config available at path '%s'", folderPath)
		return nil
	}

	err := manager.ConfigureAgent(folderPath)
	if err != nil {
		return err
	}

	log.Infof("Successfully configured agent")
	return nil
}
