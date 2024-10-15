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
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCertificateEmptyPath(t *testing.T) {
	_, err := ReadCertificate("")

	assert.NotNil(t, err, "Custom certificate should not be returned when path is empty")
	assert.Equal(t, err.Error(), "Certificate path not set", "Invalid error message")
}

func TestCertificateFolderNotExist(t *testing.T) {
	getStat = func(path string) (os.FileInfo, error) {
		return nil, fmt.Errorf("SomeRandomError")
	}

	_, err := ReadCertificate("SomeRandomPath")

	assert.NotNil(t, err, "Custom certificate should not be returned when folder stat returns error")
	assert.Equal(t, err.Error(), "Certificate folder does not exist", "Invalid error message")
}

func TestCertificateFolderNotOwnedByRoot(t *testing.T) {
	getStat = func(path string) (os.FileInfo, error) {
		return nil, nil
	}

	getSys = func(info os.FileInfo) interface{} {
		obj := syscall.Stat_t{}

		obj.Uid = 1
		obj.Gid = 1
		return &obj
	}

	_, err := ReadCertificate("SomeRandomPath")

	assert.NotNil(t, err, "Custom certificate should not be returned when folder stat returns error")
	assert.Equal(t, err.Error(), "Certificate folder is not owned by root", "Invalid error message")
}

func TestCertificateFileNotExists(t *testing.T) {

	getStatFileInfoReturns := []os.FileInfo{
		nil, nil,
	}
	getStatErrorReturns := []error{
		nil, fmt.Errorf("SomeRandomError"),
	}
	getStatCounter := 0
	getStat = func(path string) (os.FileInfo, error) {
		defer func() { getStatCounter += 1 }()
		return getStatFileInfoReturns[getStatCounter], getStatErrorReturns[getStatCounter]
	}

	getSys = func(info os.FileInfo) interface{} {
		obj := syscall.Stat_t{}

		obj.Uid = 0
		obj.Gid = 0
		return &obj
	}

	_, err := ReadCertificate("SomeRandomPath")

	assert.NotNil(t, err, "Custom certificate should not be returned when folder stat returns error")
	assert.Equal(t, err.Error(), "Certificate does not exist", "Invalid error message")
}

func TestCertificateIsNotAFile(t *testing.T) {

	getStat = func(path string) (os.FileInfo, error) {
		return nil, nil
	}

	getSys = func(info os.FileInfo) interface{} {
		obj := syscall.Stat_t{}

		obj.Uid = 0
		obj.Gid = 0
		return &obj
	}

	getIsRegular = func(info os.FileInfo) bool {
		return false
	}

	_, err := ReadCertificate("SomeRandomPath")

	assert.NotNil(t, err, "Custom certificate should not be returned when folder stat returns error")
	assert.Equal(t, err.Error(), "Certificate path is not a file", "Invalid error message")
}

func TestCertificateIsNotReadOnly(t *testing.T) {

	getStat = func(path string) (os.FileInfo, error) {
		return nil, nil
	}

	getSys = func(info os.FileInfo) interface{} {
		obj := syscall.Stat_t{}

		obj.Uid = 0
		obj.Gid = 0

		return &obj
	}

	getIsRegular = func(info os.FileInfo) bool {
		return true
	}

	getPerm = func(info os.FileInfo) os.FileMode {
		return 0500
	}

	_, err := ReadCertificate("SomeRandomPath")

	assert.NotNil(t, err, "Custom certificate should not be returned when folder stat returns error")
	assert.Regexp(t, "^Certificate does not have only owner read permission", err.Error(), "Invalid error message")
}

func TestCertificateFileNotOwnedByRoot(t *testing.T) {

	getStat = func(path string) (os.FileInfo, error) {
		return nil, nil
	}

	getSysUidReturns := []uint32{
		0, 0,
	}
	getSysGidReturns := []uint32{
		0, 1,
	}
	getSysCounter := 0
	getSys = func(info os.FileInfo) interface{} {
		obj := syscall.Stat_t{}

		obj.Uid = getSysUidReturns[getSysCounter]
		obj.Gid = getSysGidReturns[getSysCounter]
		getSysCounter += 1

		return &obj
	}

	getIsRegular = func(info os.FileInfo) bool {
		return true
	}

	getPerm = func(info os.FileInfo) os.FileMode {
		return 0400
	}

	_, err := ReadCertificate("SomeRandomPath")

	assert.NotNil(t, err, "Custom certificate should not be returned when folder stat returns error")
	assert.Equal(t, "Certificate is not owned by root", err.Error(), "Invalid error message")
}

func TestCertificateFailToRead(t *testing.T) {

	getStat = func(path string) (os.FileInfo, error) {
		return nil, nil
	}

	getSys = func(info os.FileInfo) interface{} {
		obj := syscall.Stat_t{}

		obj.Uid = 0
		obj.Gid = 0

		return &obj
	}

	getIsRegular = func(info os.FileInfo) bool {
		return true
	}

	getPerm = func(info os.FileInfo) os.FileMode {
		return 0400
	}

	readFile = func(filePath string) ([]byte, error) {
		return nil, fmt.Errorf("SomeRandomError")
	}

	_, err := ReadCertificate("SomeRandomPath")

	assert.NotNil(t, err, "Custom certificate should not be returned when folder stat returns error")
	assert.Equal(t, "Failed to read certificate", err.Error(), "Invalid error message")
}

func TestCertificateSuccess(t *testing.T) {

	getStat = func(path string) (os.FileInfo, error) {
		return nil, nil
	}

	getSys = func(info os.FileInfo) interface{} {
		obj := syscall.Stat_t{}

		obj.Uid = 0
		obj.Gid = 0

		return &obj
	}

	getIsRegular = func(info os.FileInfo) bool {
		return true
	}

	getPerm = func(info os.FileInfo) os.FileMode {
		return 0400
	}

	readFile = func(filePath string) ([]byte, error) {
		return []byte("Success"), nil
	}

	cert, err := ReadCertificate("SomeRandomPath")

	assert.Nil(t, err, "Custom certificate should be returned")
	assert.NotNil(t, cert, "Custom certificate should be returned")
	assert.Equal(t, "Success", string(cert), "Invalid certificate")
}
