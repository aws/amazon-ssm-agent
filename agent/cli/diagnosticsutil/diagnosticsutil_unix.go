// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package diagnosticsutil

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/proxyconfig"
	"github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider/model"
)

const (
	// ExpectedServiceRunningUser is the user we expect the agent to be running as
	ExpectedServiceRunningUser = "root"

	// newlineCharacter is the system specific newline character
	newlineCharacter = "\n"
)

// IsRunningElevatedPermissions checks if the ssm-cli is being executed as administrator
func IsRunningElevatedPermissions() error {
	currentUser, err := user.Current()
	if err != nil {
		return err
	}

	if currentUser.Username == ExpectedServiceRunningUser {
		return nil
	} else {
		return fmt.Errorf("get-diagnostics needs to be executed by %s", ExpectedServiceRunningUser)
	}
}

// AssumeAgentEnvironmentProxy reads the amazon-ssm-agent environment variables and assumes the same proxy settings
func AssumeAgentEnvironmentProxy() {
	pid, err := getRunningAgentPid()

	if err != nil {
		return
	}

	// Create proxy key map
	supportedProxyVars := map[string]bool{}
	for _, proxyKey := range proxyconfig.ProxyEnvVariables {
		supportedProxyVars[proxyKey] = true
	}

	// Read the environ file to extract all environment variables for process
	procEnvDataBytes, err := ioutil.ReadFile(path.Join("/proc", fmt.Sprintf("%v", pid), "environ"))
	procEnvDataBytes = bytes.TrimSpace(procEnvDataBytes)
	if err != nil || len(procEnvDataBytes) == 0 {
		// Failed to read or is empty environ file, skip process
		return
	}

	for _, procEnvBytes := range bytes.Split(procEnvDataBytes, []byte{0}) {
		procEnvString := strings.TrimSpace(string(procEnvBytes))
		if len(procEnvString) == 0 {
			// string is empty
			continue
		}

		procEnvSplit := strings.Split(procEnvString, "=")
		if len(procEnvSplit) < 2 {
			// if split at = does not create two indices, skip
			continue
		}

		if supportedProxyVars[procEnvSplit[0]] {
			os.Setenv(procEnvSplit[0], procEnvSplit[1])
		}
	}
}

func GetUserRunningAgentProcess() (string, error) {
	pid, err := getRunningAgentPid()
	if err != nil {
		return "", err
	}

	output, err := ExecuteCommandWithTimeout(2*time.Second, "ps", "-o", "user=", "-p", fmt.Sprint(pid))

	if err != nil {
		return "", fmt.Errorf("failed to query for user running pid %v: %s", pid, err)
	}

	return output, nil
}

func getAgentFilePath() (string, error) {
	// if service is running as snap and the installed ssm-cli is not being executed from /snap/amazon-ssm-agent/current/ssm-cli
	// we need to change the binary path by resolving the 'current' symlink to get the actual full path of the amazon-ssm-agent process
	ssmAgentBinaryPath := model.SSMAgentBinaryName
	var err error
	if IsAgentInstalledSnap() && !strings.HasPrefix(ssmAgentBinaryPath, "/snap/") {
		ssmAgentBinaryPath, err = filepath.EvalSymlinks("/snap/amazon-ssm-agent/current/amazon-ssm-agent")
		if err != nil {
			return "", err
		}
	}

	return ssmAgentBinaryPath, nil
}

func getAgentProcessPath() (string, error) {
	return getAgentFilePath()
}
