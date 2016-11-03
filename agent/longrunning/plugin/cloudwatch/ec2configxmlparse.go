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
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
)

const (
	EC2SeriveceConfigFileName = "config.xml"
	PluginName                = "AWS.EC2.Windows.CloudWatch.PlugIn"
)

type Query struct {
	Plugins PluginState `xml:"Plugins"`
}

type PluginState struct {
	PluginList []PluginInfo `xml:"Plugin"`
}

type PluginInfo struct {
	Name  string `xml:"Name"`
	State string `xml:"State"`
}

// ParseXml parses the ec2config xml file and check if the aws:cloudWatch plugin is enabled or not.
func ParseXml() (bool, error) {
	lock.RLock()
	defer lock.RUnlock()

	fileName := fileutil.BuildPath(
		appconfig.EC2ConfigSettingPath,
		EC2SeriveceConfigFileName)

	if !fileutil.Exists(fileName) {
		return false, nil
	}

	xmlFile, err := os.Open(fileName)
	if err != nil {
		return false, err
	}
	defer xmlFile.Close()

	var fileContent []byte
	if fileContent, err = ioutil.ReadAll(xmlFile); err != nil {
		return false, err
	}

	var configSettings Query
	if err = xml.Unmarshal(fileContent, &configSettings); err != nil {
		return false, err
	}

	for _, configSetting := range configSettings.Plugins.PluginList {
		if configSetting.Name == PluginName {
			switch configSetting.State {
			case "Enabled":
				return true, nil
			case "Disabled":
				return false, nil
			default:
				return false, fmt.Errorf("Allowed value for plugin state in config file: Enabled|Disabled, %v is not allowed.", configSetting.State)
			}
		}
	}
	return false, nil
}
