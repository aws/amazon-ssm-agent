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

package runtimeconfighandler

import (
	"fmt"
	"path/filepath"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider/datastore/filesystem"
)

var newFileSystem = filesystem.NewFileSystem

func NewRuntimeConfigHandler(configName string) IRuntimeConfigHandler {
	return &runtimeConfigHandler{
		configName: configName,
		fileSystem: newFileSystem(),
	}
}

type IRuntimeConfigHandler interface {
	ConfigExists() (bool, error)
	GetConfig() ([]byte, error)
	SaveConfig([]byte) error
}

type runtimeConfigHandler struct {
	configName string
	fileSystem filesystem.IFileSystem
}

func (r *runtimeConfigHandler) ConfigExists() (bool, error) {
	configPath := filepath.Join(appconfig.RuntimeConfigFolderPath, r.configName)
	if _, err := r.fileSystem.Stat(configPath); r.fileSystem.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("failed to check if runtime config '%s' exists: %v", r.configName, err)
	}

	return true, nil
}

func (r *runtimeConfigHandler) GetConfig() ([]byte, error) {
	configPath := filepath.Join(appconfig.RuntimeConfigFolderPath, r.configName)
	bytesContent, err := r.fileSystem.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read runtime config '%s': %v", r.configName, err)
	}

	return bytesContent, nil
}

func (r *runtimeConfigHandler) SaveConfig(content []byte) error {
	if err := r.fileSystem.MkdirAll(appconfig.RuntimeConfigFolderPath, appconfig.ReadWriteExecuteAccess); err != nil {
		return fmt.Errorf("failed to create runtime config folder '%s': %v", appconfig.RuntimeConfigFolderPath, err)
	}

	configPath := filepath.Join(appconfig.RuntimeConfigFolderPath, r.configName)
	if err := r.fileSystem.WriteFile(configPath, content, appconfig.ReadWriteAccess); err != nil {
		return fmt.Errorf("failed to write runtime config '%s': %v", r.configName, err)
	}

	return nil
}
