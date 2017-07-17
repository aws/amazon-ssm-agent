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

// Package longrunning implements longrunning plugins
package longrunning

import (
	"io"
	"io/ioutil"
	"os"

	"github.com/aws/amazon-ssm-agent/agent/fileutil"
)

type FileSysUtil interface {
	Exists(filePath string) bool
	MakeDirs(destinationDir string) error
	WriteIntoFileWithPermissions(absolutePath, content string, perm os.FileMode) (bool, error)
	ReadFile(filename string) ([]byte, error)
	ReadAll(r io.Reader) ([]byte, error)
}

type FileSysUtilImpl struct{}

func (FileSysUtilImpl) Exists(filePath string) bool {
	return fileutil.Exists(filePath)
}

func (FileSysUtilImpl) MakeDirs(destinationDir string) error {
	return fileutil.MakeDirs(destinationDir)
}

func (FileSysUtilImpl) WriteIntoFileWithPermissions(absolutePath, content string, perm os.FileMode) (bool, error) {
	return fileutil.WriteIntoFileWithPermissions(absolutePath, content, perm)
}

func (FileSysUtilImpl) ReadFile(filename string) ([]byte, error) {
	return ioutil.ReadFile(filename)
}

func (FileSysUtilImpl) ReadAll(r io.Reader) ([]byte, error) {
	return ioutil.ReadAll(r)
}
