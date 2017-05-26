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
)

var ConvertGuidToCompressedGuidCmd = `function Convert-GuidToCompressedGuid {
						[CmdletBinding()]
						[OutputType()]
						param (
							[Parameter(ValueFromPipeline, ValueFromPipelineByPropertyName, Mandatory)]
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
var ArgsToReadRegistryFromProducts = `$products = Get-ItemProperty HKLM:\Software\Classes\Installer\Products\* | Select-Object  @{n="PSChildName";e={$_."PSChildName"}} |
				      Select -expand PSChildName

				     `
var ArgsToReadRegistryFromWindowsCurrentVersionUninstall = `Get-ItemProperty HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*  |
								Where-Object {($_.DisplayName -ne $null -and $_DisplayName -ne '' -and $_.DisplayName -notmatch '^KB[000000-999999]') -and
								       ($_.UninstallString -ne $null -and $_.UninstallString -ne '') -and
								       ($_.SystemComponent -eq $null -or ($_.SystemComponent -ne $null -and $_.SystemComponent -eq '0'))  -and
								       ($_.ParentKeyName -eq $null) -and
								       ($_.WindowsInstaller -eq $null -or ($_.WindowsInstaller -eq 1 -and $products -contains (Convert-GuidToCompressedGuid $_.PSChildName))) -and
								       ($_.ReleaseType -eq $null -or
										($_.ReleaseType -ne $null -and
										$_.ReleaseType -ne 'Security Update' -and
										$_.ReleaseType -ne 'Update Rollup' -and
										$_.ReleaseType -ne 'Hotfix'))
							        } |
							       Select-Object @{n="Name";e={$_."DisplayName"}},@{n="WindowsInstaller";e={$_."WindowsInstaller"}},
							       @{n="PSChildName";e={$_."PSChildName"}}, @{n="Version";e={$_."DisplayVersion"}}, Publisher,
							       @{n="InstalledTime";e={[datetime]::ParseExact($_."InstallDate","yyyyMMdd",$null).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")}} | ConvertTo-Json `
var ArgsToReadRegistryFromWow6432Node = `Get-ItemProperty HKLM:\Software\Wow6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*  |
					     Where-Object {($_.DisplayName -ne $null -and $_DisplayName -ne '' -and $_.DisplayName -notmatch '^KB[000000-999999]') -and
						     ($_.UninstallString -ne $null -and $_.UninstallString -ne '') -and
						     ($_.SystemComponent -eq $null -or ($_.SystemComponent -ne $null -and $_.SystemComponent -eq '0'))  -and
						     ($_.ParentKeyName -eq $null) -and
						     ($_.WindowsInstaller -eq $null -or ($_.WindowsInstaller -eq 1 -and $products -contains (Convert-GuidToCompressedGuid $_.PSChildName))) -and
						     ($_.ReleaseType -eq $null -or
							     ($_.ReleaseType -ne $null -and
							     $_.ReleaseType -ne 'Security Update' -and
							     $_.ReleaseType -ne 'Update Rollup' -and
							     $_.ReleaseType -ne 'Hotfix'))
              				     } |
              				     Select-Object @{n="Name";e={$_."DisplayName"}},@{n="WindowsInstaller";e={$_."WindowsInstaller"}},
              				     @{n="PSChildName";e={$_."PSChildName"}},
               				     @{n="Version";e={$_."DisplayVersion"}}, Publisher, @{n="InstalledTime";e={[datetime]::ParseExact($_."InstallDate","yyyyMMdd",$null).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")}} | ConvertTo-Json `

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

		When quering the registry, the following rules will be applied

		1. There must be a value within named DisplayName and it must have text in it. This is the name that will appear in Add/Remove Programs for this program -
		and yes you can have a bit of fun and change the value of this to anything you like and it will show up as that in Add/Remove Programs, as I have done in the screenshot :)
		2. There must be a value within named UninstallString and it must have text in it. This is the command line that Add/Remove Programs will execute when you attempt to uninstall this program.
		Knowing this can come in handy in certain situations.
		3. There must NOT be a value named SystemComponent that is set to 1. If the SystemComponent value does not exist or if it does exist but is set to 0 then that is fine,
		but if it is set to 1 then this program will not be added to the list. This is usually only set on programs that have been installed via a Windows Installer package (MSI). See below.
		4. There must NOT be a value named WindowsInstaller that is set to 1. Again if it is set to 0 or if it does not exist then that is fine.
		5. The subkey must not have a name that starts with KB and is followed by 6 numbers, e.g KB879032. If it has this name format then it will be classed as a Windows Update and will be added to
		the list of programs that only appear when you click Show Installed Updates.
		6. There must NOT be a value named ParentKeyName, as this indicates that this is an update to an existing program (and the text within the ParentKeyName value will indicate which program it is an update for)
		7. There must NOT be a value named ReleaseType set to any of the following: Security Update, Update Rollup, Hotfix. As again this indicates that it is an update rather than a full program.

		For rule 4, if the record has a windows installer value of 1, it will look at HKLM\Software\Classes\Installer\Products, we will convert its guid to a compressed version, and if we find a
		corresponding record in the Products, we will add it to the results too.

		For example, if the Guid id is {2BE0FA87-5B36-43CF-95C8-C68D6673FB94}, the compressed Guid will be {78AF0EB263B543CF8C5949BF3766D86C}

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
