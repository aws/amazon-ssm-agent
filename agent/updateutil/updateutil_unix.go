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

//go:build darwin || freebsd || linux || netbsd || openbsd
// +build darwin freebsd linux netbsd openbsd

// Package updateutil contains updater specific utilities.
package updateutil

import (
	"os/exec"
	"strings"
	"syscall"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/versionutil"
)

const (
	legacyUpdaterArtifactsRoot   = "/var/log/amazon/ssm/update/"
	firstAgentWithNewUpdaterPath = "1.1.86.0"
)

func prepareProcess(command *exec.Cmd) {
	// make the process the leader of its process group
	// (otherwise we cannot kill it properly)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func isAgentServiceRunning(log log.T) (bool, error) {
	serviceStatus, err := execCommand("status", "amazon-ssm-agent").Output()
	if err != nil {
		return false, err
	}

	agentStatus := strings.TrimSpace(string(serviceStatus))
	return strings.Contains(agentStatus, "amazon-ssm-agent start/running"), nil
}

// UpdateInstallDelayer delays the agent install when domain join reboot doc found
func (util *Utility) UpdateInstallDelayer(ctx context.T, updateRoot string) error {
	return nil
}

// LoadUpdateDocumentState loads the update document state from Pending queue
func (util *Utility) LoadUpdateDocumentState(ctx context.T, commandId string) error {
	return nil
}

func setPlatformSpecificCommand(parts []string) []string {
	return parts
}

// ResolveUpdateRoot returns the platform specific path to update artifacts
func ResolveUpdateRoot(sourceVersion string) (string, error) {
	compareResult, err := versionutil.VersionCompare(sourceVersion, firstAgentWithNewUpdaterPath)
	if err != nil {
		return "", err
	}
	// New versions that with new binary locations
	if compareResult >= 0 {
		return appconfig.UpdaterArtifactsRoot, nil
	}

	return legacyUpdaterArtifactsRoot, nil
}
