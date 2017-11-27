// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package service

import (
	"encoding/json"
	"fmt"

	"os/exec"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/twinj/uuid"
)

var (
	startMarker       = "<start" + randomString(8) + ">"
	endMarker         = "<end" + randomString(8) + ">"
	serviceInfoScript = `
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$serviceInfo = Get-Service | Select-Object Name, DisplayName, Status, DependentServices, ServicesDependedOn, ServiceType, StartType
$jsonObj = @()
foreach($s in $serviceInfo) {
$Name = $s.Name
$DisplayName = $s.DisplayName
$Status = $s.Status
$DependentServices = $s.DependentServices
$ServicesDependedOn = $s.ServicesDependedOn
$ServiceType = $s.ServiceType
$StartType = $s.StartType
$jsonObj += @"
{"Name": "` + mark(`$Name`) + `", "DisplayName": "` + mark(`$DisplayName`) + `", "Status": "$Status", "DependentServices": "` + mark(`$DependentServices`) + `",
"ServicesDependedOn": "` + mark(`$ServicesDependedOn`) + `", "ServiceType": "$ServiceType", "StartType": "$StartType"}
"@
}
$result = $jsonObj -join ","
$result = "[" + $result + "]"
[Console]::WriteLine($result)
`
)

const (
	PowershellCmd = "powershell"
)

func randomString(length int) string {
	return uuid.NewV4().String()[:length]
}

func mark(s string) string {
	return startMarker + s + endMarker
}

// LogError is a wrapper on log.Error for easy testability
func LogError(log log.T, err error) {
	// To debug unit test, please uncomment following line
	// fmt.Println(err)
	log.Error(err)
}

var cmdExecutor = executeCommand

func executeCommand(command string, args ...string) ([]byte, error) {
	return exec.Command(command, args...).CombinedOutput()
}

// executePowershellCommands executes commands in Powershell to get all windows processes.
func executePowershellCommands(log log.T, command, args string) (output []byte, err error) {
	if output, err = cmdExecutor(PowershellCmd, command+" "+args); err != nil {
		log.Debugf("Failed to execute command : %v %v with error - %v",
			command,
			args,
			err.Error())
		log.Debugf("Command Stderr: %v", string(output))
		err = fmt.Errorf("Command failed with error: %v", string(output))
	}

	return
}

func collectDataFromPowershell(log log.T, powershellCommand string, serviceInfo *[]model.ServiceData) (err error) {
	var output []byte
	var cleanOutput string
	log.Infof("Executing command: %v", powershellCommand)
	output, err = executePowershellCommands(log, powershellCommand, "")
	if err != nil {
		log.Errorf("Error executing command - %v", err.Error())
		return
	}
	log.Debugf("Command output before clean up: %v", string(output))

	cleanOutput, err = pluginutil.ReplaceMarkedFields(pluginutil.CleanupNewLines(string(output)), startMarker, endMarker, pluginutil.CleanupJSONField)
	if err != nil {
		LogError(log, err)
		return
	}
	log.Debugf("Command output: %v", string(cleanOutput))

	if err = json.Unmarshal([]byte(cleanOutput), serviceInfo); err != nil {
		err = fmt.Errorf("Unable to parse command output - %v", err.Error())
		log.Error(err.Error())
		log.Infof("Error parsing command output - no data to return")
	}
	return
}

func collectServiceData(context context.T, config model.Config) (data []model.ServiceData, err error) {
	log := context.Log()
	log.Infof("collectServiceData called")
	err = collectDataFromPowershell(log, serviceInfoScript, &data)
	return
}
