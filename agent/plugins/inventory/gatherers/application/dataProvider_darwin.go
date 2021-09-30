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

//go:build darwin
// +build darwin

// Package application contains application gatherer.
package application

import (
	"encoding/xml"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
)

var (
	systemProfilerCmd  = "system_profiler"
	xmlFormatArg       = "-xml"
	applicationDataArg = "SPApplicationsDataType"
	appNameKey         = "_name"
	publisherKey       = "obtained_from"
	architectureKey    = "runtime_environment"
	versionKey         = "version"
	// if wanted to collect the information of additional agent packages, add
	// the tag --pkgs="agent-package-name". We can add multiple --pkgs
	// options to the pkgutil command. And even we can do pattern match with
	// --pkgs options, following command will support (because of xargs)
	pkgutilCmd = fmt.Sprintf(`pkgutil --pkgs=%s | \
	                        xargs -n 1 pkgutil --pkg-info-plist | \
	                        grep -v DOCTYPE | \
	                        grep -v 'xml version="1.0" encoding="UTF-8"'`, amazonSsmAgentMac)
	packageNameKey    = "pkgid"
	packageVersionKey = "pkg-version"
	packageInsTimeKey = "install-time"
)

// decoupling exec.Command for easy testability
var cmdExecutor = executeCommand

func executeCommand(command string, args ...string) ([]byte, error) {
	return exec.Command(command, args...).CombinedOutput()
}

func platformInfoProvider(log log.T) (name string, err error) {
	return platform.PlatformName(log)
}

// collectPlatformDependentApplicationData collects all application data from the system using system_profiler command.
func collectPlatformDependentApplicationData(context context.T) (appData []model.ApplicationData) {

	var err error
	log := context.Log()

	cmd := systemProfilerCmd
	args := []string{xmlFormatArg, applicationDataArg}

	if appData, err = getApplicationData(context, cmd, args); err != nil {
		log.Info("system_profiler command failed!")
	}

	pkgData, err := getInstalledPackages(context, pkgutilCmd)
	if err == nil {
		var i int
		for i = 0; i < len(pkgData); i++ {
			appData = append(appData, pkgData[i])
		}
	} else {
		log.Info("pkgutil command failed!")
	}
	return
}

func getInstalledPackages(context context.T, command string) (data []model.ApplicationData, err error) {
	log := context.Log()
	var output []byte
	log.Debugf("Executing command: %v", command)
	if output, err = cmdExecutor("bash", "-c", command); err != nil {
		log.Errorf("Failed to execute command : %v with error - %v",
			command,
			err.Error())
		log.Debugf("Command Stderr: %v", string(output))
		err = fmt.Errorf("Command failed with error: %v", string(output))
	} else {
		cmdOutput := string(output)
		modifiedCmdOutput := "<WrapperXMLTag>" + cmdOutput + "</WrapperXMLTag>"

		if data, err = convertToApplicationDataFromInstalledPkg(modifiedCmdOutput); err != nil {
			err = fmt.Errorf("Unable to convert installed Packages to ApplicationData - %v", err.Error())
			log.Errorf(err.Error())
		} else {
			log.Infof("Number of packages detected - %v", len(data))
		}
	}
	return
}

func convertToApplicationDataFromInstalledPkg(input string) (data []model.ApplicationData, err error) {
	// Application type struct to hold the app xml string
	type Application struct {
		App string `xml:",innerxml"`
	}
	// Applications type struct to hold array of Application at path
	// plist>dict
	type Applications struct {
		Apps []Application `xml:"plist>dict"`
	}

	commandOutput := Applications{Apps: []Application{}}
	err = xml.Unmarshal([]byte(input), &commandOutput)
	if err != nil {
		return
	}

	// Loop over the Applications data, and create the output data
	var i int
	for i = 0; i < len(commandOutput.Apps); i++ {
		appName := getFieldValue(commandOutput.Apps[i].App, packageNameKey, "string")

		// Convert Unix timestamp to DateTime
		packageInstalledTime := getFieldValue(commandOutput.Apps[i].App, packageInsTimeKey, "integer")
		packageInstalledTimeToInteger, errParseInt := strconv.ParseInt(packageInstalledTime, 10, 64)
		var installedDateTime = ""
		if errParseInt == nil {
			tm := time.Unix(packageInstalledTimeToInteger, 0).UTC()
			installedDateTime = tm.Format(time.RFC3339)
		}

		itemContent := model.ApplicationData{
			Name:            appName,
			ApplicationType: "",
			Publisher:       "",
			Version:         getFieldValue(commandOutput.Apps[i].App, packageVersionKey, "string"),
			InstalledTime:   installedDateTime,
			Architecture:    "",
			URL:             "",
			Summary:         "",
			PackageId:       "",
			Release:         "",
			Epoch:           "",
			CompType:        componentType(appName),
		}
		data = append(data, itemContent)
	}
	return
}

// getApplicationData runs a terminal command and gets information about all packages/applications
func getApplicationData(context context.T, command string, args []string) (data []model.ApplicationData, err error) {
	var output []byte
	log := context.Log()

	log.Debugf("Executing command: %v %v", command, args)

	if output, err = cmdExecutor(command, args...); err != nil {
		log.Errorf("Failed to execute command : %v %v with error - %v",
			command,
			args,
			err.Error())
		log.Debugf("Command Stderr: %v", string(output))
		err = fmt.Errorf("Command failed with error: %v", string(output))
	} else {
		cmdOutput := string(output)
		log.Debugf("Command output: %v", cmdOutput)

		if data, err = convertToApplicationData(cmdOutput); err != nil {
			err = fmt.Errorf("Unable to convert query output to ApplicationData - %v", err.Error())
			log.Errorf(err.Error())
		} else {
			log.Infof("Number of applications detected - %v", len(data))
		}
	}
	return
}

// convert command output to XML (deserialize)
// Get the application data from xpath array>dict>array>dict
// parse the Application data, and the get the value for the respective keys
func convertToApplicationData(input string) (data []model.ApplicationData, err error) {
	/*
	   Sample Applications Data
	   <?xml version="1.0" encoding="UTF-8"?>
	   <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
	   <plist version="1.0">
	   <array>
	       <dict>
	           <key>_SPCommandLineArguments</key>
	           <array>
	               <string>/usr/sbin/system_profiler</string>
	               <string>-nospawn</string>
	               <string>-xml</string>
	               <string>SPApplicationsDataType</string>
	               <string>-detailLevel</string>
	               <string>full</string>
	           </array>
	           <key>_SPCompletionInterval</key>
	           <real>2.8108129501342773</real>
	           <key>_SPResponseTime</key>
	           <real>2.9170479774475098</real>
	           <key>_dataType</key>
	           <string>SPApplicationsDataType</string>
	           <key>_detailLevel</key>
	           <integer>1</integer>
	           <key>_items</key>
	           <array>
	               <dict>
	                   <key>_name</key>
	                   <string>Calendar</string>
	                   <key>has64BitIntelCode</key>
	                   <string>yes</string>
	                   <key>lastModified</key>
	                   <date>2019-04-03T07:20:22Z</date>
	                   <key>obtained_from</key>
	                   <string>apple</string>
	                   <key>path</key>
	                   <string>/Applications/Calendar.app</string>
	                   <key>runtime_environment</key>
	                   <string>arch_x86</string>
	                   <key>signed_by</key>
	                   <array>
	                       <string>Software Signing</string>
	                       <string>Apple Code Signing Certification Authority</string>
	                       <string>Apple Root CA</string>
	                   </array>
	                   <key>version</key>
	                   <string>11.0</string>
	               </dict>
	               <dict>
	                   <key>_name</key>
	                   <string>Amazon Chime</string>
	                   <key>has64BitIntelCode</key>
	                   <string>yes</string>
	                   <key>lastModified</key>
	                   <date>2020-02-06T22:52:21Z</date>
	                   <key>obtained_from</key>
	                   <string>identified_developer</string>
	                   <key>path</key>
	                   <string>/Applications/Amazon Chime.app</string>
	                   <key>runtime_environment</key>
	                   <string>arch_x86</string>
	                   <key>signed_by</key>
	                   <array>
	                       <string>Developer ID Application: AMZN Mobile LLC (94KV3E626L)</string>
	                       <string>Developer ID Certification Authority</string>
	                       <string>Apple Root CA</string>
	                   </array>
	                   <key>version</key>
	                   <string>4.28.7255</string>
	               </dict>
	           </array>
	       </dict>
	   </array>
	   </plist>
	*/
	// Application type struct to hold the app xml string
	type Application struct {
		App string `xml:",innerxml"`
	}
	// Applications type struct to hold array of Application at path array>dict>array>dict
	type Applications struct {
		Apps []Application `xml:"array>dict>array>dict"`
	}

	commandOutput := Applications{Apps: []Application{}}
	err = xml.Unmarshal([]byte(input), &commandOutput)
	if err != nil {
		return
	}

	// Loop over the Applications data, and create the output data
	var i int
	for i = 0; i < len(commandOutput.Apps); i++ {
		appName := getFieldValue(commandOutput.Apps[i].App, appNameKey, "string")
		itemContent := model.ApplicationData{
			Name:            appName,
			ApplicationType: "",
			Publisher:       getFieldValue(commandOutput.Apps[i].App, publisherKey, "string"),
			Version:         getFieldValue(commandOutput.Apps[i].App, versionKey, "string"),
			InstalledTime:   "",
			Architecture:    getFieldValue(commandOutput.Apps[i].App, architectureKey, "string"),
			URL:             "",
			Summary:         "",
			PackageId:       "",
			Release:         "",
			Epoch:           "",
			CompType:        componentType(appName),
		}
		data = append(data, itemContent)
	}
	return
}

// value of "key" is present as <key>key</string> in input string
// Next line of the above xml tag, <string>value</string> contains value
func getFieldValue(input string, key string, fieldValueTagName string) string {
	/*
	   input string format
	   <key>_name</key>
	   <string>Calendar</string>
	   <key>has64BitIntelCode</key>
	   <string>yes</string>
	   <key>lastModified</key>
	   <date>2019-04-03T07:20:22Z</date>
	   <key>obtained_from</key>
	   <string>apple</string>
	   <key>path</key>
	   <string>/Applications/Calendar.app</string>
	   <key>runtime_environment</key>
	   <string>arch_x86</string>
	   <key>signed_by</key>
	   <array>
	       <string>Software Signing</string>
	       <string>Apple Code Signing Certification Authority</string>
	       <string>Apple Root CA</string>
	   </array>
	   <key>version</key>
	   <string>11.0</string>
	*/
	var keyItem = "<key>" + key + "</key>"
	keyStartPos := strings.Index(input, keyItem)
	if keyStartPos < 0 {
		return ""
	}
	var valueStartTag = `<` + fieldValueTagName + `>`
	var valueEndTag = `</` + fieldValueTagName + `>`

	afterKeyStr := input[keyStartPos+len(keyItem):]

	nextTagStartPos := strings.Index(afterKeyStr, "<")
	nextStringTagPos := strings.Index(afterKeyStr, valueStartTag) // "<string")

	// verify next tag is string xml tag else return empty string
	if nextTagStartPos != nextStringTagPos {
		return ""
	}
	nextEndStringTagPos := strings.Index(afterKeyStr, valueEndTag) // "</string>")
	valueStartPos := nextStringTagPos + len(valueStartTag)         // "<string>")

	return strings.TrimSpace(afterKeyStr[valueStartPos:nextEndStringTagPos])
}
