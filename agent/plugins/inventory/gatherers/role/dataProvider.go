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
	"encoding/xml"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/twinj/uuid"
)

var (
	startMarker    = "<start" + randomString(8) + ">"
	endMarker      = "<end" + randomString(8) + ">"
	roleInfoScript = `
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
import-module ServerManager
$roleInfo = Get-WindowsFeature | Select-Object Name, DisplayName, Description, Installed, InstallState, FeatureType, Path, SubFeatures, ServerComponentDescriptor, DependsOn, Parent
$jsonObj = @()
foreach($r in $roleInfo) {
$Name = $r.Name
$DisplayName = $r.DisplayName
$Description = $r.Description
$Installed = $r.Installed
$InstalledState = $r.InstallState
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
[Console]::WriteLine($result)
`
	roleInfoScriptUsingRegistry = `
  [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
	$keyExists = Test-Path "Registry::HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Windows\CurrentVersion\Setup\OC Manager\Subcomponents"
	$jsonObj = @()
	if ($keyExists) {
		$key = Get-Item "Registry::HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Windows\CurrentVersion\Setup\OC Manager\Subcomponents"
		$valueNames = $key.GetValueNames();
		foreach ($valueName in $valueNames) {
			$value = $key.GetValue($valueName);
			if ($value -gt 0) {
				$installed = "True"
			} else {
				$installed = "False"
			}
			$jsonObj += @"
{"Name": "$valueName", "Installed": "$installed"}
"@

		}
	}
	$result = $jsonObj -join ","
	$result = "[" + $result + "]"
	[Console]::WriteLine($result)
`
)

const (
	// Use powershell to get role info
	PowershellCmd = "powershell"
	QueryFileName = "roleInfo.xml"
)

type RoleService struct {
	RoleService []RoleService
	DisplayName string `xml:"DisplayName,attr"`
	Installed   string `xml:"Installed,attr"`
	Id          string `xml:"Id,attr"`
	Default     string `xml:"Default,attr"`
}

type Role struct {
	RoleService []RoleService
	DisplayName string `xml:"DisplayName,attr"`
	Installed   string `xml:"Installed,attr"`
	Id          string `xml:"Id,attr"`
	Default     string `xml:"Default,attr"`
}

type Feature struct {
	Feature     []Feature
	DisplayName string `xml:"DisplayName,attr"`
	Installed   string `xml:"Installed,attr"`
	Id          string `xml:"Id,attr"`
	Default     string `xml:"Default,attr"`
}

type Result struct {
	Role    []Role
	Feature []Feature
}

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
var readFile = readAllText
var resultPath = getResultFilePath

func executeCommand(command string, args ...string) ([]byte, error) {
	return exec.Command(command, args...).CombinedOutput()
}

func readAllText(path string) (xmlData string, err error) {
	xmlData, err = fileutil.ReadAllText(path)
	return
}

func getResultFilePath(log log.T) (path string, err error) {
	var machineID string
	machineID, err = platform.InstanceID()

	if err != nil {
		log.Errorf("Error getting machineID")
		return
	}
	path = filepath.Join(appconfig.DefaultDataStorePath,
		machineID,
		appconfig.InventoryRootDirName,
		appconfig.RoleInventoryRootDirName,
		QueryFileName)
	return
}

func readServiceData(roleService RoleService, roleInfo *[]model.RoleData) {
	roleData := model.RoleData{
		Name:        roleService.Id,
		DisplayName: roleService.DisplayName,
		Installed:   strings.Title(roleService.Installed),
		FeatureType: "Role Service",
	}
	*roleInfo = append(*roleInfo, roleData)
	for i := 0; i < len(roleService.RoleService); i++ {
		readServiceData(roleService.RoleService[i], roleInfo)
	}
}

func readRoleData(role Role, roleInfo *[]model.RoleData) {
	roleData := model.RoleData{
		Name:        role.Id,
		DisplayName: role.DisplayName,
		Installed:   strings.Title(role.Installed),
		FeatureType: "Role",
	}
	*roleInfo = append(*roleInfo, roleData)

	for i := 0; i < len(role.RoleService); i++ {
		readServiceData(role.RoleService[i], roleInfo)
	}
}

func readFeatureData(feature Feature, roleInfo *[]model.RoleData) {

	roleData := model.RoleData{
		Name:        feature.Id,
		DisplayName: feature.DisplayName,
		Installed:   strings.Title(feature.Installed),
		FeatureType: "Feature",
	}
	*roleInfo = append(*roleInfo, roleData)

	for i := 0; i < len(feature.Feature); i++ {
		readFeatureData(feature.Feature[i], roleInfo)
	}
}

func readAllData(result Result, roleInfo *[]model.RoleData) {
	roles := result.Role
	features := result.Feature

	for i := 0; i < len(roles); i++ {
		readRoleData(roles[i], roleInfo)
	}

	for i := 0; i < len(features); i++ {
		readFeatureData(features[i], roleInfo)
	}
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

	cleanOutput, err = pluginutil.ReplaceMarkedFields(pluginutil.CleanupNewLines(string(output)), startMarker, endMarker, pluginutil.CleanupJSONField)
	if err != nil {
		LogError(log, err)
		return
	}
	log.Debugf("Command output: %v", string(cleanOutput))

	if err = json.Unmarshal([]byte(cleanOutput), roleInfo); err != nil {
		err = fmt.Errorf("Unable to parse command output - %v", err.Error())
		log.Error(err.Error())
		log.Infof("Error parsing command output - no data to return")
	}
	return
}

// Some early 2008 versions use ServerManager for role management, so use that for collecting data.
func collectDataUsingServerManager(log log.T, roleInfo *[]model.RoleData) (err error) {
	var xmlData, path string
	var output []byte

	path, err = resultPath(log)

	if err != nil {
		log.Errorf("Error getting path of file")
		return
	}

	powershellCommand := "Servermanagercmd.exe -q " + path
	output, err = executePowershellCommands(log, powershellCommand, "")
	log.Debugf("Command output: %v", string(output))

	if err != nil {
		log.Errorf("Error executing command - %v", err.Error())
		return
	}

	xmlData, err = readFile(path)
	if err != nil {
		log.Errorf("Error reading role info file - %v", err.Error())
		return
	}

	v := Result{}
	err = xml.Unmarshal([]byte(xmlData), &v)
	if err != nil {
		log.Errorf("Error unmarshalling xml: %v", err.Error())
		return
	}

	readAllData(v, roleInfo)
	fileutil.DeleteFile(path)
	return
}

func collectRoleData(context context.T, config model.Config) (data []model.RoleData, err error) {
	log := context.Log()
	log.Infof("collectRoleData called")

	err = collectDataFromPowershell(log, roleInfoScript, &data)
	// Some early 2008 releases uses server manager for getting role information
	if err != nil {
		log.Infof("Trying collecting role data using server manager")
		err = collectDataUsingServerManager(log, &data)
	}
	// In some versions of 2003, roles information is stored as subcomponents in registry.
	if err != nil {
		log.Infof("Trying collecting role data using registry")
		err = collectDataFromPowershell(log, roleInfoScriptUsingRegistry, &data)
	}
	if err != nil {
		log.Errorf("Failed to collect role data using possible mechanisms")
	}
	return
}
