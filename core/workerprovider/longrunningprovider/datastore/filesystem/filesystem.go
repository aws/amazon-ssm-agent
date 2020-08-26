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

// Package filesystem contains related functions from os, io, and io/ioutil packages
package filesystem

import (
	"fmt"
	"io/ioutil"
	"os"
)

type IFileSystem interface {
	MkdirAll(path string, perm os.FileMode) error
	WriteFile(filename string, data []byte, perm os.FileMode) error
	ReadFile(filename string) ([]byte, error)
	Stat(name string) (os.FileInfo, error)
	IsNotExist(err error) bool
	DeleteFile(fileName string) error
	ReadDir(path string) ([]os.FileInfo, error)
	AppendToFile(filename string, content string, perm os.FileMode) error
}

type FileSystem struct{}

func NewFileSystem() *FileSystem {
	return &FileSystem{}
}

// ReadDir returns directory/file names
func (fileSystem *FileSystem) ReadDir(path string) ([]os.FileInfo, error) {
	return ioutil.ReadDir(path)
}

// MkdirAll makes directory
func (fileSystem *FileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// WriteFile writes data to a file
func (fileSystem *FileSystem) WriteFile(filename string, data []byte, perm os.FileMode) error {
	return ioutil.WriteFile(filename, data, perm)
}

// ReadFile reads data from a file
func (fileSystem *FileSystem) ReadFile(filename string) ([]byte, error) {
	return ioutil.ReadFile(filename)
}

// Stat returns a FileInfo describing the named file.
func (fileSystem *FileSystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

// IsNotExist returns a boolean indicating whether the error is known to
// report that a file or directory does not exist.
func (fileSystem *FileSystem) IsNotExist(err error) bool {
	return os.IsNotExist(err)
}

// Deletes the file
func (fileSystem *FileSystem) DeleteFile(fileName string) error {
	return os.Remove(fileName)
}

// Creates a file when not found else appends it. Adds the content accordingly
func (fileSystem *FileSystem) AppendToFile(filename string, content string, perm os.FileMode) error {
	fileWriter, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, perm)
	if err != nil {
		err = fmt.Errorf("failed to open the file at %v: %v", filename, err)
	}

	if fileWriter.WriteString(content); err != nil {
		err = fmt.Errorf("failed to write contents to file")
	}
	defer fileWriter.Close()
	return err
}
