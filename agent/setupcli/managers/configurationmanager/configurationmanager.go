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

import (
	"io"
	"os"
	"path/filepath"

	"github.com/aws/amazon-ssm-agent/agent/fileutil"
)

const agentConfigFile = "amazon-ssm-agent.json"

type configurationManager struct {
}

func (m *configurationManager) IsAgentAlreadyConfigured() bool {
	return fileutil.Exists(filepath.Join(agentConfigFolderPath, agentConfigFile))
}

func (m *configurationManager) IsConfigAvailable(folderPath string) bool {
	return fileutil.Exists(filepath.Join(folderPath, agentConfigFile))
}

func (m *configurationManager) ConfigureAgent(folderPath string) error {
	srcPath := filepath.Join(folderPath, agentConfigFile)
	destPath := filepath.Join(agentConfigFolderPath, agentConfigFile)

	source, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer source.Close()

	err = fileutil.MakeDirs(agentConfigFolderPath)
	if err != nil {
		return err
	}

	destination, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

func New() *configurationManager {
	return &configurationManager{}
}
