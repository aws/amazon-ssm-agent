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

// +build darwin freebsd linux netbsd openbsd

package fileutil

import (
	"os"
	"syscall"
)

const (
	rootUid        uint32      = 0
	rootGid        uint32      = 0
	permissionMask os.FileMode = 0777
)

// Harden the provided path with non-inheriting ACL for admin access only.
func Harden(path string) (err error) {

	var fi os.FileInfo

	if fi, err = os.Stat(path); err != nil {
		return
	}

	if fi.Mode()&permissionMask != RWPermission {
		if err = os.Chmod(path, RWPermission); err != nil {
			return
		}
	}

	s := fi.Sys().(*syscall.Stat_t)
	if s.Uid != rootUid || s.Gid != rootGid {
		if err = os.Chown(path, int(rootUid), int(rootGid)); err != nil {
			return
		}
	}
	return
}
