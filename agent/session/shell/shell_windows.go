// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
//
// +build windows

// Package shell implements session shell plugin.
package shell

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	agentContracts "github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	mgsConfig "github.com/aws/amazon-ssm-agent/agent/session/config"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/aws/amazon-ssm-agent/agent/session/utility"
	"github.com/aws/amazon-ssm-agent/agent/session/winpty"
)

var pty *winpty.WinPTY
var u = &utility.SessionUtil{}

const (
	defaultConsoleCol      = 200
	defaultConsoleRow      = 60
	winptyDllName          = "winpty.dll"
	winptyDllFolderName    = "SessionManagerShell"
	winptyCmd              = "powershell"
	startRecordSessionCmd  = "Start-Transcript"
	newLineCharacter       = "\r\n"
	screenBufferSizeCmd    = "$host.UI.RawUI.BufferSize = New-Object System.Management.Automation.Host.Size($host.UI.RawUI.BufferSize.Width,%d)%s"
	logon32LogonNetwork    = uintptr(3)
	logon32ProviderDefault = uintptr(0)
)

var (
	advapi32          = syscall.NewLazyDLL("advapi32.dll")
	logonProc         = advapi32.NewProc("LogonUserW")
	impersonateProc   = advapi32.NewProc("ImpersonateLoggedOnUser")
	revertSelfProc    = advapi32.NewProc("RevertToSelf")
	winptyDllDir      = fileutil.BuildPath(appconfig.DefaultPluginPath, winptyDllFolderName)
	winptyDllFilePath = filepath.Join(winptyDllDir, winptyDllName)
)

//StartPty starts winpty agent and provides handles to stdin and stdout.
// isSessionLogger determines whether its a customer shell or shell used for logging.
func StartPty(
	log log.T,
	shellProps mgsContracts.ShellProperties,
	isSessionLogger bool,
	config agentContracts.Configuration) (stdin *os.File, stdout *os.File, err error) {
	log.Info("Starting winpty")
	if _, err := os.Stat(winptyDllFilePath); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("Missing %s file.", winptyDllFilePath)
	}

	var finalCmd string
	if strings.TrimSpace(shellProps.Windows.Commands) == "" || isSessionLogger {
		finalCmd = winptyCmd
	} else {
		finalCmd = winptyCmd + " " + shellProps.Windows.Commands
	}

	if !shellProps.Windows.RunAsElevated && !isSessionLogger {
		// Reset password for default ssm user
		var newPassword string
		newPassword, err = u.GeneratePasswordForDefaultUser()
		if err != nil {
			return nil, nil, err
		}
		var userExists bool
		if userExists, err = u.ChangePassword(appconfig.DefaultRunAsUserName, newPassword); err != nil {
			log.Errorf("Failed to generate new password for %s: %v", appconfig.DefaultRunAsUserName, err)
			return
		}

		// create ssm-user before starting a new session
		if !userExists {
			if newPassword, err = u.CreateLocalAdminUser(log); err != nil {
				return nil, nil, fmt.Errorf("Failed to create user %s: %v", appconfig.DefaultRunAsUserName, err)
			}
		} else {
			// enable user
			if err = u.EnableLocalUser(log); err != nil {
				return nil, nil, fmt.Errorf("Failed to enable user %s: %v", appconfig.DefaultRunAsUserName, err)
			}
		}

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			err = startPtyAsUser(log, appconfig.DefaultRunAsUserName, newPassword, finalCmd)
		}()
		wg.Wait()
	} else {
		pty, err = winpty.Start(winptyDllFilePath, finalCmd, defaultConsoleCol, defaultConsoleRow, winpty.DEFAULT_WINPTY_FLAGS)
	}

	if err != nil {
		return nil, nil, err
	}

	return pty.StdIn, pty.StdOut, err
}

//Stop closes winpty process handle and stdin/stdout.
func Stop(log log.T) (err error) {
	log.Info("Stopping winpty")
	if err = pty.Close(); err != nil {
		return fmt.Errorf("Stop winpty failed: %s", err)
	}

	log.Debugf("Disabling ssm-user")
	u.DisableLocalUser(log)
	return nil
}

//SetSize sets size of console terminal window.
func SetSize(log log.T, ws_col, ws_row uint32) (err error) {
	if err = pty.SetSize(ws_col, ws_row); err != nil {
		return fmt.Errorf("Set winpty size failed: %s", err)
	}

	return nil
}

//startPtyAsUser starts a winpty process in runas user context.
func startPtyAsUser(log log.T, user string, pass string, shellCmd string) (err error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	log.Debugf("Impersonating %s", appconfig.DefaultRunAsUserName)
	if err = impersonate(log, user, pass); err != nil {
		log.Error(err)
		return
	}

	// Start Winpty under the user context thread.
	if pty, err = winpty.Start(winptyDllFilePath, shellCmd, defaultConsoleCol, defaultConsoleRow, winpty.WINPTY_FLAG_IMPERSONATE_THREAD); err != nil {
		log.Error(err)
		return
	}

	if err = revertToSelf(); err != nil {
		log.Error(err)
		return
	}
	log.Debug("Reverted to system profile.")

	return
}

//impersonate attempts to impersonate the user.
func impersonate(log log.T, user string, pass string) error {
	token, err := logonUser(user, pass)
	if err != nil {
		return err
	}
	defer mustCloseHandle(log, token)

	if rc, _, ec := syscall.Syscall(impersonateProc.Addr(), 1, uintptr(token), 0, 0); rc == 0 {
		return error(ec)
	}
	return nil
}

//logonUser attempts to log a user on to the local computer to generate a token.
func logonUser(user, pass string) (token syscall.Handle, err error) {
	// ".\0" meaning "this computer:
	domain := [2]uint16{uint16('.'), 0}

	var pu, pp []uint16
	if pu, err = syscall.UTF16FromString(user); err != nil {
		return
	}
	if pp, err = syscall.UTF16FromString(pass); err != nil {
		return
	}

	if rc, _, ec := syscall.Syscall6(logonProc.Addr(), 6,
		uintptr(unsafe.Pointer(&pu[0])),
		uintptr(unsafe.Pointer(&domain[0])),
		uintptr(unsafe.Pointer(&pp[0])),
		logon32LogonNetwork,
		logon32ProviderDefault,
		uintptr(unsafe.Pointer(&token))); rc == 0 {
		err = error(ec)
	}
	return
}

//revertToSelf reverts the impersonation process.
func revertToSelf() error {
	if rc, _, ec := syscall.Syscall(revertSelfProc.Addr(), 0, 0, 0, 0); rc == 0 {
		return error(ec)
	}
	return nil
}

//mustCloseHandle ensures to close the user token handle.
func mustCloseHandle(log log.T, handle syscall.Handle) {
	if err := syscall.CloseHandle(handle); err != nil {
		log.Error(err)
	}
}

// generateLogData generates a log file with the executed commands.
func (p *ShellPlugin) generateLogData(log log.T, config agentContracts.Configuration) error {
	platformVersion, _ := platform.PlatformVersion(log)

	osVersionSplit := strings.Split(platformVersion, ".")
	if osVersionSplit == nil || len(osVersionSplit) < 2 {
		return fmt.Errorf("error occurred while parsing OS version: %s", platformVersion)
	}

	// check if the OS version is 6.1 or higher
	osMajorVersion, err := strconv.Atoi(osVersionSplit[0])
	if err != nil {
		return err
	}

	osMinorVersion, err := strconv.Atoi(osVersionSplit[1])
	if err != nil {
		return err
	}

	// Generate logs based on the OS version number
	// https://docs.microsoft.com/en-us/windows/desktop/SysInfo/operating-system-version
	if osMajorVersion >= 10 {
		if err = generateTranscriptFile(log, p.logFilePath, p.ipcFilePath, true, config); err != nil {
			return err
		}
	} else if osMajorVersion >= 6 && osMinorVersion >= 3 {
		transcriptFile := filepath.Join(config.OrchestrationDirectory, "transcriptFile"+mgsConfig.LogFileExtension)
		if err = generateTranscriptFile(log, transcriptFile, p.ipcFilePath, false, config); err != nil {
			return err
		}
		cleanControlCharacters(transcriptFile, p.logFilePath)
	} else {
		cleanControlCharacters(p.ipcFilePath, p.logFilePath)
	}

	return nil
}

// generateTranscriptFile generates a transcript file using PowerShell
func generateTranscriptFile(
	log log.T,
	transcriptFile string,
	loggerFile string,
	enableVirtualTerminalProcessingForWindows bool,
	config agentContracts.Configuration) error {
	shadowShellInput, _, err := StartPty(log, mgsContracts.ShellProperties{}, true, config)
	if err != nil {
		return err
	}

	defer func() {
		if err := recover(); err != nil {
			if err = Stop(log); err != nil {
				log.Errorf("Error occured while closing pty: %v", err)
			}
		}
	}()

	time.Sleep(5 * time.Second)

	// Increase buffer size
	screenBufferSizeCmdInput := fmt.Sprintf(screenBufferSizeCmd, mgsConfig.ScreenBufferSize, newLineCharacter)
	shadowShellInput.Write([]byte(screenBufferSizeCmdInput))

	time.Sleep(5 * time.Second)

	// Start shell recording
	recordCmdInput := fmt.Sprintf("%s %s%s", startRecordSessionCmd, transcriptFile, newLineCharacter)
	shadowShellInput.Write([]byte(recordCmdInput))

	time.Sleep(5 * time.Second)

	// Start shell logger
	loggerCmdInput := fmt.Sprintf("%s %s %t%s", appconfig.DefaultSessionLogger, loggerFile, enableVirtualTerminalProcessingForWindows, newLineCharacter)
	shadowShellInput.Write([]byte(loggerCmdInput))

	// Sleep till the logger completes execution
	time.Sleep(time.Minute)

	// Exit shell
	exitCmdInput := fmt.Sprintf("%s%s", mgsConfig.Exit, newLineCharacter)
	shadowShellInput.Write([]byte(exitCmdInput))

	// Sleep till the shell successfully exits before uploading
	time.Sleep(5 * time.Second)

	return nil
}

// cleanControlCharacters cleans up control characters from the log file
func cleanControlCharacters(sourceFileName, destinationFileName string) error {
	sourceFile, err := os.Open(sourceFileName)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(destinationFileName)
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	escapeCharRegEx := regexp.MustCompile(`←`)
	specialChar1RegEx := regexp.MustCompile(`\[\?[0-9]+[a-zA-Z]`)
	specialChar2RegEx := regexp.MustCompile(`\[[0-9]+[A-Z]`)
	newLineCharRegEx := regexp.MustCompile(`\[0K`)

	emptyString := []byte("")
	scanner := bufio.NewScanner(sourceFile)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		var line []byte
		line = append(line, scanner.Bytes()...)
		line = escapeCharRegEx.ReplaceAll(line, emptyString)
		line = specialChar1RegEx.ReplaceAll(line, emptyString)
		line = specialChar2RegEx.ReplaceAll(line, emptyString)
		line = newLineCharRegEx.ReplaceAll(line, emptyString)

		// clean up pending escape characters if any
		var output []byte
		for _, v := range line {
			if v == 27 {
				output = append(output, emptyString...)
			} else {
				output = append(output, v)
			}
		}

		destinationFile.Write(append(output, []byte(newLineCharacter)...))
	}
	return nil
}

// InputStreamMessageHandler passes payload byte stream to shell stdin
func (p *ShellPlugin) InputStreamMessageHandler(log log.T, streamDataMessage mgsContracts.AgentMessage) error {
	if p.stdin == nil || p.stdout == nil {
		// This is to handle scenario when cli/console starts sending size data but pty has not been started yet
		// Since packets are rejected, cli/console will resend these packets until pty starts successfully in separate thread
		log.Tracef("Pty unavailable. Reject incoming message packet")
		return nil
	}

	switch mgsContracts.PayloadType(streamDataMessage.PayloadType) {
	case mgsContracts.Output:
		log.Tracef("Output message received: %d", streamDataMessage.SequenceNumber)

		// deal with powershell nextline issue https://github.com/lzybkr/PSReadLine/issues/579
		payloadString := string(streamDataMessage.Payload)
		if strings.Contains(payloadString, "\r\n") {
			// From windows machine, do nothing
		} else if strings.Contains(payloadString, "\n") {
			// From linux machine, replace \n with \r
			num := strings.Index(payloadString, "\n")
			payloadString = strings.Replace(payloadString, "\n", "\r", num-1)
		}

		if _, err := p.stdin.Write([]byte(payloadString)); err != nil {
			log.Errorf("Unable to write to stdin, err: %v.", err)
			return err
		}
	case mgsContracts.Size:
		var size mgsContracts.SizeData
		if err := json.Unmarshal(streamDataMessage.Payload, &size); err != nil {
			log.Errorf("Invalid size message: %s", err)
			return err
		}
		log.Tracef("Resize data received: cols: %d, rows: %d", size.Cols, size.Rows)
		if err := SetSize(log, size.Cols, size.Rows); err != nil {
			log.Errorf("Unable to set pty size: %s", err)
			return err
		}
	}
	return nil
}
