// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package configurepackage implements the ConfigurePackage plugin.
// configurepackage_deps contains platform dependencies that should be able to be stubbed out in tests
package configurepackage

import (
	"os"

	"github.com/aws/amazon-ssm-agent/agent/fileutil"
)

// TODO:MF: This should be able to go away when localpackages has encapsulated all filesystem access
var filesysdep fileSysDep = &fileSysDepImp{}

// dependency on filesystem and os utility functions
type fileSysDep interface {
	MakeDirExecute(destinationDir string) (err error)
	WriteFile(filename string, content string) error
	Uncompress(src, dest string) error
	RemoveAll(path string) error
}

type fileSysDepImp struct{}

func (fileSysDepImp) MakeDirExecute(destinationDir string) (err error) {
	return fileutil.MakeDirsWithExecuteAccess(destinationDir)
}

func (fileSysDepImp) WriteFile(filename string, content string) error {
	return fileutil.WriteAllText(filename, content)
}

func (fileSysDepImp) Uncompress(src, dest string) error {
	return fileutil.Unzip(src, dest)
}

func (fileSysDepImp) RemoveAll(path string) error {
	return os.RemoveAll(path)
}
