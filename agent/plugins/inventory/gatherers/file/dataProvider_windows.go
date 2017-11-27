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

	"path/filepath"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/twinj/uuid"
)

var (
	startMarker       = "<start" + randomString(8) + ">"
	endMarker         = "<end" + randomString(8) + ">"
	FileInfoBatchSize = 100
	fileInfoScript    = `
  [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
  function getjson($Paths){
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
			$ProductName = $p.VersionInfo.ProductName
			$ProductVersion = $p.VersionInfo.ProductVersion
			$ProductLanguage = $p.VersionInfo.Language
			$CompanyName = $p.VersionInfo.CompanyName
			$InstalledDir = $p.DirectoryName
			$ModTime = $p.LastWriteTime
			$jsonObj += @"
{"CompanyName": "` + mark(`$CompanyName`) + `", "ProductName": "` + mark(`$ProductName`) + `", "ProductVersion": "$ProductVersion", "ProductLanguage": "$ProductLanguage", "Name":"$Name", "Size":"$Length",
"Description":"` + mark(`$Description`) + `" ,"FileVersion":"$Version","InstalledDate":"$InstalledDate","LastAccessTime":"$LastAccessTime","InstalledDir":"` + mark(`$InstalledDir`) + `","ModificationTime":"$ModTime"}
"@
		}
		$result = $jsonObj -join ","
		$result = "[" + $result + "]"
		[Console]::WriteLine($result)
	} catch {
		Write-Error $_.Exception.Message
	}

}

getjson -Paths `
)

const (
	PowershellCmd  = "powershell"
	SleepTimeMs    = 5000
	ScriptFileName = "getFileInfo.ps1"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func randomString(length int) string {
	return uuid.NewV4().String()[:length]
}

func mark(s string) string {
	return startMarker + s + endMarker
}

var cmdExecutor = executeCommand
var writeFileText = writeFile

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
		log.Errorf("Failed to execute command : %v %v with error - %v",
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
	var output []byte
	var cleanOutput string
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

	if err = json.Unmarshal([]byte(cleanOutput), fileInfo); err != nil {
		err = fmt.Errorf("Unable to parse command output - %v", err.Error())
		log.Error(err.Error())
		log.Infof("Error parsing command output - no data to return")
	}
	return
}

func writeFile(path string, commands string) (err error) {
	err = fileutil.WriteAllText(path, commands)
	return
}

// Powershell has limit on number of parameters. So execute command using script.
func createScript(commands string, log log.T) (path string, err error) {
	var machineID string
	machineID, err = platform.InstanceID()

	if err != nil {
		log.Errorf("Error getting machineID")
		return
	}

	path = filepath.Join(appconfig.DefaultDataStorePath,
		machineID,
		appconfig.InventoryRootDirName,
		appconfig.FileInventoryRootDirName,
		ScriptFileName)
	log.Infof("Writing to script file %v", path)

	err = writeFileText(path, commands)
	if err != nil {
		log.Errorf(err.Error())
	}
	return
}

func getPowershellCmd(log log.T, paths []string) (cmd string, err error) {
	var transformed []string
	for _, x := range paths {
		transformed = append(transformed, `"`+x+`"`)
	}
	cmd = fileInfoScript + strings.Join(transformed, ",")
	return
}

//getMetaData creates powershell script for getting file metadata and executes the script
func getMetaDataForFiles(log log.T, paths []string) (fileInfo []model.FileData, err error) {
	var cmd string
	cmd, err = getPowershellCmd(log, paths)
	if err != nil {
		return
	}
	err = collectDataFromPowershell(log, cmd, &fileInfo)
	return

}

// Tries to create a powershell script and executes it
func createAndRunScript(log log.T, paths []string) (fileInfo []model.FileData, err error) {
	var cmd, path string
	cmd, err = getPowershellCmd(log, paths)
	if err != nil {
		log.Errorf(err.Error())
		return
	}
	path, err = createScript(cmd, log)
	if err != nil {
		log.Errorf(err.Error())
		return
	}

	powershellArg := "& '" + path + "'"
	log.Infof("Executing command %v", powershellArg)
	err = collectDataFromPowershell(log, powershellArg, &fileInfo)

	fileutil.DeleteFile(path)
	return
}

// Its is more efficient to run using script. So try to run command using script.
// If there is an error we should try fallback method.
func getMetaData(log log.T, paths []string) (fileInfo []model.FileData, err error) {
	var batchPaths []string

	var scriptErr error
	fileInfo, scriptErr = createAndRunScript(log, paths)

	// If err running the script, try fallback option
	if scriptErr != nil {
		for i := 0; i < len(paths); i += FileInfoBatchSize {
			batchPaths = paths[i:min(i+FileInfoBatchSize, len(paths))]
			fileInfoBatch, metaDataErr := getMetaDataForFiles(log, batchPaths)
			if metaDataErr != nil {
				log.Error(metaDataErr)
				err = metaDataErr
				return
			}
			fileInfo = append(fileInfo, fileInfoBatch...)
		}
		return
	}
	err = scriptErr
	return
}
