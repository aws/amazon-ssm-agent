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

// seelogConfig helps build the agent seelog configuration.
// This can be overridden by passing a custom configuration file in LogFileConfig

package log

import "path/filepath"

func DefaultConfig() []byte {
	return LoadLog(DefaultLogDir, LogFile)
}

func LoadLog(defaultLogDir string, logFile string) []byte {
	var logFilePath, errorFilePath string

	logFilePath = filepath.Join(defaultLogDir, logFile)
	errorFilePath = filepath.Join(defaultLogDir, ErrorFile)

	logConfig := `
<seelog type="adaptive" mininterval="2000000" maxinterval="100000000" critmsgcount="500" minlevel="info">
    <exceptions>
        <exception filepattern="test*" minlevel="error"/>
    </exceptions>
    <outputs formatid="fmtinfo">
        <console formatid="fmtinfo"/>

        `
	logConfig += `<rollingfile type="size" filename="` + logFilePath + `" maxsize="30000000" maxrolls="5"/>`
	logConfig += `
		<filter levels="error,critical" formatid="fmterror">
		`
	logConfig += `<rollingfile type="size" filename="` + errorFilePath + `" maxsize="10000000" maxrolls="5"/>`
	logConfig += `
        </filter>
    </outputs>
    <formats>
        <format id="fmterror" format="%Date %Time %LEVEL [%FuncShort @ %File.%Line] %Msg%n"/>
        <format id="fmtdebug" format="%Date %Time %LEVEL [%FuncShort @ %File.%Line] %Msg%n"/>
        <format id="fmtinfo" format="%Date %Time %LEVEL %Msg%n"/>
    </formats>
</seelog>
`
	return []byte(logConfig)
}
