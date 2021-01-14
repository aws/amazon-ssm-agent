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

// Package manager encapsulates everything related to long running plugin manager that starts, stops & configures long running plugins
package manager

import (
	"path/filepath"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/datastore"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/plugin"
)

// dataStoreT defines the operations that manager uses to interact with its data-store
type dataStoreT interface {
	Write(data map[string]plugin.PluginInfo) error
	Read() (map[string]plugin.PluginInfo, error)
}

// ds contains the implementation of long running plugin manager's dataStore
type ds struct {
	dsImpl  datastore.FsStore
	context context.T
}

// Write writes new data in the data-store
func (d ds) Write(data map[string]plugin.PluginInfo) error {
	location, fileName, err := getDataStoreLocation(d.context)
	if err != nil {
		return err
	}
	return d.dsImpl.Write(data, location, fileName)
}

// Read reads data from the data-store
func (d ds) Read() (map[string]plugin.PluginInfo, error) {
	_, fileName, err := getDataStoreLocation(d.context)
	if err != nil {
		return nil, err
	}
	return d.dsImpl.Read(fileName)
}

// getDataStoreLocation returns the absolute path where long running plugins data-store is saved.
func getDataStoreLocation(context context.T) (location, fileName string, err error) {
	var shortInstanceId string

	if shortInstanceId, err = context.Identity().ShortInstanceID(); err != nil {
		return
	}
	location = filepath.Join(appconfig.DefaultDataStorePath,
		shortInstanceId,
		appconfig.LongRunningPluginsLocation,
		appconfig.LongRunningPluginDataStoreLocation)
	fileName = filepath.Join(appconfig.DefaultDataStorePath,
		shortInstanceId,
		appconfig.LongRunningPluginsLocation,
		appconfig.LongRunningPluginDataStoreLocation,
		appconfig.LongRunningPluginDataStoreFileName)
	return
}

// isPlatformSupported returns if target plugin supported by current platform.
func isPlatformSupported(log log.T, pluginName string) bool {
	isSupported, _ := plugin.IsLongRunningPluginSupportedForCurrentPlatform(log, pluginName)
	return isSupported
}
