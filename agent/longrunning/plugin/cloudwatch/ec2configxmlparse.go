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

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/longrunning"
)

const (
	EC2ServiceConfigFileName = "config.xml"
	PluginName               = "AWS.EC2.Windows.CloudWatch.PlugIn"
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

// Ec2ConfigXmlParser is an interface for Ec2Config's configuration xml parser
type Ec2ConfigXmlParser interface {
	IsCloudWatchEnabled() (bool, error)
}

// Ec2ConfigXmlParserImpl provides functionality to parse the cloudwatch config state from Ec2Config's configuration xml
type Ec2ConfigXmlParserImpl struct {
	FileSysUtil longrunning.FileSysUtil
}

// IsCloudWatchEnabled returns true if the CloudWatch is enabled in Ec2Config xml file.
func (e *Ec2ConfigXmlParserImpl) IsCloudWatchEnabled() (bool, error) {
	var fileContent []byte
	var err error

	fileName := fileutil.BuildPath(
		appconfig.EC2ConfigSettingPath,
		EC2ServiceConfigFileName)

	if !e.FileSysUtil.Exists(fileName) {
		return false, nil
	}

	lock.RLock()
	defer lock.RUnlock()

	if fileContent, err = e.FileSysUtil.ReadFile(fileName); err != nil {
		return false, err
	}

	var configSettings Query
	if err = xml.Unmarshal(fileContent, &configSettings); err != nil {
		return false, err
	}

	if configSettings.Plugins.PluginList == nil {
		return false, fmt.Errorf("%v contains an invalid format", fileName)
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
