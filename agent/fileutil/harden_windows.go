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

// +build windows

package fileutil

import (
	"fmt"
	"os"
	"unsafe"

	acl "github.com/hectane/go-acl"
	aclapi "github.com/hectane/go-acl/api"
	"golang.org/x/sys/windows"
)

// access mask with full access, 4 most significant bits are set to 1.
// https://msdn.microsoft.com/en-us/library/windows/desktop/aa374892%28v=vs.85%29.aspx
var fullAccessAccessMask = uint32(15) << 28

// Harden the provided path with non-inheriting ACL for admin access only.
func Harden(path string) (err error) {

	if _, err = os.Stat(path); err != nil {
		return
	}

	builtinAdministratorsSID, buildinAdministratorsSIDLen := mallocSID(aclapi.SECURITY_MAX_SID_SIZE)
	if err = aclapi.CreateWellKnownSid(
		aclapi.WinBuiltinAdministratorsSid,
		nil, builtinAdministratorsSID,
		&buildinAdministratorsSIDLen,
	); err != nil {
		return fmt.Errorf("Failed to create SID for Built-in Administrators. %v", err)
	}

	localSystemSID, localSystemSIDLen := mallocSID(aclapi.SECURITY_MAX_SID_SIZE)
	if err = aclapi.CreateWellKnownSid(
		aclapi.WinLocalSystemSid,
		nil, localSystemSID,
		&localSystemSIDLen,
	); err != nil {
		return fmt.Errorf("Failed to create SID for LOCALSYSTYEM. %v", err)
	}

	if err = acl.Apply(
		path,
		true,  // replace current ACL
		false, // disable inheritance
		acl.GrantSid(fullAccessAccessMask, builtinAdministratorsSID),
		acl.GrantSid(fullAccessAccessMask, localSystemSID),
	); err != nil {
		return fmt.Errorf("Failed to apply ACL. %v", err)
	}
	return
}

// Allocate memory space for SID.
func mallocSID(sidSize int) (sidPtr *windows.SID, sidLen uint32) {
	var sid = make([]byte, sidSize)
	sidPtr = (*windows.SID)(unsafe.Pointer(&sid[0]))
	sidLen = uint32(len(sid))
	return
}
