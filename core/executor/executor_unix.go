// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package executor

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"syscall"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
)

// Collect processes in these states when querying from /Proc
var accepted_process_states = map[string]bool{
	"R": true, // Running/Runnable
	"S": true, // Interruptible sleep
	"D": true, // uninterruptible sleep
	"Z": true, // zombie
}

func prepareProcess(command *exec.Cmd) {
	// make the process the leader of its process group
	// (otherwise we cannot kill it properly)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func terminateProcess(process *os.Process) error {
	return process.Signal(syscall.SIGTERM)
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

// Running powershell on linux required the HOME env variable to be set and to remove the TERM env variable
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

// Unix man: http://www.skrenta.com/rt/man/ps.1.html , return the process table of the current user, in agent it'll be root
// verified on RHEL, Amazon Linux, Ubuntu, Centos, and FreeBSD
var listProcessPs = func() ([]byte, error) {
	return exec.Command("ps", "-e", "-o", "pid,ppid,state,command").CombinedOutput()
}

// Unix man: http://man7.org/linux/man-pages/man5/proc.5.html
// listProcessProc is a fallback function for when listProcessPs fails, it reads the /proc folder for process information
var listProcessProc = func() ([]OsProcess, error) {
	var procFolder = "/proc"
	var currProcUid = uint32(os.Geteuid())
	var results []OsProcess

	// Get files in proc dir
	files, err := ioutil.ReadDir(procFolder)

	if err != nil {
		return results, err
	}

	for _, f := range files {
		// Only look at folders in /proc
		if f.IsDir() {
			// Cast folder name to int to only include pid folders
			if pid, err := strconv.Atoi(f.Name()); err == nil {
				// Read the cmdline file to extract the command used to start the process
				cmd, err := ioutil.ReadFile(path.Join(procFolder, f.Name(), "cmdline"))
				if err != nil || len(cmd) == 0 {
					// Failed to read or is empty cmdline file, skip process
					continue
				}

				// Check owner of process, ignore check if cast fails
				if stat, ok := f.Sys().(*syscall.Stat_t); ok {
					if stat.Uid != currProcUid {
						// Owner of process is not uid of agent process
						continue
					}
				}

				// Read the stat file for process state
				stat, err := ioutil.ReadFile(path.Join(procFolder, f.Name(), "stat"))
				if err != nil {
					// Failed to read stat file, skip process
					continue
				}

				// Split the file and make sure there are at least 4 entries
				splitStat := strings.SplitN(string(stat), " ", 5)
				if len(splitStat) < 4 {
					// Failed to split stat file, skip process
					continue
				}

				// Check if process state is valid
				state := splitStat[2]
				if !accepted_process_states[state] {
					// State of process is not valid, skip process
					continue
				}

				// Get process parent
				ppid := -1
				if ppid, err = strconv.Atoi(splitStat[3]); err != nil {
					// Failed to convert ppid to int
					continue
				}

				// split at null character
				cmdString := string(bytes.SplitN(cmd, []byte{0}, 2)[0])

				results = append(results, OsProcess{Pid: pid, PPid: ppid, State: state, Executable: cmdString})
			}
		}
	}
	return results, err
}

var getProcess = func() ([]OsProcess, error) {
	output, err := listProcessPs()
	if err != nil {
		// Default to Proc if ps fails
		return listProcessProc()
	}

	var results []OsProcess
	procList := strings.Split(string(output), "\n")
	for i := 1; i < len(procList); i++ {
		parts := strings.Fields(procList[i])
		if len(parts) < 4 {
			continue
		}
		pid, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}

		ppid, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}

		state := parts[2]
		if len(state) > 1 {
			state = string(state[0])
		}

		results = append(results, OsProcess{Pid: pid, PPid: ppid, State: state, Executable: parts[3]})
	}
	return results, nil
}
