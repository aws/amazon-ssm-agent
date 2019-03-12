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
	"golang.org/x/sys/windows"
)

const (
	// Success code for NetUser function calls
	nerrSuccess = 0
	// Nil is a zero value for pointer types
	nilPointerValue = 0
	// User level privilege
	userPrivUser = 1
	// Sever name for local machine
	serverNameLocalMachine = 0
	// Indicates that the logon script is executed
	ufScript = 1

	// Information level for domain and name of the new local group member.
	levelForLocalGroupMembersInfo3 = 3
	// Information level for password data
	levelForUserInfo1003 = 1003
	// Level for fetching USER_INFO_1 structure data
	levelForUserInfo1 = 1

	// Windows error code for user not found
	errCodeForUserNotFound = 2221
	// Winows error code for user account already exists
	errCodeForUserAlreadyExists = 2224
	// Windows error code for account name already a member of the group
	errCodeForUserAlreadyGroupMember = 1378
)

type USER_INFO_1003 struct {
	Usri1003_password *uint16
}

type USER_INFO_1 struct {
	Usri1_name         *uint16
	Usri1_password     *uint16
	Usri1_password_age uint32
	Usri1_priv         uint32
	Usri1_home_dir     *uint16
	Usri1_comment      *uint16
	Usri1_flags        uint32
	Usri1_script_path  *uint16
}

type LOCALGROUP_MEMBERS_INFO_3 struct {
	Lgrmi3_domainandname *uint16
}

var (
	modNetapi32             = syscall.NewLazyDLL("netapi32.dll")
	netUserSetInfo          = modNetapi32.NewProc("NetUserSetInfo")
	netUserGetInfo          = modNetapi32.NewProc("NetUserGetInfo")
	netUserAdd              = modNetapi32.NewProc("NetUserAdd")
	netApiBufferFree        = modNetapi32.NewProc("NetApiBufferFree")
	netLocalGroupAddMembers = modNetapi32.NewProc("NetLocalGroupAddMembers")
)

// AddNewUser adds new user using NetUserAdd function of netapi32.dll on local machine
func (u *SessionUtil) AddNewUser(username string, password string) (userExists bool, err error) {
	var (
		errParam uint32
		uPointer *uint16
		pPointer *uint16
	)

	if uPointer, err = syscall.UTF16PtrFromString(username); err != nil {
		return false, fmt.Errorf("Unable to encode username to UTF16")
	}

	if pPointer, err = syscall.UTF16PtrFromString(password); err != nil {
		return false, fmt.Errorf("Unable to encode password to UTF16")
	}

	uInfo := USER_INFO_1{
		Usri1_name:     uPointer,
		Usri1_password: pPointer,
		Usri1_priv:     userPrivUser,
		Usri1_flags:    ufScript,
	}

	ret, _, _ := netUserAdd.Call(
		uintptr(serverNameLocalMachine),
		uintptr(uint32(levelForUserInfo1)),
		uintptr(unsafe.Pointer(&uInfo)),
		uintptr(unsafe.Pointer(&errParam)),
	)

	if ret != nerrSuccess {
		if ret == errCodeForUserAlreadyExists {
			userExists = true
			err = nil
		} else {
			userExists = false
			err = fmt.Errorf("NetUserAdd call failed. Error Code: %d", ret)
		}
	}

	return
}

// AddUserToLocalAdministratorsGroup adds user to local built in administrators group using NetLocalGroupAddMembers function of netapi32.dll
func (u *SessionUtil) AddUserToLocalAdministratorsGroup(username string) (adminGroupName string, err error) {
	var (
		uPointer *uint16
		gPointer *uint16
	)

	if adminGroupName, err = u.getBuiltInAdministratorsGroupName(); err != nil {
		return
	}

	if uPointer, err = syscall.UTF16PtrFromString(username); err != nil {
		return "", fmt.Errorf("Unable to encode username to UTF16")
	}

	if gPointer, err = syscall.UTF16PtrFromString(adminGroupName); err != nil {
		return "", fmt.Errorf("Unable to encode adminGroupName to UTF16")
	}

	localGroupMembers := make([]LOCALGROUP_MEMBERS_INFO_3, 1)
	localGroupMembers[0] = LOCALGROUP_MEMBERS_INFO_3{
		Lgrmi3_domainandname: uPointer,
	}

	ret, _, _ := netLocalGroupAddMembers.Call(
		uintptr(serverNameLocalMachine),
		uintptr(unsafe.Pointer(gPointer)),
		uintptr(uint32(levelForLocalGroupMembersInfo3)),
		uintptr(unsafe.Pointer(&localGroupMembers[0])),
		uintptr(uint32(len(localGroupMembers))),
	)

	// return error if API call failed and user is NOT a group member
	if ret != nerrSuccess && ret != errCodeForUserAlreadyGroupMember {
		err = fmt.Errorf("NetLocalGroupAddMembers call failed. Error Code: %d", ret)
	}

	return
}

// getBuiltInAdministratorsGroupName fetches builtin local administrators group name
func (u *SessionUtil) getBuiltInAdministratorsGroupName() (adminGroupName string, err error) {
	var sid *windows.SID
	if err = windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0,
		0,
		0,
		0,
		0,
		0,
		&sid); err != nil {
		return
	}

	// Passing system name as empty string and LookupAccountSidW will translate it to local system
	if adminGroupName, _, _, err = sid.LookupAccount(""); err != nil {
		return
	}

	return
}

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

	ret, _, _ := netUserSetInfo.Call(
		uintptr(serverNameLocalMachine),
		uintptr(unsafe.Pointer(uPointer)),
		uintptr(uint32(levelForUserInfo1003)),
		uintptr(unsafe.Pointer(&USER_INFO_1003{Usri1003_password: pPointer})),
		uintptr(unsafe.Pointer(&errParam)),
	)

	if ret != nerrSuccess {
		return fmt.Errorf("NetUserSetInfo call failed. %d", ret)
	}
	return nil
}

// ResetPasswordIfDefaultUserExists resets default RunAs user password if user exists
func (u *SessionUtil) ResetPasswordIfDefaultUserExists(context context.T) (err error) {
	var userExists bool
	if userExists, err = u.doesUserExist(appconfig.DefaultRunAsUserName); err != nil {
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

// doesUserExist checks if given user already exists using NetUserGetInfo function of netapi32.dll on local machine
func (u *SessionUtil) doesUserExist(username string) (bool, error) {
	var (
		uPointer         *uint16
		userInfo1Pointer uintptr
		err              error
		userExists       bool
	)

	if uPointer, err = syscall.UTF16PtrFromString(username); err != nil {
		return false, fmt.Errorf("Unable to encode username to UTF16")
	}

	ret, _, _ := netUserGetInfo.Call(
		uintptr(serverNameLocalMachine),
		uintptr(unsafe.Pointer(uPointer)),
		uintptr(uint32(levelForUserInfo1)),
		uintptr(unsafe.Pointer(&userInfo1Pointer)),
	)

	if userInfo1Pointer != nilPointerValue {
		defer netApiBufferFree.Call(uintptr(unsafe.Pointer(userInfo1Pointer)))
	}

	if ret == nerrSuccess {
		userExists = true
	} else if uint(ret) == errCodeForUserNotFound {
		userExists = false
	} else {
		userExists = false
		err = fmt.Errorf("NetUserGetInfo call failed. %d", ret)
	}

	return userExists, err
}
