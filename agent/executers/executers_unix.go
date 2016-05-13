// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// +build darwin freebsd linux netbsd openbsd

package executers

import (
	"os"
	"os/exec"
	"syscall"
)

const (
	// RunCommandScriptName is the script name where all downloaded or provided commands will be stored
	RunCommandScriptName               = "_script.sh"
	ExitCodeTrap                       = ""
	CommandStoppedPreemptivelyExitCode = 137 // Fatal error (128) + signal for SIGKILL (9) = 137
)

func prepareProcess(command *exec.Cmd) {
	// make the process the leader of its process group
	// (otherwise we cannot kill it properly)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killProcess(process *os.Process) error {
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

// NewShellCommandExecuter creates a shell executer where the shell command is 'sh'.
func NewShellCommandExecuter() *ShellCommandExecuter {
	return &ShellCommandExecuter{
		ShellCommand:          "sh",
		ShellDefaultArguments: []string{"-c"},
		ShellExitCodeTrap:     ExitCodeTrap,
		ScriptName:            RunCommandScriptName,
		StdoutFileName:        "stdout",
		StderrFileName:        "stderr",
	}
}
