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

//go:build freebsd || linux || netbsd || openbsd
// +build freebsd linux netbsd openbsd

package instancedetailedinformation

import (
	"bytes"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
)

const (
	lscpuCmd          = "lscpu"
	socketsKey        = "Socket(s)"
	coresPerSocketKey = "Core(s) per socket"
	threadsPerCoreKey = "Thread(s) per core"
	cpuModelNameKey   = "Model name"
	cpusKey           = "CPU(s)"
	cpuSpeedMHzKey    = "CPU MHz"
)

// cmdExecutor decouples exec.Command for easy testability
var cmdExecutor = executeCommand

// unixUname decouples unix.Uname for easy testability
var unixUname = executeUnixUname

func executeCommand(command string, args ...string) ([]byte, error) {
	return exec.Command(command, args...).CombinedOutput()
}

func executeUnixUname(uname *unix.Utsname) error {
	return unix.Uname(uname)
}

// collectPlatformDependentInstanceData collects data from the system.
func collectPlatformDependentInstanceData(context context.T) (appData []model.InstanceDetailedInformation) {
	log := context.Log()

	var instanceDetailedInformation model.InstanceDetailedInformation

	log.Infof("Executing command: %v", lscpuCmd)
	if output, err := cmdExecutor(lscpuCmd); err == nil {
		log.Infof("Parsing output %v", string(output))
		instanceDetailedInformation = parseLscpuOutput(string(output))
		log.Infof("Parsed output %v", instanceDetailedInformation)
	} else {
		log.Errorf("Failed to execute command : %v; error: %v", lscpuCmd, err.Error())
		log.Debugf("Command Stderr: %v", string(output))
		return
	}

	var uname unix.Utsname
	if err := unixUname(&uname); err == nil {
		lastChar := bytes.IndexByte(uname.Release[:], 0)
		instanceDetailedInformation.KernelVersion = string(uname.Release[:lastChar])
	} else {
		log.Errorf("Failed to gather kernel version %v", err.Error())
	}

	appData = append(appData, instanceDetailedInformation)
	return
}

// parseLscpuOutput collects relevant fields from lscpu output, which has the following format (some lines omitted):
//
//	CPU(s):                2
//	Thread(s) per core:    1
//	Core(s) per socket:    2
//	Socket(s):             1
//	Model name:            Intel(R) Xeon(R) CPU E5-2676 v3 @ 2.40GHz
//	CPU MHz:               2400.072
func parseLscpuOutput(output string) model.InstanceDetailedInformation {
	cpuSpeedMHzStr := getFieldValue(output, cpuSpeedMHzKey)
	if cpuSpeedMHzStr != "" {
		cpuSpeedMHzStr = strconv.Itoa(int(math.Trunc(parseFloat(cpuSpeedMHzStr, 0))))
	}

	socketsStr := getFieldValue(output, socketsKey)

	cpuCoresStr := ""
	coresPerSocketStr := getFieldValue(output, coresPerSocketKey)
	if socketsStr != "" && coresPerSocketStr != "" {
		sockets := parseInt(socketsStr, 0)
		coresPerSocket := parseInt(coresPerSocketStr, 0)
		cpuCoresStr = strconv.Itoa(sockets * coresPerSocket)
	}

	hyperThreadEnabledStr := ""
	threadsPerCoreStr := getFieldValue(output, threadsPerCoreKey)
	if threadsPerCoreStr != "" {
		hyperThreadEnabledStr = boolToStr(parseInt(threadsPerCoreStr, 0) > 1)
	}

	itemContent := model.InstanceDetailedInformation{
		CPUModel:              getFieldValue(output, cpuModelNameKey),
		CPUs:                  getFieldValue(output, cpusKey),
		CPUSpeedMHz:           cpuSpeedMHzStr,
		CPUSockets:            socketsStr,
		CPUCores:              cpuCoresStr,
		CPUHyperThreadEnabled: hyperThreadEnabledStr,
	}

	return itemContent
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

func parseInt(value string, defaultValue int) int {
	res, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return res
}

func parseFloat(value string, defaultValue float64) float64 {
	res, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return defaultValue
	}
	return res
}

func boolToStr(b bool) string {
	return fmt.Sprintf("%v", b)
}
