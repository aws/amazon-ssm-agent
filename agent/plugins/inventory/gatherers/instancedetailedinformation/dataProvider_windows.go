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

package instancedetailedinformation

import (
	"encoding/json"
	"fmt"
	"os/exec"

	"strings"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
)

const (
	PowershellCmd = "powershell"
	CPUInfoScript = `
$wmi_proc = Get-WmiObject -Class Win32_Processor
if (@($wmi_proc)[0].NumberOfCores) #Modern OS
{
    $Sockets = @($wmi_proc).Count
    $Cores = ($wmi_proc | Measure-Object -Property NumberOfCores -Sum).Sum
    $CPUs = ($wmi_proc | Measure-Object -Property NumberOfLogicalProcessors -Sum).Sum

}
else #Legacy OS
{
    $Sockets = @($wmi_proc | Select-Object -Property SocketDesignation -Unique).Count
    $Cores = @($wmi_proc).Count
    $CPUs=$Cores
}
$CPUModel=@($wmi_proc)[0].Name
$CPUSpeed=@($wmi_proc)[0].MaxClockSpeed
if ($Cores -lt $CPUs) {
    $Hyperthread="true"
} else {
    $Hyperthread="false"
}
Write-Host -nonewline @"
{"CPUModel":"$CPUModel","CPUSpeedMHz":"$CPUSpeed","CPUs":"$CPUs","CPUSockets":"$Sockets","CPUCores":"$Cores","CPUHyperThreadEnabled":"$HyperThread"}
"@`
	OsInfoScript = `GET-WMIOBJECT -class win32_operatingsystem |
SELECT-OBJECT ServicePackMajorVersion,BuildNumber | % { Write-Output @"
{"OSServicePack":"$($_.ServicePackMajorVersion)"}
"@}`
)

// decoupling exec.Command for easy testability
var cmdExecutor = executeCommand

func executeCommand(command string, args ...string) ([]byte, error) {
	return exec.Command(command, args...).CombinedOutput()
}

// collectPlatformDependentInstanceData collects data from the system.
func collectPlatformDependentInstanceData(context context.T) (appData []model.InstanceDetailedInformation) {
	log := context.Log()
	log.Infof("Getting %v data", GathererName)
	var instanceDetailedInfo model.InstanceDetailedInformation
	err1 := collectDataFromPowershell(context, CPUInfoScript, &instanceDetailedInfo)
	err2 := collectDataFromPowershell(context, OsInfoScript, &instanceDetailedInfo)
	if err1 != nil && err2 != nil {
		// if both commands fail, return no data
		return
	}
	appData = append(appData, instanceDetailedInfo)
	str, _ := json.Marshal(appData)
	log.Debugf("%v gathered: %v", GathererName, string(str))
	return
}

func collectDataFromPowershell(context context.T, powershellCommand string, instanceDetailedInfoResult *model.InstanceDetailedInformation) (err error) {
	log := context.Log()
	log.Infof("Executing command: %v", powershellCommand)
	output, err := executePowershellCommands(context, powershellCommand, "")
	if err != nil {
		log.Errorf("Error executing command - %v", err.Error())
		return
	}
	output = []byte(cleanupNewLines(string(output)))
	log.Infof("Command output: %v", string(output))

	if err = json.Unmarshal([]byte(output), instanceDetailedInfoResult); err != nil {
		err = fmt.Errorf("Unable to parse command output - %v", err.Error())
		log.Error(err.Error())
		log.Infof("Error parsing command output - no data to return")
	}
	return
}

func cleanupNewLines(s string) string {
	return strings.Replace(strings.Replace(s, "\n", "", -1), "\r", "", -1)
}

// executePowershellCommands executes commands in powershell to get all  applications installed.
func executePowershellCommands(context context.T, command, args string) (output []byte, err error) {
	log := context.Log()
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
