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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
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
	newLineCharacter       = "\r"
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
func StartPty(log log.T, isSessionShell bool) (stdin *os.File, stdout *os.File, err error) {
	log.Info("Starting winpty")
	if _, err := os.Stat(winptyDllFilePath); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("Missing %s file.", winptyDllFilePath)
	}

	if isSessionShell {
		// Reset password for default ssm user
		var newPassword string
		newPassword, err = u.GeneratePasswordForDefaultUser()
		if err != nil {
			return nil, nil, err
		}
		if err = exec.Command(appconfig.PowerShellPluginCommandName, "net", "user", appconfig.DefaultRunAsUserName, newPassword).Run(); err != nil {
			log.Errorf("Failed to generate new password for %s: %v", appconfig.DefaultRunAsUserName, err)
			return
		}

		// TODO: check for logon user token after changing the password
		time.Sleep(8 * time.Second)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			err = startPtyAsUser(log, appconfig.DefaultRunAsUserName, newPassword)
		}()
		wg.Wait()
	} else {
		pty, err = winpty.Start(winptyDllFilePath, winptyCmd, defaultConsoleCol, defaultConsoleRow, winpty.DEFAULT_WINPTY_FLAGS)
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
func startPtyAsUser(log log.T, user string, pass string) (err error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	log.Debugf("Impersonating %s", appconfig.DefaultRunAsUserName)
	if err = impersonate(log, user, pass); err != nil {
		log.Error(err)
		return
	}

	// Start Winpty under the user context thread.
	if pty, err = winpty.Start(winptyDllFilePath, winptyCmd, defaultConsoleCol, defaultConsoleRow, winpty.WINPTY_FLAG_IMPERSONATE_THREAD); err != nil {
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
