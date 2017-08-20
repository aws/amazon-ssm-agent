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

// +build windows

package log

import (
	"io/ioutil"
	"path/filepath"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
)

// Default log directory
var DefaultLogDir = filepath.Join(appconfig.SSMDataPath, "Logs")

// The underlying logger is based of https://github.com/cihub/seelog
// See Seelog documentation to customize the logger
var DefaultSeelogConfigFilePath = filepath.Join(appconfig.DefaultProgramFolder, appconfig.SeelogConfigFileName)

// getLogConfigBytes reads and returns the seelog configs from the config file path if present
// otherwise returns the seelog default configurations
// Windows uses default log configuration if there is no seelog.xml override provided.
func getLogConfigBytes() (logConfigBytes []byte) {
	var err error
	if logConfigBytes, err = ioutil.ReadFile(DefaultSeelogConfigFilePath); err != nil {
		logConfigBytes = DefaultConfig()
	}
	return
}
