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

// seelogConfig helps build the agent seelog configuration.
// This can be overridden by passing a custom configuration file in LogFileConfig

package log

import (
	"path/filepath"
)

func defaultConfig() []byte {
	return loadLog(DefaultLogDir, LogFile)
}

func defaultUpdaterConfig(logRoot string, logFile string) []byte {
	return loadLog(logRoot, logFile)
}

func loadLog(defaultLogDir string, logFile string) []byte {
	var logFilePath, errorFilePath string

	logFilePath = filepath.Join(defaultLogDir, logFile)
	errorFilePath = filepath.Join(defaultLogDir, ErrorFile)

	logConfig := `
<seelog type="adaptive" mininterval="2000000" maxinterval="100000000" critmsgcount="500" minlevel="debug">
    <exceptions>
        <exception filepattern="test*" minlevel="error"/>
    </exceptions>
    <outputs formatid="all">
        <console formatid="all"/>
        `
	logConfig += `<file path="` + logFilePath + `"/>`
	logConfig += `
		<filter levels="error,critical" formatid="fmterror">
		`
	logConfig += `<file path="` + errorFilePath + `"/>`
	logConfig += `
        </filter>
    </outputs>
    <formats>
        <format id="fmterror" format="%Date %Time %LEVEL [%FuncShort @ %File.%Line] %Msg%n"/>
        <format id="all" format="%Date %Time %LEVEL [%FuncShort @ %File.%Line] %Msg%n"/>
    </formats>
</seelog>
`
	return []byte(logConfig)
}
