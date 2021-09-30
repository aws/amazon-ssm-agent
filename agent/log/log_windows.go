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

//go:build windows
// +build windows

package log

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/cihub/seelog"
)

// Default log directory
var DefaultLogDir = filepath.Join(appconfig.SSMDataPath, "Logs")
var ExeNamePlaceHolder = "{{EXECUTABLENAME}}"

// The underlying logger is based of https://github.com/cihub/seelog
// See Seelog documentation to customize the logger
var DefaultSeelogConfigFilePath = appconfig.SeelogFilePath

var getExePath = getExecutablePath

// getLogConfigBytes reads and returns the seelog configs from the config file path if present
// otherwise returns the seelog default configurations
// Windows uses default log configuration if there is no seelog.xml override provided.
func getLogConfigBytes() (logConfigBytes []byte) {
	var err error
	if logConfigBytes, err = ioutil.ReadFile(DefaultSeelogConfigFilePath); err != nil {
		logConfigBytes = defaultConfigForExe()
	}

	if logConfigStr := string(logConfigBytes); strings.Contains(logConfigStr, ExeNamePlaceHolder) {
		return []byte(strings.Replace(logConfigStr, ExeNamePlaceHolder, exeLogFileName(), -1))
	}

	return
}

func defaultConfigForExe() []byte {
	return LoadLog(DefaultLogDir, exeLogFileName()+".log", seelog.InfoStr)
}

func exeLogFileName() string {
	exeName := filepath.Base(getExePath())
	return strings.Replace(exeName, ".exe", "", -1)
}

func getExecutablePath() string {
	return os.Args[0]
}
