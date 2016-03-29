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

// +build windows

package executers

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	// RunCommandScriptName is the script name where all downloaded or provided commands will be stored
	RunCommandScriptName = "_script.ps1"
	// PowershellArgs specifies the default arguments that we pass to powershell
	PowershellArgs = "-InputFormat None -Noninteractive -NoProfile -ExecutionPolicy unrestricted -f"
	// Currently we run powershell as powershell.exe [arguments], with this approach we are not able to get the $LASTEXITCODE value
	// if we want to run multiple commands then we need to run them via shell and not directly the command.
	// https://groups.google.com/forum/#!topic/golang-nuts/ggd3ww3ZKcI
	ExitCodeTrap                       = " ; exit $LASTEXITCODE"
	CommandStoppedPreemptivelyExitCode = -1
)

func prepareProcess(command *exec.Cmd) {
	// nothing to do on windows
}

func killProcess(process *os.Process) error {
	return process.Kill()
}

// NewShellCommandExecuter creates a shell executer where the shell command is 'sh'.
func NewShellCommandExecuter() *ShellCommandExecuter {
	return &ShellCommandExecuter{
		ShellCommand:          filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe"),
		ShellDefaultArguments: strings.Split(PowershellArgs, " "),
		ShellExitCodeTrap:     ExitCodeTrap,
		ScriptName:            RunCommandScriptName,
		StdoutFileName:        "stdout",
		StderrFileName:        "stderr",
	}
}
