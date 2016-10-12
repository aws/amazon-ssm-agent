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

// Package application contains application gatherer.

// +build windows

package application

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/inventory/model"
)

const (
	PowershellCmd            = "powershell"
	ArgsFor32BitApplications = `Get-ItemProperty HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\* | Where-Object {$_.DisplayName -ne $null} | Select-Object @{Name="Name";Expression={$_."DisplayName"}}, @{Name="Version";Expression={$_."DisplayVersion"}}, Publisher, @{Name="InstalledTime";Expression={$_."InstallDate"}} | ConvertTo-Json`
	ArgsFor64BitApplications = `Get-ItemProperty HKLM:\Software\Wow6432Node\Microsoft\Windows\CurrentVersion\Uninstall\* | Where-Object {$_.DisplayName -ne $null} | Select-Object @{Name="Name";Expression={$_."DisplayName"}}, @{Name="Version";Expression={$_."DisplayVersion"}}, Publisher, @{Name="InstalledTime";Expression={$_."InstallDate"}} | ConvertTo-Json`

	Arch64Bit = "64-Bit"
	Arch32Bit = "32-Bit"
)

// decoupling exec.Command for easy testability
var cmdExecutor = executeCommand

func executeCommand(command string, args ...string) ([]byte, error) {
	return exec.Command(command, args...).CombinedOutput()
}

// CollectApplicationData collects application data for windows platform
func CollectApplicationData(context context.T) []inventory.ApplicationData {

	/*
		Note:

		We use powershell to query registry for a list of 64 bit windows apps using following command:

		Get-ItemProperty HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\* | Where-Object {$_.DisplayName -ne $null} | Select-Object @{Name="Name";Expression={$_."DisplayName"}} | ConvertTo-Json

		Similarly we use following command to query list of 32 bit windows apps:

		Get-ItemProperty HKLM:\Software\Wow6432Node\Microsoft\Windows\CurrentVersion\Uninstall\* | Where-Object {$_.DisplayName -ne $null} | Select-Object @{Name="Name";Expression={$_."DisplayName"}} | ConvertTo-Json

		We make use of Calculated property of Select-Object to format the data accordingly.

		For more details refer to: https://technet.microsoft.com/en-us/library/ff730948.aspx
	*/

	//TODO: Verify if HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\* contains 32 bit applications for 32 bit OS.
	//TODO: powershell commands can be put in a script to generate that data - and then we can simply execute the script to get the data.
	//it will enable us to run other complicated queries too.

	var data, apps []inventory.ApplicationData

	//getting all 64 bit applications
	apps = ExecutePowershellCommands(context, PowershellCmd, ArgsFor64BitApplications, Arch64Bit)
	data = append(data, apps...)

	//getting all 32 bit applications
	apps = ExecutePowershellCommands(context, PowershellCmd, ArgsFor32BitApplications, Arch32Bit)
	data = append(data, apps...)

	return data
}

// ExecutePowershellCommands executes commands in powershell to get all windows applications installed.
func ExecutePowershellCommands(context context.T, command, args, arch string) (data []inventory.ApplicationData) {

	var output []byte
	var err error
	log := context.Log()

	log.Infof("Getting all %v windows applications", arch)
	log.Infof("Executing command: %v %v", command, args)

	if output, err = cmdExecutor(command, args); err != nil {
		log.Debugf("Failed to execute command : %v %v with error - %v",
			command,
			args,
			err.Error())
		log.Debugf("Command Stderr: %v", string(output))
		err = fmt.Errorf("Command failed with error: %v", string(output))
		log.Error(err.Error())
		log.Infof("No application data to return")
	} else {
		cmdOutput := string(output)
		log.Debugf("Command output: %v", cmdOutput)

		if data, err = ConvertToApplicationData(cmdOutput, arch); err != nil {
			err = fmt.Errorf("Unable to convert query output to ApplicationData - %v", err.Error())
			log.Error(err.Error())
			log.Infof("No application data to return")
		} else {
			log.Infof("Number of applications detected by %v - %v", GathererName, len(data))

			str, _ := json.Marshal(data)
			log.Debugf("Gathered applications: %v", string(str))
		}
	}

	return
}

// ConvertToApplicationData converts powershell command output to an array of inventory.ApplicationData
func ConvertToApplicationData(cmdOutput, architecture string) (data []inventory.ApplicationData, err error) {
	//This implementation is closely tied to the kind of powershell command we run in windows. A change in command
	//MUST be accompanied with a change in json conversion logic as well.

	/*
			Sample powershell command that we run in windows to get applications information:

			Get-ItemProperty HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\* |
			Where-Object {$_.DisplayName -ne $null} |
			Select-Object @{Name="Name";Expression={$_."DisplayName"}},@{Name="Version";Expression={$_."DisplayVersion"}} |
			ConvertTo-Json

			Above command will generate data in json format:
			[
			    {
				"Name":  "EC2ConfigService",
				"Version":  "3.17.1032.0"
			    },
			    {
				"Name":  "aws-cfn-bootstrap",
				"Version":  "1.4.10"
			    },
			    {
				"Name":  "AWS PV Drivers",
				"Version":  "7.3.2"
			    }
			]

		        Since command output is in json - we do following operations:
		        - trim spaces
		        - unmarshal the string
		        - add architecture details as given input

	*/

	//trim spaces
	str := strings.TrimSpace(cmdOutput)

	//unmarshall json string & add architecture information
	if err = json.Unmarshal([]byte(str), &data); err == nil {

		//iterate over all entries and add default value of architecture as given input
		for i, v := range data {
			//set architecture to given input
			v.Architecture = architecture
			data[i] = v
		}
	}

	return
}
