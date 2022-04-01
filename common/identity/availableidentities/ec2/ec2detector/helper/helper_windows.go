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
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
)

const (
	commandExecTimeout = 10 * time.Second
	commandMaxRetry    = 3
	biosInfoCmd        = "Get-CimInstance -ClassName Win32_BIOS"
)

var execCommand = func(cmd string, params ...string) (string, error) {
	var err error
	var byteOutput []byte

	ctx, cancel := context.WithTimeout(context.Background(), commandExecTimeout)
	defer cancel()
	for i := 0; i < commandMaxRetry; i++ {
		byteOutput, err = exec.CommandContext(ctx, cmd, params...).Output()
		if err == nil {
			break
		}
	}
	return strings.TrimSpace(string(byteOutput)), err
}

func (d *detectorHelper) initCache() {
	d.cache = map[string]string{}

	args := append(strings.Split(appconfig.PowerShellCommandArgs, " "), biosInfoCmd)

	output, err := execCommand(appconfig.PowerShellPluginCommandName, args...)
	if err != nil {
		return
	}

	for _, biosLine := range strings.Split(output, "\r\n") {
		splitLine := strings.SplitN(biosLine, ":", 2)
		if len(splitLine) != 2 {
			continue
		}

		key := strings.TrimSpace(splitLine[0])
		value := strings.TrimSpace(splitLine[1])
		d.cache[key] = value
	}
}

func (d *detectorHelper) GetSystemInfo(attribute string) string {
	d.lock.Lock()
	defer d.lock.Unlock()

	if d.cache == nil {
		d.cache = map[string]string{}
	}

	var ok bool
	var result string
	if result, ok = d.cache[attribute]; !ok {
		// If attribute is not set in cache, reset cache and retrieve again
		d.initCache()
		result, _ = d.cache[attribute]
	}

	return result
}
