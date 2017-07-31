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

package role

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
	startMarker    = "<start" + randomString(8) + ">"
	endMarker      = "<end" + randomString(8) + ">"
	roleInfoScript = `import-module ServerManager
$roleInfo = Get-WindowsFeature | Select-Object Name, DisplayName, Description, Installed, InstalledState, FeatureType, Path, SubFeatures, ServerComponentDescriptor, DependsOn, Parent
$jsonObj = @()
foreach($r in $roleInfo) {
$Name = $r.Name
$DisplayName = $r.DisplayName
$Description = $r.Description
$Installed = $r.Installed
$InstalledState = $r.InstalledState
$FeatureType = $r.FeatureType
$Path = $r.Path
$SubFeatures = $r.SubFeatures
$ServerComponentDescriptor = $r.ServerComponentDescriptor
$DependsOn = $r.DependsOn
$Parent = $r.Parent
$jsonObj += @"
{"Name": "` + mark(`$Name`) + `", "DisplayName": "` + mark(`$DisplayName`) + `", "Description": "` + mark(`$Description`) + `", "Installed": "$Installed",
"InstalledState": "$InstalledState", "FeatureType": "$FeatureType", "Path": "` + mark(`$Path`) + `", "SubFeatures": "` + mark(`$SubFeatures`) + `", "ServerComponentDescriptor": "` + mark(`$ServerComponentDescriptor`) + `", "DependsOn": "` + mark(`$DependsOn`) + `", "Parent": "` + mark(`$Parent`) + `"}
"@
}
$result = $jsonObj -join ","
$result = "[" + $result + "]"
Write-Output $result
`
)

const (
	// Use powershell to get role info
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

func collectDataFromPowershell(log log.T, powershellCommand string, roleInfo *[]model.RoleData) (err error) {
	log.Infof("Executing command: %v", powershellCommand)
	var output []byte
	var cleanOutput string
	output, err = executePowershellCommands(log, powershellCommand, "")
	if err != nil {
		log.Errorf("Error executing command - %v", err.Error())
		return
	}
	log.Debugf("Command output before clean up: %v", string(output))
	cleanOutput, err = pluginutil.ReplaceMarkedFields(string(output), startMarker, endMarker, pluginutil.CleanupJSONField)
	if err != nil {
		LogError(log, err)
		return
	}
	output = []byte(pluginutil.CleanupNewLines(cleanOutput))
	log.Debugf("Command output: %v", string(output))

	if err = json.Unmarshal([]byte(output), roleInfo); err != nil {
		err = fmt.Errorf("Unable to parse command output - %v", err.Error())
		log.Error(err.Error())
		log.Infof("Error parsing command output - no data to return")
	}
	return
}

func collectRoleData(context context.T, config model.Config) (data []model.RoleData, err error) {
	log := context.Log()
	log.Infof("collectRoleData called")
	err = collectDataFromPowershell(log, roleInfoScript, &data)
	return
}
