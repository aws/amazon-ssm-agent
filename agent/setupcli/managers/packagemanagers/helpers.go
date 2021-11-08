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

package packagemanagers

import (
	"fmt"
	"path/filepath"

	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/servicemanagers"
)

// InstallAgent verifies we have all files for installation and attempts to install
func InstallAgent(pManager IPackageManager, sManager servicemanagers.IServiceManager, folderPath string) error {
	neededFiles := pManager.GetFilesReqForInstall()

	// Verify files are available in folder
	for _, fileName := range neededFiles {
		filePath := filepath.Join(folderPath, fileName)
		pathExists, err := common.FileExists(filePath)
		if err != nil {
			return fmt.Errorf("failed to determine if file '%s' exists: %v", filePath, err)
		} else if !pathExists {
			return fmt.Errorf("required file does not exist at path: %v", pathExists)
		}
	}

	if err := pManager.InstallAgent(folderPath); err != nil {
		return err
	}

	return sManager.ReloadManager()
}

// UninstallAgent calls uninstall of the manager for ease of extension in case we need retries
func UninstallAgent(manager IPackageManager) error {
	return manager.UninstallAgent()
}
