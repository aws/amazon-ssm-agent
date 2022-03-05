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

//Package fileutil contains utilities for working with the file system.

package fileutil

import (
	"io/ioutil"
	"os"
)

type IosFS interface {
	IsNotExist(err error) bool
	MkdirAll(path string, perm os.FileMode) error
	Open(name string) (*os.File, error)
	Remove(name string) error
	Rename(oldpath string, newpath string) error
	Stat(name string) (os.FileInfo, error)
	WriteFile(filename string, data []byte, perm os.FileMode) error
}

// osFS implements IosFS using the local disk.
type osFS struct{}

// IsNotExist returns a boolean indicating whether the error is known to report that
// a file or directory does not exist. It is satisfied by ErrNotExist as well as some syscall errors
func (osFS) IsNotExist(err error) bool {
	return os.IsNotExist(err)
}

// Mkdir creates a new directory with the specified name and permission bits (before umask).
func (osFS) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// Open opens the named file for reading. If successful, methods on the returned file can be used for reading;
// the associated file descriptor has mode O_RDONLY.
func (osFS) Open(name string) (*os.File, error) {
	return os.Open(name)
}

// Stat returns the FileInfo structure describing file.
func (osFS) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

// RemoveAll removes path and any children it contains. It removes everything it can but returns the
// first error it encounters. If the path does not exist, RemoveAll returns nil (no error).
func (osFS) Remove(name string) error {
	return os.Remove(name)
}

// Rename renames (moves) oldpath to newpath. If newpath already exists and is not a directory,
// Rename replaces it. OS-specific restrictions may apply when oldpath and newpath are in different directories.
func (osFS) Rename(oldpath string, newpath string) error {
	return os.Rename(oldpath, newpath)
}

// WriteFile writes data to a file named by filename. If the file does not exist,
// WriteFile creates it with permissions perm; otherwise WriteFile truncates it before writing.
func (osFS) WriteFile(filename string, data []byte, perm os.FileMode) error {
	return ioutil.WriteFile(filename, data, perm)
}
