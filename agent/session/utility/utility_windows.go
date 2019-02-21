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

// utility package implements all the shared methods between clients.
package utility

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
)

type USER_INFO_1003 struct {
	Usri1003_password *uint16
}

const (
	// Success code for NetUser function calls
	NERR_SUCCESS = 0
	// Nil is a zero value for pointer types
	NIL_POINTER_VALUE = 0
	// Windows error code for user not found
	USER_NAME_NOT_FOUND_ERROR_CODE = 2221
	// Information level for password data
	INFO_LEVEL_PASSWORD = 1003
	// Level for fetching USER_INFO_1 structure data
	LEVEL_USER_INFO_1 = 1
	// Sever name for local machine
	SERVER_NAME_LOCAL_MACHINE = 0
)

var (
	modNetapi32         = syscall.NewLazyDLL("netapi32.dll")
	usrNetUserSetInfo   = modNetapi32.NewProc("NetUserSetInfo")
	usrNetUserGetInfo   = modNetapi32.NewProc("NetUserGetInfo")
	usrNetApiBufferFree = modNetapi32.NewProc("NetApiBufferFree")
)

// ChangePassword changes password for given user using NetUserSetInfo function of netapi32.dll on local machine
func (u *SessionUtil) ChangePassword(username string, password string) error {
	var (
		errParam uint32
		uPointer *uint16
		pPointer *uint16
		err      error
	)

	if uPointer, err = syscall.UTF16PtrFromString(username); err != nil {
		return fmt.Errorf("Unable to encode username to UTF16")
	}

	if pPointer, err = syscall.UTF16PtrFromString(password); err != nil {
		return fmt.Errorf("Unable to encode password to UTF16")
	}

	ret, _, _ := usrNetUserSetInfo.Call(
		uintptr(SERVER_NAME_LOCAL_MACHINE),
		uintptr(unsafe.Pointer(uPointer)),
		uintptr(uint32(INFO_LEVEL_PASSWORD)),
		uintptr(unsafe.Pointer(&USER_INFO_1003{Usri1003_password: pPointer})),
		uintptr(unsafe.Pointer(&errParam)),
	)

	if ret != NERR_SUCCESS {
		return fmt.Errorf("NetUserSetInfo call failed. %d", ret)
	}
	return nil
}

// ResetPasswordIfDefaultUserExists resets default RunAs user password if user exists
func (u *SessionUtil) ResetPasswordIfDefaultUserExists(context context.T) (err error) {
	var userExists bool
	if userExists, err = u.DoesUserExist(appconfig.DefaultRunAsUserName); err != nil {
		return fmt.Errorf("Error occured while checking if %s user exists, %v", appconfig.DefaultRunAsUserName, err)
	}

	if userExists {
		log := context.Log()
		log.Infof("%s already exists. Resetting password.", appconfig.DefaultRunAsUserName)
		newPassword, err := u.GeneratePasswordForDefaultUser()
		if err != nil {
			return err
		}
		if err = u.ChangePassword(appconfig.DefaultRunAsUserName, newPassword); err != nil {
			return fmt.Errorf("Error occured while changing password for %s, %v", appconfig.DefaultRunAsUserName, err)
		}
	}

	return nil
}

// DoesUserExist checks if given user already exists using NetUserGetInfo function of netapi32.dll on local machine
func (u *SessionUtil) DoesUserExist(username string) (bool, error) {
	var (
		uPointer         *uint16
		userInfo1Pointer uintptr
		err              error
		userExists       bool
	)

	if uPointer, err = syscall.UTF16PtrFromString(username); err != nil {
		return false, fmt.Errorf("Unable to encode username to UTF16")
	}

	ret, _, _ := usrNetUserGetInfo.Call(
		uintptr(SERVER_NAME_LOCAL_MACHINE),
		uintptr(unsafe.Pointer(uPointer)),
		uintptr(uint32(LEVEL_USER_INFO_1)),
		uintptr(unsafe.Pointer(&userInfo1Pointer)),
	)

	if userInfo1Pointer != NIL_POINTER_VALUE {
		defer usrNetApiBufferFree.Call(uintptr(unsafe.Pointer(userInfo1Pointer)))
	}

	if ret == NERR_SUCCESS {
		userExists = true
	} else if uint(ret) == USER_NAME_NOT_FOUND_ERROR_CODE {
		userExists = false
	} else {
		userExists = false
		err = fmt.Errorf("NetUserGetInfo call failed. %d", ret)
	}

	return userExists, err
}
