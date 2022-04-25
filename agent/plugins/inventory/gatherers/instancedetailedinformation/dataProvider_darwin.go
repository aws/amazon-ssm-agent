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

package instancedetailedinformation

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
)

const (
	sysctlCmd        = "sysctl"
	cpuModelNameKey  = "machdep.cpu.brand_string"
	cpuCoreKey       = "hw.physicalcpu"
	cpusKey          = "hw.logicalcpu"
	cpuFreqKey       = "hw.cpufrequency"
	threadTypeKey    = "hw.cputhreadtype"
	kernelVersionKey = "kern.osrelease"
)

// cmdExecutor decouples exec.Command for easy testability
var cmdExecutor = executeCommand

func executeCommand(command string, args ...string) ([]byte, error) {
	return exec.Command(command, args...).CombinedOutput()
}

// collectPlatformDependentInstanceData collects data from the system.
func collectPlatformDependentInstanceData(context context.T) (appData []model.InstanceDetailedInformation) {
	log := context.Log()

	var output []byte
	var err error
	cmd := sysctlCmd
	args := []string{cpuModelNameKey, cpuCoreKey, cpusKey, cpuFreqKey, threadTypeKey}

	log.Infof("Executing command: %v", cmd)
	if output, err = cmdExecutor(cmd, args...); err != nil {
		log.Errorf("Failed to execute command : %v %v; with error: %v", cmd, args, err.Error())
		log.Debugf("Command Stderr: %v", string(output))
		return
	}

	log.Infof("Parsing output %v", string(output))
	r := parseSysctlOutput(string(output))
	log.Infof("Parsed output %v", r)
	return r
}

// parseSysctlOutput collects relevant fields from sysctl output, which has the following format
// machdep.cpu.brand_string: Intel(R) Core(TM) i7-8569U CPU @ 2.80GHz
// hw.physicalcpu: 4
// hw.logicalcpu: 8
// hw.cpufrequency: 2800000000
// hw.cputhreadtype: 1
func parseSysctlOutput(output string) (data []model.InstanceDetailedInformation) {
	var cpuSpeed = parseInt(getFieldValue(output, cpuFreqKey), 0)
	// convert the frequency to MHz
	var cpuSpeedMHzStr = strconv.Itoa(cpuSpeed / 1000000)
	var threadTypeVal = parseInt(getFieldValue(output, threadTypeKey), 0)
	var hyperThreadEnabledStr = boolToStr(threadTypeVal > 0)

	itemContent := model.InstanceDetailedInformation{
		CPUModel:              getFieldValue(output, cpuModelNameKey),
		CPUCores:              getFieldValue(output, cpuCoreKey),
		CPUs:                  getFieldValue(output, cpusKey),
		CPUSpeedMHz:           cpuSpeedMHzStr,
		CPUSockets:            "",
		CPUHyperThreadEnabled: hyperThreadEnabledStr,
		KernelVersion:         getFieldValue(output, kernelVersionKey),
	}

	data = append(data, itemContent)
	return
}

func parseInt(value string, defaultValue int) int {
	res, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return res
}

// getFieldValue looks for the first substring of the form "key: value \n" and returns the "value"
// if no such field found, returns empty string
func getFieldValue(input string, key string) string {
	keyStartPos := strings.Index(input, key+":")
	if keyStartPos < 0 {
		return ""
	}

	// add "\n" sentinel in case the key:value pair is on the last line and there is no newline at the end
	afterKey := input[keyStartPos+len(key)+1:] + "\n"
	valueEndPos := strings.Index(afterKey, "\n")
	return strings.TrimSpace(afterKey[:valueEndPos])
}

func boolToStr(b bool) string {
	return fmt.Sprintf("%v", b)
}
