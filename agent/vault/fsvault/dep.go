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

package fsvault

import (
	"io/ioutil"
	"os"

	"encoding/json"

	"github.com/aws/amazon-ssm-agent/agent/fileutil"
)

var fs fileSystem = &fsvFileSystem{}

type fileSystem interface {
	Exists(path string) bool
	MakeDirs(path string) (err error)
	RecursivelyHarden(path string) error
	ReadFile(path string) ([]byte, error)
	Remove(path string) error
	HardenedWriteFile(path string, data []byte) (err error)
}

type fsvFileSystem struct{}

func (fsvFileSystem) Exists(path string) bool              { return fileutil.Exists(path) }
func (fsvFileSystem) MakeDirs(path string) error           { return fileutil.MakeDirs(path) }
func (fsvFileSystem) RecursivelyHarden(path string) error  { return fileutil.RecursivelyHarden(path) }
func (fsvFileSystem) ReadFile(path string) ([]byte, error) { return ioutil.ReadFile(path) }
func (fsvFileSystem) Remove(path string) error             { return os.Remove(path) }
func (fsvFileSystem) HardenedWriteFile(path string, data []byte) error {
	return fileutil.HardenedWriteFile(path, data)
}

var jh jsonHandler = &fsvJsonHandler{}

type jsonHandler interface {
	Marshal(v interface{}) ([]byte, error)
	Unmarshal(data []byte, v interface{}) error
}

type fsvJsonHandler struct{}

func (fsvJsonHandler) Marshal(v interface{}) ([]byte, error)      { return json.Marshal(v) }
func (fsvJsonHandler) Unmarshal(data []byte, v interface{}) error { return json.Unmarshal(data, v) }
