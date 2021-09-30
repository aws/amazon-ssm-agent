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

package certreader

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
)

var getSys = func(info os.FileInfo) interface{} {
	return info.Sys()
}

var getIsRegular = func(info os.FileInfo) bool {
	return info.Mode().IsRegular()
}

var getPerm = func(info os.FileInfo) os.FileMode {
	return info.Mode().Perm()
}

var getStat = os.Stat
var readFile = ioutil.ReadFile

// ReadCertificates returns a certificate from a given path
// If certificate or its containing folder is not owned by root, or does not have read only permissions, it returns an error
func ReadCertificate(certificatePath string) ([]byte, error) {

	if certificatePath == "" {
		return nil, fmt.Errorf("Certificate path not set")
	}

	// Get folder stat
	folderStat, err := getStat(filepath.Dir(certificatePath))
	if err != nil {
		return nil, fmt.Errorf("Certificate folder does not exist")
	}

	// Get folder resource information
	folderSys := getSys(folderStat)
	if folderSys.(*syscall.Stat_t).Uid != 0 ||
		folderSys.(*syscall.Stat_t).Gid != 0 {
		return nil, fmt.Errorf("Certificate folder is not owned by root")
	}

	// Get file stat
	fileStat, err := getStat(certificatePath)
	if err != nil {
		return nil, fmt.Errorf("Certificate does not exist")
	}

	// Check if is file
	if !getIsRegular(fileStat) {
		return nil, fmt.Errorf("Certificate path is not a file")
	}

	// Check if certificate has read only permission
	if getPerm(fileStat) != 0400 {
		return nil, fmt.Errorf("Certificate does not have only owner read permission: %d", uint32(getPerm(fileStat)))
	}

	// Check if file ownership is root
	fileSys := getSys(fileStat)
	if fileSys.(*syscall.Stat_t).Uid != 0 ||
		fileSys.(*syscall.Stat_t).Gid != 0 {
		return nil, fmt.Errorf("Certificate is not owned by root")
	}

	// Read certificate
	cert, err := readFile(certificatePath)
	if err != nil {
		return nil, fmt.Errorf("Failed to read certificate")
	}

	return cert, nil
}
