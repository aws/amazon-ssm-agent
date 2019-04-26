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

//TODO: This package is a start to migration of the fileutil code to be inside an interface for better mocking.
// Package filemanager have all the file related dependencies used by the execute package
package filemanager

import (
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
)

// FileSystem implements dependency on filesystem and os utility functions
type FileSystem interface {
	MakeDirs(destinationDir string) (err error)
	WriteFile(filename string, content string) error
	ReadFile(filename string) (string, error)
	MoveAndRenameFile(sourcePath, sourceName, destPath, destName string) (result bool, err error)
	DeleteFile(filename string) (err error)
	DeleteDirectory(filename string) (err error)
	Exists(filename string) bool
	IsDirectory(srcPath string) bool
	AppendToFile(fileDirectory string, filename string, content string) (filePath string, err error)
}

type FileSystemImpl struct{}

// MakeDirs creates a directory with execute access
func (f FileSystemImpl) MakeDirs(destinationDir string) (err error) {
	return fileutil.MakeDirsWithExecuteAccess(destinationDir)
}

// MoveAndRenameFile moves and renames the file
func (f FileSystemImpl) MoveAndRenameFile(sourcePath, sourceName, destPath, destName string) (result bool, err error) {
	return fileutil.MoveAndRenameFile(sourcePath, sourceName, destPath, destName)
}

// DeleteFile deletes the file
func (f FileSystemImpl) DeleteFile(filename string) (err error) {
	return fileutil.DeleteFile(filename)
}

// DeleteDirectory deletes the file
func (f FileSystemImpl) DeleteDirectory(filename string) (err error) {
	return fileutil.DeleteDirectory(filename)
}

// WriteFile writes the content in the file path provided
func (f FileSystemImpl) WriteFile(filename string, content string) error {
	return fileutil.WriteAllText(filename, content)
}

// ReadFile reads the contents of file in path provided
func (f FileSystemImpl) ReadFile(filename string) (string, error) {
	return fileutil.ReadAllText(filename)
}

func (f FileSystemImpl) Exists(root string) bool {
	return fileutil.Exists(root)
}

func (f FileSystemImpl) IsDirectory(srcPath string) bool {
	return fileutil.IsDirectory(srcPath)
}

// AppendToFile appends contents to file
func (f FileSystemImpl) AppendToFile(fileDirectory string, filename string, content string) (filePath string, err error) {
	return fileutil.AppendToFile(fileDirectory, filename, content)
}
