// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// +build darwin freebsd linux netbsd openbsd

package log

import (
	"fmt"
	"io/ioutil"
)

const (
	// DefaultSeelogConfigFilePath specifies the default seelog location
	// The underlying logger is based of https://github.com/cihub/seelog
	// See Seelog documentation to customize the logger
	DefaultSeelogConfigFilePath = "/etc/amazon/ssm/seelog.xml"

	DefaultLogDir = "/var/log/amazon/ssm"
)

// InitLogger initializes the logger using the settings specified in the application config file.
// otherwise initializes the logger based on default settings
// Linux uses seelog.xml file as configuration by default.
func initLogger() (logger T) {
	var logConfigBytes []byte
	var err error
	if logConfigBytes, err = ioutil.ReadFile(DefaultSeelogConfigFilePath); err != nil {
		fmt.Println("Error occured fetching the seelog config file path: ", err)
		logConfigBytes = defaultConfig()
	}

	return initLoggerFromBytes(logConfigBytes)
}
