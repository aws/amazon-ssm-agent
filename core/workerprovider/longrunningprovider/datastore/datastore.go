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

// Package datastore provides interface to read and write json data from/to disk
package datastore

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider/datastore/filesystem"
)

const (
	ReadWriteAccess        = 0600
	ReadWriteExecuteAccess = 0700
)

type IStore interface {
	Write(datajson string, path string, name string) error
	Read(name string, dest interface{}) error
}

type LocalFileStore struct {
	fileSystem filesystem.IFileSystem
	log        log.T
	lock       sync.RWMutex
}

// NewLocalFileStore returns a local file store
func NewLocalFileStore(log log.T) *LocalFileStore {
	return &LocalFileStore{
		fileSystem: filesystem.NewFileSystem(),
		log:        log,
	}
}

// Write writes data into disk
func (localFileStore *LocalFileStore) Write(datajson string, path string, filename string) error {
	localFileStore.lock.Lock()
	defer localFileStore.lock.Unlock()

	if exist, _ := localFileStore.exists(path); !exist {
		if err := localFileStore.createPath(path); err != nil {
			return err
		}
	}

	if err := localFileStore.writeIntoFileWithPermission(filename, []byte(datajson)); err != nil {
		return err
	}

	return nil
}

// Read reads data from disk
func (localFileStore *LocalFileStore) Read(filename string, dest interface{}) error {
	localFileStore.lock.RLock()
	defer localFileStore.lock.RUnlock()

	content, err := localFileStore.load(filename)
	if err != nil {
		return err
	}

	localFileStore.log.Debugf("worker process: %s" + string(content))

	return json.Unmarshal(content, &dest)
}

// exists returns true if the given file/directory exists, otherwise return false.
func (localFileStore *LocalFileStore) exists(name string) (bool, error) {
	_, err := localFileStore.fileSystem.Stat(name)
	if err == nil {
		return true, nil
	}
	if localFileStore.fileSystem.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// createPath makes directory with ReadWriteExecuteAccess
func (localFileStore *LocalFileStore) createPath(path string) error {
	err := localFileStore.fileSystem.MkdirAll(path, ReadWriteExecuteAccess)
	if err != nil {
		err = fmt.Errorf("failed to create directory %v. %v", path, err)
	}
	return err
}

// writeIntoFileWithPermission writes data in a file
func (localFileStore *LocalFileStore) writeIntoFileWithPermission(filename string, data []byte) error {
	return localFileStore.fileSystem.WriteFile(filename, data, os.FileMode(int(ReadWriteAccess)))
}

// load loads data from a file
func (localFileStore *LocalFileStore) load(filename string) ([]byte, error) {

	exist, err := localFileStore.exists(filename)
	if !exist {
		return []byte{}, fmt.Errorf("file doesn't exist %s", filename)
	}

	result, err := localFileStore.fileSystem.ReadFile(filename)
	if err != nil {
		return []byte{}, err
	}

	return result, err
}
