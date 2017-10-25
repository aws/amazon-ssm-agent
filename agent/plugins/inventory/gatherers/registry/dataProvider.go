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

package registry

import (
	"encoding/json"
	"errors"
	"fmt"

	"os/exec"

	"path/filepath"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/twinj/uuid"
)

const (
	PowershellCmd      = "powershell"
	MaxValueCountLimit = 250
	ValueLimitExceeded = "ValueLimitExceeded"
)

type filterObj struct {
	Path       string
	Recursive  bool
	ValueNames []string
}

var ValueCountLimitExceeded = errors.New("Exceeded register value count limit")

// LogError is a wrapper on log.Error for easy testability
func LogError(log log.T, err error) {
	// To debug unit test, please uncomment following line
	// fmt.Println(err)
	log.Error(err)
}

func randomString(length int) string {
	return uuid.NewV4().String()[:length]
}

var cmdExecutor = executeCommand

func executeCommand(command string, args ...string) ([]byte, error) {
	return exec.Command(command, args...).CombinedOutput()
}

// executePowershellCommands executes commands in Powershell to get registry values.
func executePowershellCommands(log log.T, command, args string) (output []byte, err error) {
	if output, err = cmdExecutor(PowershellCmd, command+" "+args); err != nil {
		log.Errorf("Failed to execute command : %v %v with error - %v",
			command,
			args,
			err.Error())
		log.Debugf("Command Stderr: %v", string(output))
		err = fmt.Errorf("Command failed with error: %v", string(output))
	}

	return
}

func collectDataFromPowershell(log log.T, powershellCommand string, registryInfo *[]model.RegistryData) (err error) {
	log.Infof("Executing command: %v", powershellCommand)
	var output []byte
	var cleanOutput string
	output, err = executePowershellCommands(log, powershellCommand, "")
	if err != nil {
		log.Errorf("Error executing command - %v", err.Error())
		return
	}
	log.Debugf("Before cleanup %v", string(output))
	cleanOutput, err = pluginutil.ReplaceMarkedFields(pluginutil.CleanupNewLines(string(output)), startMarker, endMarker, pluginutil.CleanupJSONField)
	if err != nil {
		LogError(log, err)
		return
	}

	log.Debugf("Command output: %v", string(cleanOutput))
	if cleanOutput == ValueLimitExceeded {
		log.Error("Number of values collected exceeded limit")
		err = ValueCountLimitExceeded
		return
	}
	if err = json.Unmarshal([]byte(cleanOutput), registryInfo); err != nil {
		err = fmt.Errorf("Unable to parse command output - %v", err.Error())
		log.Error(err.Error())
		log.Infof("Error parsing command output - no data to return")
	}
	return
}

func collectRegistryData(context context.T, config model.Config) (data []model.RegistryData, err error) {
	log := context.Log()
	log.Infof("collectRegistryData called")
	config.Filters = strings.Replace(config.Filters, `\`, `/`, -1)
	var filterList []filterObj
	if err = json.Unmarshal([]byte(config.Filters), &filterList); err != nil {
		return
	}

	valueScanLimit := MaxValueCountLimit
	for _, filter := range filterList {
		var temp []model.RegistryData

		path := filepath.FromSlash(filter.Path)
		recursive := filter.Recursive
		valueNames := filter.ValueNames
		log.Infof("valueNames %v", valueNames)
		registryPath := "Registry::" + path
		execScript := registryInfoScript + "-Path \"" + registryPath + "\" -ValueLimit " + fmt.Sprint(valueScanLimit)
		if recursive == true {
			execScript += " -Recursive"
		}
		if valueNames != nil && len(valueNames) > 0 {
			valueNamesArg := strings.Join(valueNames, ",")
			execScript += " -Values " + valueNamesArg
		}

		if getRegistryErr := collectDataFromPowershell(log, execScript, &temp); getRegistryErr != nil {
			LogError(log, getRegistryErr)
			if getRegistryErr == ValueCountLimitExceeded {
				err = getRegistryErr
				return
			}
			continue
		}

		data = append(data, temp...)
		valueScanLimit = MaxValueCountLimit - len(data)
	}
	log.Infof("Collected %d registry entries", len(data))
	return
}
