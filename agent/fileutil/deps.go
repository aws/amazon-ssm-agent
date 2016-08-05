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

//Package fileutil contains utilities for working with the file system.
package fileutil

import (
	"io"
	"io/ioutil"
	"os"
)

var fs fileSystem = osFS{}
var ioUtil ioUtility = ioU{}

type fileSystem interface {
	IsNotExist(err error) bool
	MkdirAll(path string, perm os.FileMode) error
	Open(name string) (ioFile, error)
	Remove(name string) error
	Rename(oldpath string, newpath string) error
	Stat(name string) (os.FileInfo, error)
}

// osFS implements fileSystem using the local disk.
type osFS struct{}

func (osFS) IsNotExist(err error) bool                    { return os.IsNotExist(err) }
func (osFS) MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }
func (osFS) Open(name string) (ioFile, error)             { return os.Open(name) }
func (osFS) Stat(name string) (os.FileInfo, error)        { return os.Stat(name) }
func (osFS) Remove(name string) error                     { return os.Remove(name) }
func (osFS) Rename(oldpath string, newpath string) error  { return os.Rename(oldpath, newpath) }

type ioFile interface {
	io.Closer
	io.Reader
	io.ReaderAt
	io.Seeker
	Stat() (os.FileInfo, error)
}

type ioUtility interface {
	WriteFile(filename string, data []byte, perm os.FileMode) error
}

type ioU struct{}

func (ioU) WriteFile(filename string, data []byte, perm os.FileMode) error {
	return ioutil.WriteFile(filename, data, perm)
}
