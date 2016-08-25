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

// Package datastore has utilites to read and write from long running plugins data-store

package datastore

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/plugin"
	"github.com/aws/amazon-ssm-agent/agent/platform"
)

// DataStore is the interface to provide utilities to read & write from a data store
type DataStore interface {
	Write(data map[string]plugin.PluginInfo) error
	Read() (map[string]plugin.PluginInfo, error)
}

var (
	dataModified bool
	lock         sync.RWMutex
	dataStore    map[string]plugin.PluginInfo
)

type FsStore struct{}

// Write overwrites long running plugins specific data back to data store (file system)
func (fs FsStore) Write(data map[string]plugin.PluginInfo) error {

	lock.Lock()
	defer lock.Unlock()

	var err error
	var s string

	//get data store location
	l, f := fs.getDataStoreLocation()

	//verify if parent folder exist
	if !fileutil.Exists(l) {
		if err = fileutil.MakeDirs(l); err != nil {
			return err
		}
	}

	if s, err = jsonutil.Marshal(data); err != nil {
		return err
	}

	//it's fine even if we overwrite the content of previous file
	if _, err = fileutil.WriteIntoFileWithPermissions(f, s, os.FileMode(int(appconfig.ReadWriteAccess))); err != nil {
		return err
	}

	dataModified = true
	return nil
}

// Read reads long running plugins data from data store (file system)
func (fs FsStore) Read() (map[string]plugin.PluginInfo, error) {

	var err error

	lock.RLock()
	defer lock.RUnlock()

	if dataStore == nil || dataModified {
		//read from disk to see if there were any long running plugins that were getting executed earlier
		dataStore, err = fs.load()
	}

	return dataStore, err
}

// load loads data from data-store (file system)
func (fs FsStore) load() (data map[string]plugin.PluginInfo, err error) {
	log.SetFlags(0)
	var content []byte
	data = make(map[string]plugin.PluginInfo)
	err = nil

	_, f := fs.getDataStoreLocation()

	if !fs.dataStoreFileExist() {
		log.Println(fmt.Sprintf("datastore file %s doesn't exist - no long running plugins to execute", f))
		return data, nil
	}

	if content, err = ioutil.ReadFile(f); err == nil {
		err = json.Unmarshal(content, &data)
	}

	return data, err
}

// dataStoreFileExist returns true if the dataStore file exists in the given location
func (fs FsStore) dataStoreFileExist() bool {
	_, f := fs.getDataStoreLocation()
	return fileutil.Exists(f)
}

// getDataStoreLocation returns the absolute path where long running plugins data-store is saved.
func (fs FsStore) getDataStoreLocation() (location, fileName string) {
	var instanceId string
	var err error

	if instanceId, err = platform.InstanceID(); err != nil {
		log.Println(fmt.Sprintf("error fetching the instanceID, %v", err))
		return
	}
	location = filepath.Join(appconfig.DefaultDataStorePath,
		instanceId,
		appconfig.LongRunningPluginsLocation,
		appconfig.LongRunningPluginDataStoreLocation)
	fileName = filepath.Join(appconfig.DefaultDataStorePath,
		instanceId,
		appconfig.LongRunningPluginsLocation,
		appconfig.LongRunningPluginDataStoreLocation,
		appconfig.LongRunningPluginDataStoreFileName)
	return
}
