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

//go:build darwin
// +build darwin

package diagnosticsutil

import (
	"fmt"
	"os/user"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
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
		return fmt.Errorf("get-diagnostics needs to be executed by  %s", ExpectedServiceRunningUser)
	}
}

// AssumeAgentEnvironmentProxy is a noop on darwin because there is no other special proxy configuration
func AssumeAgentEnvironmentProxy() {
	proxyconfig.SetProxyConfig(log.NewSilentMockLog())
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
	return model.SSMAgentBinaryName, nil
}

func getAgentProcessPath() (string, error) {
	return getAgentFilePath()
}
