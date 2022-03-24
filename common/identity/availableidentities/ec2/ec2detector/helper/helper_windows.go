// Copyright 2022 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package helper

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
)

const (
	commandExecTimeout = 5 * time.Second
)

var execCommand = func(cmd string, params ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandExecTimeout)
	defer cancel()
	b, e := exec.CommandContext(ctx, cmd, params...).Output()

	if e != nil {
		return "", e
	}

	return strings.TrimSpace(string(b)), e
}

func (*detectorHelper) GetSystemInfo(attribute string) string {
	wmicCommand := filepath.Join(appconfig.EnvWinDir, "System32", "wbem", "wmic.exe")
	output, err := execCommand(wmicCommand, "path", "win32_computersystemproduct", "get", attribute)
	if err != nil {
		return ""
	}

	data := strings.Split(output, "\r\n")
	if len(data) > 1 {
		return strings.TrimSpace(data[1])
	} else if len(data) == 1 {
		data = strings.Split(data[0], " = ")
		if len(data) > 1 {
			return strings.TrimSpace(data[1])
		}
	}

	return ""
}
