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
)

type USER_INFO_1003 struct {
	Usri1003_password *uint16
}

const (
	// Success code for NetUserSetInfo function call
	NET_USER_SET_INFO_STATUS_SUCCESS = 0
	// Information level for password data
	INFO_LEVEL_PASSWORD = 1003
	// Sever name for local machine
	SERVER_NAME_LOCAL_MACHINE = 0
)

var (
	modNetapi32       = syscall.NewLazyDLL("netapi32.dll")
	usrNetUserSetInfo = modNetapi32.NewProc("NetUserSetInfo")
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

	if ret != NET_USER_SET_INFO_STATUS_SUCCESS {
		return fmt.Errorf("NetUserSetInfo call failed. %d", ret)
	}
	return nil
}
