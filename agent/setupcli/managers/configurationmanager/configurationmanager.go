// Copyright 2023 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package configurationmanager helps us to handle agent config in ssm-setup-cli
package configurationmanager

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/onprem"
)

// agentConfigFile represents the agent config file name
const agentConfigFile = "amazon-ssm-agent.json"

var (
	fileExists  = fileutil.Exists
	osOpen      = os.Open
	makeDir     = fileutil.MakeDirs
	osCreate    = os.Create
	ioCopy      = io.Copy
	fileWrite   = fileutil.WriteIntoFileWithPermissions
	readAllText = fileutil.ReadAllText
)

// configurationManager contains functions for handling agent configurations
type configurationManager struct {
}

// New returns a new instance of configuration manager
func New() *configurationManager {
	return &configurationManager{}
}

// IsConfigAvailable verifies whether the config is present or not in a specific folder path
func (m *configurationManager) IsConfigAvailable(folderPath string) bool {
	// verify in default agent folder path if folderPath passed is blank
	if folderPath == "" {
		return fileExists(filepath.Join(agentConfigFolderPath, agentConfigFile))
	}
	// verify agent config presence in folder path
	return fileExists(filepath.Join(folderPath, agentConfigFile))
}

// ConfigureAgent copies the config in the folder to the applicable location to configure the agent
func (m *configurationManager) ConfigureAgent(folderPath string) error {
	// source path where agent config is present
	srcPath := filepath.Join(folderPath, agentConfigFile)
	// destination path where agent config gets stored
	destPath := filepath.Join(agentConfigFolderPath, agentConfigFile)

	// Open agent config file stream
	source, err := osOpen(srcPath)
	if err != nil {
		return err
	}
	defer source.Close()

	// make directory if the agent config folder is not present with proper permissions
	err = makeDir(agentConfigFolderPath)
	if err != nil {
		return err
	}

	// write the config in the destination folder
	destination, err := osCreate(destPath)
	if err != nil {
		return err
	}
	defer destination.Close()
	_, err = ioCopy(destination, source)
	return err
}

// CreateUpdateAgentConfigWithOnPremIdentity copies the config in the folder to the applicable location to configure the agent
func (m *configurationManager) CreateUpdateAgentConfigWithOnPremIdentity() error {
	var err error
	configJsonData := make(map[string]interface{})

	// default agent config path
	defaultAgentConfigPath := filepath.Join(agentConfigFolderPath, agentConfigFile)

	// create agent config directory if already not created
	err = makeDir(agentConfigFolderPath)
	if err != nil {
		return fmt.Errorf("error while creating directory: %v", err)
	}

	// read config data and store it in a map
	if fileExists(defaultAgentConfigPath) {
		configJsonData, err = getExistingAgentConfigData(defaultAgentConfigPath)
		if err != nil {
			return err
		}
	}

	// update the agent config map with the Onprem identity
	identityRefObj := &appconfig.IdentityCfg{
		ConsumptionOrder: []string{onprem.IdentityType},
	}
	configJsonData["Identity"] = identityRefObj

	// Marshall into json string
	agentConfigJsonStr, err := jsonutil.Marshal(configJsonData)
	if err != nil {
		return fmt.Errorf("error while updating identity: %v", err)
	}

	// Update agent config with On-prem identity added
	if s, err := fileWrite(defaultAgentConfigPath, jsonutil.Indent(agentConfigJsonStr), os.FileMode(int(appconfig.ReadWriteAccess))); s && err == nil {
		return nil
	}
	return fmt.Errorf("error while writing config file with Onprem identity: %v", err)
}

// getExistingAgentConfigData gets the agent config data and store it in a map
func getExistingAgentConfigData(agentConfigPath string) (map[string]interface{}, error) {
	var configJsonData map[string]interface{}
	data, err := readAllText(agentConfigPath)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	err = jsonutil.Unmarshal(data, &configJsonData)
	if err != nil {
		return nil, fmt.Errorf("error while unmarshalling json file file: %v", err)
	}

	return configJsonData, nil
}
