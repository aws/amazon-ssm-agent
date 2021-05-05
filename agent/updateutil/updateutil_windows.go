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

// Package updateutil contains updater specific utilities.
package updateutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

var getPlatformSku = platform.PlatformSku

func prepareProcess(command *exec.Cmd) {
}

func isAgentServiceRunning(log log.T) (bool, error) {
	serviceName := "AmazonSSMAgent"
	expectedState := svc.Running

	manager, err := mgr.Connect()
	if err != nil {
		log.Warnf("Cannot connect to service manager: %v", err)
		return false, err
	}
	defer manager.Disconnect()

	service, err := manager.OpenService(serviceName)
	if err != nil {
		log.Warnf("Cannot open agent service: %v", err)
		return false, err
	}
	defer service.Close()

	serviceStatus, err := service.Query()
	if err != nil {
		log.Warnf("Cannot query agent service: %v", err)
		return false, err
	}

	return serviceStatus.State == expectedState, err
}

func setPlatformSpecificCommand(parts []string) []string {
	cmd := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe") + " -ExecutionPolicy unrestricted"
	return append(strings.Split(cmd, " "), parts...)
}
