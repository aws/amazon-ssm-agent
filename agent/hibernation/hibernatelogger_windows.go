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

// Package hibernation is responsible for the agent in hibernate mode.
package hibernation

import (
	"path/filepath"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
)

func getHibernateSeelogConfig() string {

	seelogConfigDirPath := filepath.Join(appconfig.SSMDataPath, "Logs")
	seelogConfigFilePath := filepath.Join(seelogConfigDirPath, hibernateLogFile)

	seelogConfig := `<seelog type="adaptive" mininterval="2000000" maxinterval="100000000" critmsgcount="500" minlevel="debug">
		<outputs formatid="fmtinfo">
			<console formatid="fmtinfo"/>
			<rollingfile type="size" filename="` + seelogConfigFilePath + `" maxsize="30000" maxrolls="2"/>
		</outputs>
		<formats>
			<format id="fmtinfo" format="%Date %Time %LEVEL %Msg%n"/>
		</formats>
	</seelog>`
	return seelogConfig
}
