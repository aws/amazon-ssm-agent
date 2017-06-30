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

// Package file contains file gatherer.
package file

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/twinj/uuid"
)

var (
	startMarker    = "<start" + randomString(8) + ">"
	endMarker      = "<end" + randomString(8) + ">"
	fileInfoScript = `function getjson($Paths){
	try {
		$a = Get-ItemProperty -Path $Paths -EA SilentlyContinue |
		SELECT-OBJECT Name,Length,VersionInfo,@{n="LastWriteTime";e={[datetime]::ParseExact($_."LastWriteTime","MM/dd/yyyy HH:mm:ss",$null).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")}},
		@{n="CreationTime";e={[datetime]::ParseExact($_."CreationTime","MM/dd/yyyy HH:mm:ss",$null).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")}},
		@{n="LastAccessTime";e={[datetime]::ParseExact($_."LastAccessTime","MM/dd/yyyy HH:mm:ss",$null).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")}},DirectoryName
		$jsonObj = @()
		foreach ($p in $a) {
			$Name = $p.Name
			$Length = $p.Length
			$Description = $p.VersionInfo.FileDescription
			$Version = $p.VersionInfo.FileVersion
			$InstalledDate = $p.CreationTime
			$LastAccesstime = $p.LastAccessTime
			$ProductName = $p.VersionInfo.Product
			$ProductVersion = $p.VersionInfo.ProductVersion
			$ProductLanguage = $p.VersionInfo.Language
			$CompanyName = $p.VersionInfo.CompanyName
			$InstalledDir = $p.DirectoryName
			$ModtTime = $p.LastWriteTime
			$jsonObj += @"
{"CompanyName": "$CompanyName", "ProductName": "$ProductName", "ProductVersion": "$ProductVersion", "ProductLanguage": "$ProductLanguage", "Name":"$Name", "Size":"$Length",
"Description":"` + mark(`$Description`) + `" ,"FileVersion":"$Version","InstalledDate":"$InstalledDate","LastAccessTime":"$LastAccessTime","InstalledDir":"` + mark(`$InstalledDir`) + `","ModificationTime":"$ModTime"}
"@
		}
		$result = $jsonObj -join ","
		$result = "[" + $result + "]"
		Write-Output $result
	} catch {
		Write-Error $_.Exception.Message
	}

}

getjson -Paths `
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

var cmdExecutor = executeCommand

func executeCommand(command string, args ...string) ([]byte, error) {
	return exec.Command(command, args...).CombinedOutput()
}

//expand function expands windows environment variables
func expand(s string, mapping func(string) string) (newStr string, err error) {
	newStr, err = pluginutil.ReplaceMarkedFields(s, "%", "%", mapping)
	if err != nil {
		return "", err
	}
	return
}

// executePowershellCommands executes commands in Powershell to get all windows files installed.
func executePowershellCommands(log log.T, command, args string) (output []byte, err error) {
	if output, err = cmdExecutor(PowershellCmd, command+" "+args); err != nil {
		log.Infof("Failed to execute command : %v %v with error - %v",
			command,
			args,
			err.Error())
		log.Debugf("Failed to execute command : %v %v with error - %v",
			command,
			args,
			err.Error())
		log.Debugf("Command Stderr: %v", string(output))
		err = fmt.Errorf("Command failed with error: %v", string(output))
	}

	return
}

func collectDataFromPowershell(log log.T, powershellCommand string, fileInfo *[]model.FileData) (err error) {
	log.Infof("Executing command: %v", powershellCommand)
	output, err := executePowershellCommands(log, powershellCommand, "")
	if err != nil {
		log.Errorf("Error executing command - %v", err.Error())
		return
	}
	log.Infof("Command output before clean up: %v", string(output))
	cleanOutput, err := pluginutil.ReplaceMarkedFields(string(output), startMarker, endMarker, pluginutil.CleanupJSONField)
	if err != nil {
		LogError(log, err)
		return
	}
	output = []byte(pluginutil.CleanupNewLines(cleanOutput))
	log.Infof("Command output: %v", string(output))

	if err = json.Unmarshal([]byte(output), fileInfo); err != nil {
		err = fmt.Errorf("Unable to parse command output - %v", err.Error())
		log.Error(err.Error())
		log.Infof("Error parsing command output - no data to return")
	}
	return
}

//getMetaData creates powershell script for getting file metadata and executes the script
func getMetaData(log log.T, paths []string) (fileInfo []model.FileData, err error) {
	var transformed []string
	for _, x := range paths {
		transformed = append(transformed, `"`+x+`"`)
	}
	cmd := fileInfoScript + strings.Join(transformed, ",")
	if err != nil {
		return
	}
	err = collectDataFromPowershell(log, cmd, &fileInfo)
	return

}
