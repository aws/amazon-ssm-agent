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

// Package fsvault implements vault with file system storage.
package fsvault

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
)

var (
	lock             sync.RWMutex
	manifest         map[string]string = make(map[string]string)
	initialized      bool              = false
	vaultFolderPath  string            = filepath.Join(appconfig.DefaultDataStorePath, "Vault")
	manifestFilePath string            = filepath.Join(vaultFolderPath, "Manifest")
	storeFolderPath  string            = filepath.Join(vaultFolderPath, "Store")
)

// Store data.
func Store(key string, data []byte) (err error) {

	lock.Lock()
	defer lock.Unlock()

	if err = ensureInitialized(); err != nil {
		return
	}

	p := filepath.Join(storeFolderPath, key)

	if err = fs.HardenedWriteFile(p, []byte(data)); err != nil {
		return fmt.Errorf("Failed to write data file for %s. %v\n", key, err)
	}

	manifest[key] = p
	if err = saveManifest(); err != nil {
		delete(manifest, key)
		return fmt.Errorf("Failed to save manifest when storing %s. %v\n", key, err)
	}

	return
}

// Retrieve data.
func Retrieve(key string) (data []byte, err error) {

	lock.Lock()
	defer lock.Unlock()

	if err = ensureInitialized(); err != nil {
		return
	}

	p := manifest[key] // path to the stored value

	if p == "" {
		return nil, fmt.Errorf("%s does not exist.", key)
	}

	if !fs.Exists(p) {
		return nil, fmt.Errorf("Data file of %s is missing.", key)
	}

	if data, err = fs.ReadFile(p); err != nil {
		return nil, fmt.Errorf("Failed to read data file for %s. %v", key, err)
	}

	return
}

// Remove data.
func Remove(key string) (err error) {

	lock.Lock()
	defer lock.Unlock()

	if err = ensureInitialized(); err != nil {
		return
	}

	if _, ok := manifest[key]; !ok {
		return
	}

	bkpKey := key
	bkpData := manifest[key]
	delete(manifest, key)
	if err = saveManifest(); err != nil {
		manifest[bkpKey] = bkpData
		err = fmt.Errorf("Failed to save manifest when removing %s. %v\n", key, err)
		return
	}

	if err = fs.Remove(filepath.Join(storeFolderPath, key)); err != nil {
		err = fmt.Errorf("Failed to remove value file for %s. %v", key, err)
		return
	}

	return
}

// ensureInitialized hardens the folders and files on start. Having this outside
// of init() allows us to override filesystem interface for testing.
var ensureInitialized = func() (err error) {

	if initialized {
		return
	}

	// store folder is under vault folder, creating the deepest folder and
	// harden the top-level one.
	if err = fs.MakeDirs(storeFolderPath); err != nil {
		return fmt.Errorf("Failed to create vault folder. %v", err)
	}

	// vault contains sensitive data, we have to make sure each of them are set
	// with correct permission. In Linux, setting permission for folders does
	// not guarantee child files' permission. In Windows, even though permission
	// inheritance is default, it can be turned off.
	if err = fs.RecursivelyHarden(vaultFolderPath); err != nil {
		return fmt.Errorf("Failed to set permission for vault folder or its content. %v", err)
	}

	// initialize manifest file
	if fs.Exists(manifestFilePath) {
		var content []byte
		var err error
		if content, err = fs.ReadFile(manifestFilePath); err != nil {
			return fmt.Errorf("Failed to load vault from file system. %v", err)
		}
		if err = jh.Unmarshal(content, &manifest); err != nil {
			return fmt.Errorf("Failed to unmarshal vault manifest. %v", err)
		}
	}

	initialized = true
	return nil
}

// saveManifest to file system.
var saveManifest = func() (err error) {
	var data []byte
	if data, err = jh.Marshal(manifest); err != nil {
		return fmt.Errorf("Failed to marshal manifest. %v", err)
	}

	if err = fs.HardenedWriteFile(manifestFilePath, data); err != nil {
		return fmt.Errorf("Failed to save manifest with hardened permission. %v", err)
	}
	return
}
