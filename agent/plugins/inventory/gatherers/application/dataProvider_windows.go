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
	PowershellCmd                                   = "powershell"
	SysnativePowershellCmd                          = `C:\Windows\sysnative\WindowsPowerShell\v1.0\powershell.exe `
	ArgsForDetectingOSArch                          = `get-wmiobject -class win32_processor | select-object addresswidth`
	KeywordFor64BitArchitectureReportedByPowershell = "64"
	KeywordFor32BitArchitectureReportedByPowershell = "32"
	Architecture64BitReportedByGoRuntime            = "amd64"

	ConvertGuidToCompressedGuidCmd = `function Convert-GuidToCompressedGuid {
						[CmdletBinding()]
						[OutputType('System.String')]
						param (
							[Parameter(ValueFromPipeline="", ValueFromPipelineByPropertyName="", Mandatory=$true)]
							[string]$Guid
						)
						begin {
							$Guid = $Guid.Replace('-', '').Replace('{', '').Replace('}', '')
						}
						process {
							try {
								$Groups = @(
									$Guid.Substring(0, 8).ToCharArray(),
									$Guid.Substring(8, 4).ToCharArray(),
									$Guid.Substring(12, 4).ToCharArray(),
									$Guid.Substring(16, 16).ToCharArray()
								)
								$Groups[0..2] | foreach {
									[array]::Reverse($_)
								}
								$CompressedGuid = ($Groups[0..2] | foreach { $_ -join '' }) -join ''

								$chararr = $Groups[3]
								for ($i = 0; $i -lt $chararr.count; $i++) {
									if (($i % 2) -eq 0) {
										$CompressedGuid += ($chararr[$i+1] + $chararr[$i]) -join ''
									}
								}
								$CompressedGuid
							} catch {
								Write-Error $_.Exception.Message
							}
						}
					}

				     `
	ArgsToReadRegistryFromProducts = `$products = Get-ItemProperty HKLM:\Software\Classes\Installer\Products\* | Select-Object  @{n="PSChildName";e={$_."PSChildName"}} |
				      Select -expand PSChildName

				     `
	RegistryPathCurrentVersionUninstall            = `HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*`
	RegistryPathWow6432NodeCurrentVersionUninstall = `HKLM:\Software\Wow6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*`
	ArgsToReadRegistryApplications                 = `
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
Get-ItemProperty %v |
Where-Object {($_.DisplayName -ne $null -and $_DisplayName -ne '' -and $_.DisplayName -notmatch '^KB[000000-999999]') -and
	($_.UninstallString -ne $null -and $_.UninstallString -ne '') -and
	($_.SystemComponent -eq $null -or ($_.SystemComponent -ne $null -and $_.SystemComponent -eq '0'))  -and
	($_.ParentKeyName -eq $null) -and
	($_.WindowsInstaller -eq $null -or ($_.WindowsInstaller -eq 1 -and $products -contains (Convert-GuidToCompressedGuid $_.PSChildName))) -and
	($_.ReleaseType -eq $null -or ($_.ReleaseType -ne $null -and
		$_.ReleaseType -ne 'Security Update' -and
		$_.ReleaseType -ne 'Update Rollup' -and
		$_.ReleaseType -ne 'Hotfix'))
} |
Select-Object @{n="Name";e={$_."DisplayName"}},
	@{n="PackageId";e={$_."PSChildName"}}, @{n="Version";e={$_."DisplayVersion"}}, Publisher,
	@{n="InstalledTime";e={[datetime]::ParseExact($_."InstallDate","yyyyMMdd",$null).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")}} | %% { [Console]::WriteLine(@"
{"Name":"$($_.Name)","PackageId":"$($_.PackageId)","Version":"$($_.Version)","Publisher":"$($_.Publisher)","InstalledTime":"$($_.InstalledTime)"},
"@)} `
)

var ArgsToReadRegistryFromWindowsCurrentVersionUninstall = fmt.Sprintf(ArgsToReadRegistryApplications, RegistryPathCurrentVersionUninstall)
var ArgsToReadRegistryFromWow6432Node = fmt.Sprintf(ArgsToReadRegistryApplications, RegistryPathWow6432NodeCurrentVersionUninstall)

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

		We use following rules to detect applications from Registry. This is done to ensure AWS:Application’s behavior is similar to Add/Remove programs in Windows:
		Records that meets all rules are added in the result set:

		1. ‘DisplayName’  must be present with a valid value, as this is reflected as Display Name in AWS:Application inventory type. Also, its value must not start with ‘KB’ followed by 6 numbers - as that indicates a Windows update.
		2. ‘UninstallString’ must be present, because it stores the command line that gets executed by Add/Remove programs, when user tries to uninstall a program.
		3. ‘SystemComponent’ must be either absent or present with value set to 0, because this value is usually set on programs that have been installed via a Windows Installer Package (MSI).
		4. ‘ParentKeyName’ must NOT be present, because that indicates its an update to the parent program.
		5. ‘ReleaseType’ must either be absent or if present must not have value set to ‘Security Update’, ‘Update Rollup’, ‘Hotfix’, because that indicates its an update to an existing program.
		6. ‘WindowsInstaller’ must be either absent or present with value 0. If the value is set to 1, then the application is included in the list if and only if the corresponding compressed guid (explained below) is also present in HKLM:\Software\Classes\Installer\Products\

		Calculation of compressed guid:
		Each Guid has 5 parts separated by '-'. For the first three each one will be totally reversed, and for the remaining two each one will be reversed by every other character.
		Then the final compressed Guid will be constructed by concatinating all the reversed parts without '-'.

		-Example-

		Input  : 		2BE0FA87-5B36-43CF-95C8-C68D6673FB94
		Reversed :		78AF0EB2-63B5-FC34-598C-6CD86637BF49
		Final Compressed Guid:  78AF0EB263B5FC34598C6CD86637BF49

		Reference:
		https://community.spiceworks.com/how_to/2238-how-add-remove-programs-works
	*/

	//it will enable us to run other complicated queries too.

	var data, apps []model.ApplicationData
	var cmd string

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
			cmd = ConvertGuidToCompressedGuidCmd + ArgsToReadRegistryFromProducts + ArgsToReadRegistryFromWindowsCurrentVersionUninstall
			apps = executePowershellCommands(context, PowershellCmd, cmd, model.Arch32Bit)
			data = append(data, apps...)
		} else {
			log.Error("Detected an unsupported scenario of 64 bit amazon ssm agent running on 32 bit windows OS - nothing to report")
		}
	} else if strings.Contains(osArch, KeywordFor64BitArchitectureReportedByPowershell) {
		//os architecture is 64 bit
		if exeArch == Architecture64BitReportedByGoRuntime {
			//both exe & os architecture is 64 bit

			//detecting 32 bit apps by querying Wow6432Node path in registry
			cmd = ConvertGuidToCompressedGuidCmd + ArgsToReadRegistryFromProducts + ArgsToReadRegistryFromWow6432Node
			apps = executePowershellCommands(context, PowershellCmd, cmd, model.Arch32Bit)
			data = append(data, apps...)

			//detecting 64 bit apps by querying normal registry path
			cmd = ConvertGuidToCompressedGuidCmd + ArgsToReadRegistryFromProducts + ArgsToReadRegistryFromWindowsCurrentVersionUninstall
			apps = executePowershellCommands(context, PowershellCmd, cmd, model.Arch64Bit)
			data = append(data, apps...)
		} else {
			//exe architecture is 32 bit - all queries to registry path will be redirected to wow6432 so need to use sysnative
			//reference: https://blogs.msdn.microsoft.com/david.wang/2006/03/27/howto-detect-process-bitness/

			//detecting 32 bit apps by querying Wow632 registry node
			cmd = ConvertGuidToCompressedGuidCmd + ArgsToReadRegistryFromProducts + ArgsToReadRegistryFromWow6432Node
			apps = executePowershellCommands(context, PowershellCmd, cmd, model.Arch32Bit)
			data = append(data, apps...)

			//detecting 64 bit apps by using sysnative for reading registry to avoid path redirection
			cmd = ConvertGuidToCompressedGuidCmd + ArgsToReadRegistryFromProducts + ArgsToReadRegistryFromWindowsCurrentVersionUninstall
			apps = executePowershellCommands(context, SysnativePowershellCmd, cmd, model.Arch64Bit)
			data = append(data, apps...)
		}
	} else {
		log.Error("Can't find application data because unable to detect OS architecture - nothing to report")
	}

	return data
}

// detectOSArch detects OS architecture; decouple for unit test
var detectOSArch = detectOSArchFun

func detectOSArchFun(context context.T, command, args string) (osArch string) {
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
		// Clean all Ctrl code from UTF-8 string
		cmdOutput := stripCtlFromUTF8(string(output))
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
			The powershell command that we run in windows to get applications information
			will generate data in the following format:
			    { "Name":  "EC2ConfigService", "Version":  "3.17.1032.0" },
			    { "Name":  "aws-cfn-bootstrap", "Version":  "1.4.10" },
			    { "Name":  "AWS PV Drivers", "Version":  "7.3.2" },

		        We do the following operations:
		        - convert the string to a json array string
		        - unmarshal the string
		        - add architecture details as given input
	*/

	str := convertEntriesToJsonArray(cmdOutput)
	// remove newlines because powershell 2.0 sometimes inserts newlines every 80 characters or so
	str = cleanupNewLines(str)

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
