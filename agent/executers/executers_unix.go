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

// +build darwin freebsd linux netbsd openbsd

package executers

import (
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
)

func prepareProcess(command *exec.Cmd) {
	// make the process the leader of its process group
	// (otherwise we cannot kill it properly)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killProcess(process *os.Process, signal *timeoutSignal) error {
	//   NOTE: go only kills the process but not its sub processes.
	//   The consequence is that command.Wait() does not return, for some reason.
	//   As a workaround we use some (platform specific) magic:
	//     syscall.Kill(-pid, syscall.SIGKILL)
	//   Here '-pid' means that the KILL signal is sent to all processes
	//   in the process group whose id is 'pid'. 'prepareProcess' makes
	//   the shell we spawn the leader of its own process group and so
	//   the kill here not just kills the shell but all its descendant
	//   processes. [See manpage for kill(2)]
	return syscall.Kill(-process.Pid, syscall.SIGKILL) // note the minus sign
}

// Running powershell on linux erquired the HOME env variable to be set and to remove the TERM env variable
func validateEnvironmentVariables(command *exec.Cmd) {

	if command.Path == appconfig.PowerShellPluginCommandName {
		env := command.Env
		env = append(env, fmtEnvVariable("HOME", "/"))
		i := 0
		for _, a := range env {
			if strings.Contains(a, "TERM") {
				if i == len(env)-1 {
					env = env[:i]
				} else {
					env = append(env[:i], env[i+1:]...)
				}
				break
			}
			i++
		}
		command.Env = env
	}
}
