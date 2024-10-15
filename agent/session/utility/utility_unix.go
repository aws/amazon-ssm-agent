// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
//go:build darwin || freebsd || linux || netbsd || openbsd
// +build darwin freebsd linux netbsd openbsd

// utility package implements all the shared methods between clients.
package utility

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/session/utility/model"
)

var (
	ShellPluginCommandName = "sh"
	ShellPluginCommandArgs = []string{"-c"}
	execCommand            = exec.Command
	osStat                 = os.Stat
	osOpenFile             = os.OpenFile
	osChMod                = os.Chmod
)

const (
	sudoersFile                = "/etc/sudoers.d/ssm-agent-users"
	sudoersFileCreateWriteMode = 0640
	sudoersFileReadOnlyMode    = 0440
	fs_ioc_getflags            = uintptr(0x80086601)
	fs_ioc_setflags            = uintptr(0x40086602)
	FS_APPEND_FL               = 0x00000020 /* writes to file may only append */
	FS_RESET_FL                = 0x00000000 /* reset file property */

	dsclCreateCommand = "/usr/bin/dscl . -create /Users/%s %s %s"
)

// ResetPasswordIfDefaultUserExists resets default RunAs user password if user exists
func (u *SessionUtil) ResetPasswordIfDefaultUserExists(context context.T) (err error) {
	// Do nothing here as no password is required for unix platform local user
	return nil
}

// DoesUserExist checks if given user already exists
func (u *SessionUtil) DoesUserExist(username string) (bool, error) {
	shellCmdArgs := append(ShellPluginCommandArgs, fmt.Sprintf("id %s", username))
	cmd := execCommand(ShellPluginCommandName, shellCmdArgs...)
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// The program has exited with an exit code != 0
			return false, fmt.Errorf("encountered an error while checking for %s: %v", appconfig.DefaultRunAsUserName, exitErr.Error())
		}
		return false, nil
	}
	return true, nil
}

// CreateLocalAdminUser creates a local OS user on the instance with admin permissions. The password will alway be empty
func (u *SessionUtil) CreateLocalAdminUser(log log.T) (newPassword string, err error) {

	userExists, _ := u.DoesUserExist(appconfig.DefaultRunAsUserName)
	if userExists {
		if runtime.GOOS == "darwin" {
			if err = u.ChangeUserShell(); err != nil {
				log.Warnf("Failed to change %s UserShell: %v", appconfig.DefaultRunAsUserName, err)
				return
			}
		} else {
			log.Infof("%s already exists.", appconfig.DefaultRunAsUserName)
		}
	} else {
		if err = u.createLocalUser(log); err != nil {
			return
		}
		// only create sudoers file when user does not exist
		err = u.createSudoersFileIfNotPresent(log)
	}
	return
}

// ChangeUserShell changes userShell for DefaultRunAsUser.
func (u *SessionUtil) ChangeUserShell() (err error) {
	// update user shell value
	userShellKey := "UserShell"
	userShellNewValue := "/usr/bin/false"
	commandArgs := append(ShellPluginCommandArgs, fmt.Sprintf(dsclCreateCommand, appconfig.DefaultRunAsUserName, userShellKey, userShellNewValue))
	if err = execCommand(ShellPluginCommandName, commandArgs...).Run(); err != nil {
		return err
	}
	return nil
}

// createLocalUser creates an OS local user.
func (u *SessionUtil) createLocalUser(log log.T) error {

	commandArgs := append(ShellPluginCommandArgs, fmt.Sprintf(model.AddUserCommand, appconfig.DefaultRunAsUserName))
	cmd := execCommand(ShellPluginCommandName, commandArgs...)
	if err := cmd.Run(); err != nil {
		log.Errorf("Failed to create %s: %v", appconfig.DefaultRunAsUserName, err)
		return err
	}
	log.Infof("Successfully created %s", appconfig.DefaultRunAsUserName)
	return nil
}

// createSudoersFileIfNotPresent will create the sudoers file if not present.
func (u *SessionUtil) createSudoersFileIfNotPresent(log log.T) error {

	// Return if the file exists
	if _, err := osStat(sudoersFile); err == nil {
		log.Infof("File %s already exists", sudoersFile)
		_ = u.changeModeOfSudoersFile(log)
		return err
	}

	// Create a sudoers file for ssm-user with read/write access
	file, err := osOpenFile(sudoersFile, os.O_WRONLY|os.O_CREATE, sudoersFileCreateWriteMode)
	if err != nil {
		log.Errorf("Failed to add %s to sudoers file: %v", appconfig.DefaultRunAsUserName, err)
		return err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			log.Warnf("error occurred while closing file, %v", closeErr)
		}
	}()

	if _, err := file.WriteString(fmt.Sprintf("# User rules for %s\n", appconfig.DefaultRunAsUserName)); err != nil {
		return err
	}
	if _, err := file.WriteString(fmt.Sprintf("%s ALL=(ALL) NOPASSWD:ALL\n", appconfig.DefaultRunAsUserName)); err != nil {
		return err
	}
	log.Infof("Successfully created file %s", sudoersFile)
	_ = u.changeModeOfSudoersFile(log)
	return nil
}

// changeModeOfSudoersFile will change the sudoersFile mode to 0440 (read only).
// This file is created with mode 0640 using os.Create() so needs to be updated to read only with chmod.
func (u *SessionUtil) changeModeOfSudoersFile(log log.T) error {
	fileMode := os.FileMode(sudoersFileReadOnlyMode)
	if err := osChMod(sudoersFile, fileMode); err != nil {
		log.Errorf("Failed to change mode of %s to %d: %v", sudoersFile, sudoersFileReadOnlyMode, err)
		return err
	}
	log.Infof("Successfully changed mode of %s to %d", sudoersFile, sudoersFileReadOnlyMode)
	return nil
}

func (u *SessionUtil) DisableLocalUser(log log.T) (err error) {
	// Do nothing here as no password is required for unix platform local user, so that no need to disable user.
	return nil
}

// NewListener starts a new socket listener on the address.
func NewListener(log log.T, address string) (net.Listener, error) {
	return net.Listen("unix", address)
}

// ioctl is used for making system calls to manipulate file attributes
func ioctl(f *os.File, request uintptr, attrp *int32) error {
	argp := uintptr(unsafe.Pointer(attrp))
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), request, argp)
	if errno != 0 {
		return os.NewSyscallError("ioctl", errno)
	}

	return nil
}

// SetAttr sets the attributes of a file on a linux filesystem to the given value
func (u *SessionUtil) SetAttr(f *os.File, attr int32) error {
	return ioctl(f, fs_ioc_setflags, &attr)
}

// GetAttr retrieves the attributes of a file on a linux filesystem
func (u *SessionUtil) GetAttr(f *os.File) (int32, error) {
	attr := int32(-1)
	err := ioctl(f, fs_ioc_getflags, &attr)
	return attr, err
}

// DeleteIpcTempFile resets file properties of ipcTempFile and tries deletion
func (u *SessionUtil) DeleteIpcTempFile(log log.T, sessionOrchestrationPath string) (bool, error) {
	ipcTempFilePath := filepath.Join(sessionOrchestrationPath, appconfig.PluginNameStandardStream, "ipcTempFile.log")

	// check if ipcTempFile exists
	if _, err := os.Stat(ipcTempFilePath); err != nil {
		return false, fmt.Errorf("ipcTempFile does not exist, %v", err)
	}

	// open ipcTempFile
	ipcFile, err := os.Open(ipcTempFilePath)
	if err != nil {
		return false, fmt.Errorf("failed to open ipcTempFile %s, %v", ipcTempFilePath, err)
	}
	defer func() {
		if closeErr := ipcFile.Close(); closeErr != nil {
			log.Warnf("error occurred while closing ipcFile, %v", closeErr)
		}
	}()

	// reset file attributes
	if err := u.SetAttr(ipcFile, FS_RESET_FL); err != nil {
		return false, fmt.Errorf("unable to reset file properties for %s, %v", ipcTempFilePath, err)
	}

	// delete the directory
	if err := fileutil.DeleteDirectory(sessionOrchestrationPath); err != nil {
		return false, err
	}

	return true, nil
}
