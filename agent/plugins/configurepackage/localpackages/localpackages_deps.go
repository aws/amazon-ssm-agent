// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package localpackages implements the local storage for packages managed by the ConfigurePackage plugin.
package localpackages

import (
	"io/ioutil"
	"os"

	"github.com/aws/amazon-ssm-agent/agent/fileutil"
)

// dependency on filesystem and os utility functions
type FileSysDep interface {
	MakeDirExecute(destinationDir string) (err error)
	GetDirectoryNames(srcPath string) (directories []string, err error)
	GetFileNames(srcPath string) (files []string, err error)
	Exists(filePath string) bool
	RemoveAll(path string) error
	ReadFile(filename string) ([]byte, error)
	WriteFile(filename string, content string) error
}

type fileSysDepImp struct{}

func (fileSysDepImp) MakeDirExecute(destinationDir string) (err error) {
	return fileutil.MakeDirsWithExecuteAccess(destinationDir)
}

func (fileSysDepImp) GetDirectoryNames(srcPath string) (directories []string, err error) {
	return fileutil.GetDirectoryNames(srcPath)
}

func (fileSysDepImp) GetFileNames(srcPath string) (files []string, err error) {
	return fileutil.GetFileNames(srcPath)
}

func (fileSysDepImp) Exists(filePath string) bool {
	return fileutil.Exists(filePath)
}

func (fileSysDepImp) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (fileSysDepImp) ReadFile(filename string) ([]byte, error) {
	return ioutil.ReadFile(filename)
}

func (fileSysDepImp) WriteFile(filename string, content string) error {
	return fileutil.WriteAllText(filename, content)
}
