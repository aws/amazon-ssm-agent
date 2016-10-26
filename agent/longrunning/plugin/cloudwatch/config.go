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

// Package cloudwatch implements cloudwatch plugin and its configuration
package cloudwatch

import (
	"encoding/json"
	"os"
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
)

// ConfigFileName represents the name of the configuration file for cloud watch plugin
const (
	ConfigFileName       = "AWS.EC2.Windows.CloudWatch.json"
	ConfigFileFolderName = "awsCloudWatch"
)

// cloudWatchConfig represents the data structure of cloudwatch configuration singleton,
// which contains the essential information to configure cloudwatch plugin
type CloudWatchConfig struct {
	IsEnabled           bool
	EngineConfiguration interface{}
}

var instance *CloudWatchConfig
var once sync.Once
var lock sync.RWMutex

// Initialze ensures the instance has been initialized
func Initialze() {
	once.Do(func() {
		instance = &CloudWatchConfig{}
	})
}

// Instance returns a singleton of CloudWatchConfig instance
func Instance() *CloudWatchConfig {
	return instance
}

// ParseEngineConfiguration marshals the EngineConfiguration from interface{} to string
func ParseEngineConfiguration() (config string, err error) {
	switch instance.EngineConfiguration.(type) {
	case string:
		var bytes []byte
		rawIn := json.RawMessage(instance.EngineConfiguration.(string))
		bytes, err = rawIn.MarshalJSON()
		config = string(bytes[:])
	default:
		config, err = jsonutil.Marshal(instance.EngineConfiguration)
	}

	return
}

// Update updates configuration from file system
func Update() error {
	var cwConfig CloudWatchConfig
	//var config CloudWatchConfig
	var err error
	if cwConfig, err = load(); err != nil {
		return err
	}

	instance.IsEnabled = cwConfig.IsEnabled
	instance.EngineConfiguration = cwConfig.EngineConfiguration

	return err
}

// Write writes the updated configuration of cloud watch to file system
func Write() error {
	lock.Lock()
	defer lock.Unlock()
	fileName := getFileName()
	location := getLocation()
	var err error
	var content string

	content, err = jsonutil.Marshal(instance)
	if err != nil {
		return err
	}

	//verify if parent folder exist
	if !fileUtilWrapper.Exists(location) {
		if err = fileUtilWrapper.MakeDirs(location); err != nil {
			return err
		}
	}

	//it's fine even if we overwrite the content of previous file
	if _, err = fileUtilWrapper.WriteIntoFileWithPermissions(
		fileName,
		content,
		os.FileMode(int(appconfig.ReadWriteAccess))); err != nil {
		return err
	}

	return nil
}

// Enable changes the IsEnabled property in cloud watch config from false to true
func Enable(config *CloudWatchConfig) error {
	instance.IsEnabled = true
	instance.EngineConfiguration = config.EngineConfiguration
	return Write()
}

// Disable changes the IsEnabled property in cloud watch config from true to false
func Disable() error {
	instance.IsEnabled = false
	return Write()
}

// load reads cloud watch plugin configuration from config store (file system)
func load() (CloudWatchConfig, error) {
	lock.RLock()
	defer lock.RUnlock()
	fileName := getFileName()
	var err error
	var cwConfig CloudWatchConfig

	err = jsonutil.UnmarshalFile(fileName, &cwConfig)

	return cwConfig, err
}

// getFileName returns the full name of the cloud watch config file.
func getFileName() string {
	return fileutil.BuildPath(appconfig.DefaultPluginPath, ConfigFileFolderName, ConfigFileName)
}

// getLocation returns the absolute path of the cloud watch config file folder.
func getLocation() string {
	return fileutil.BuildPath(appconfig.DefaultPluginPath, ConfigFileFolderName)
}
