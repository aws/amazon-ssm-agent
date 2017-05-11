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
	"runtime"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
)

const (
	PowershellCmd                                        = "powershell"
	SysnativePowershellCmd                               = `C:\Windows\sysnative\WindowsPowerShell\v1.0\powershell.exe `
	ArgsToReadRegistryFromWindowsCurrentVersionUninstall = `Get-ItemProperty HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\* | Where-Object {$_.DisplayName -ne $null} | Select-Object @{n="Name";e={$_."DisplayName"}}, @{n="Version";e={$_."DisplayVersion"}}, Publisher, @{n="InstalledTime";e={[datetime]::ParseExact($_."InstallDate","yyyyMMdd",$null).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")}} | ConvertTo-Json `
	ArgsToReadRegistryFromWow6432Node                    = `Get-ItemProperty HKLM:\Software\Wow6432Node\Microsoft\Windows\CurrentVersion\Uninstall\* | Where-Object {$_.DisplayName -ne $null} | Select-Object @{n="Name";e={$_."DisplayName"}}, @{n="Version";e={$_."DisplayVersion"}}, Publisher, @{n="InstalledTime";e={[datetime]::ParseExact($_."InstallDate","yyyyMMdd",$null).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")}} | ConvertTo-Json`
	ArgsForDetectingOSArch                               = `get-wmiobject -class win32_processor | select-object addresswidth`
	KeywordFor64BitArchitectureReportedByPowershell      = "64"
	KeywordFor32BitArchitectureReportedByPowershell      = "32"
	Architecture64BitReportedByGoRuntime                 = "amd64"
)

// decoupling exec.Command for easy testability
var cmdExecutor = executeCommand

func executeCommand(command string, args ...string) ([]byte, error) {
	return exec.Command(command, args...).CombinedOutput()
}

// collectPlatformDependentApplicationData collects application data for windows platform
func collectPlatformDependentApplicationData(context context.T) []model.ApplicationData {
	/*
		Note:

		We get list of installed apps by using powershell to query registry from 2 locations:

		Path-1 => HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*
		Path-2 => HKLM:\Software\Wow6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*

		Path-2 is used to get a list of 32 bit apps running on a 64 bit OS (when 64bit agent is running on 64bit OS)
		For all other scenarios we use Path-1 to get the list of installed apps.
		Reference: https://msdn.microsoft.com/en-us/library/windows/desktop/ms724072(v=vs.85).aspx

		Powershell command format: Get-ItemProperty <REGISTRY PATH> | Where-Object {$_.DisplayName -ne $null} | Select-Object @{Name="Name";Expression={$_."DisplayName"}} | ConvertTo-Json

		We use calculated property of Select-Object to format the data accordingly. Reference: https://technet.microsoft.com/en-us/library/ff730948.aspx

		For determining the OS architecture we use the following command:

		get-wmiobject -class win32_processor | select-object addresswidth

		addresswidth - On a 32-bit operating system, the value is 32 and on a 64-bit operating system it is 64.

		Reference:
		https://msdn.microsoft.com/en-us/library/aa394373%28v=vs.85%29.aspx
	*/

	//TODO: powershell commands can be put in a script to generate that data - and then we can simply execute the script to get the data.
	//it will enable us to run other complicated queries too.

	var data, apps []model.ApplicationData

	log := context.Log()

	//detecting process architecture
	exeArch := runtime.GOARCH
	log.Infof("Exe architecture as detected by golang runtime - %v", exeArch)

	//detecting OS architecture
	osArch := detectOSArch(context, PowershellCmd, ArgsForDetectingOSArch)
	log.Infof("Detected OS architecture as - %v", osArch)

	if strings.Contains(osArch, KeywordFor32BitArchitectureReportedByPowershell) {
		//os architecture is 32 bit
		if exeArch != Architecture64BitReportedByGoRuntime {
			//exe architecture is also 32 bit
			//since both exe & os are 32 bit - we need to detect only 32 bit apps
			apps = executePowershellCommands(context, PowershellCmd, ArgsToReadRegistryFromWindowsCurrentVersionUninstall, model.Arch32Bit)
			data = append(data, apps...)
		} else {
			log.Infof("Detected an unsupported scenario of 64 bit amazon ssm agent running on 32 bit windows OS - nothing to report")
		}
	} else if strings.Contains(osArch, KeywordFor64BitArchitectureReportedByPowershell) {
		//os architecture is 64 bit
		if exeArch == Architecture64BitReportedByGoRuntime {
			//both exe & os architecture is 64 bit

			//detecting 32 bit apps by querying Wow6432Node path in registry
			apps = executePowershellCommands(context, PowershellCmd, ArgsToReadRegistryFromWow6432Node, model.Arch32Bit)
			data = append(data, apps...)

			//detecting 64 bit apps by querying normal registry path
			apps = executePowershellCommands(context, PowershellCmd, ArgsToReadRegistryFromWindowsCurrentVersionUninstall, model.Arch64Bit)
			data = append(data, apps...)
		} else {
			//exe architecture is 32 bit - all queries to registry path will be redirected to wow6432 so need to use sysnative
			//reference: https://blogs.msdn.microsoft.com/david.wang/2006/03/27/howto-detect-process-bitness/

			//detecting 32 bit apps by querying Wow632 registry node
			apps = executePowershellCommands(context, PowershellCmd, ArgsToReadRegistryFromWow6432Node, model.Arch32Bit)
			data = append(data, apps...)

			//detecting 64 bit apps by using sysnative for reading registry to avoid path redirection
			apps = executePowershellCommands(context, SysnativePowershellCmd, ArgsToReadRegistryFromWindowsCurrentVersionUninstall, model.Arch64Bit)
			data = append(data, apps...)
		}
	} else {
		log.Infof("Can't find application data because unable to detect OS architecture - nothing to report")
	}

	return data
}

// detectOSArch detects OS architecture
func detectOSArch(context context.T, command, args string) (osArch string) {
	var output []byte
	var err error
	log := context.Log()

	log.Infof("Getting OS architecture")
	log.Infof("Executing command: %v %v", command, args)

	if output, err = cmdExecutor(command, args); err != nil {
		log.Debugf("Failed to execute command : %v %v with error - %v",
			command,
			args,
			err.Error())
		log.Debugf("Command Stderr: %v", string(output))
		err = fmt.Errorf("Command failed with error: %v", string(output))
		log.Error(err.Error())
		log.Infof("Unable to detect OS architecture")
	} else {
		cmdOutput := string(output)
		log.Debugf("Command output: %v", cmdOutput)

		osArch = strings.TrimSpace(cmdOutput)
	}

	return
}

// executePowershellCommands executes commands in powershell to get all windows applications installed.
func executePowershellCommands(context context.T, command, args, arch string) (data []model.ApplicationData) {

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

		if data, err = convertToApplicationData(cmdOutput, arch); err != nil {
			err = fmt.Errorf("Unable to convert query output to ApplicationData - %v", err.Error())
			log.Error(err.Error())
			log.Infof("No application data to return")
		} else {
			log.Infof("Number of %v applications detected by %v - %v", arch, GathererName, len(data))

			str, _ := json.Marshal(data)
			log.Debugf("Gathered applications: %v", string(str))
		}
	}

	return
}

// convertToApplicationData converts powershell command output to an array of model.ApplicationData
func convertToApplicationData(cmdOutput, architecture string) (data []model.ApplicationData, err error) {
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
		for i, item := range data {
			//set architecture to given input
			item.Architecture = architecture
			item.CompType = componentType(item.Name)
			data[i] = item
		}
	}

	return
}
