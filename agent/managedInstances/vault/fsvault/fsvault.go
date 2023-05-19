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
	lock                      sync.RWMutex
	manifest                  map[string]string = make(map[string]string)
	initialized               bool              = false
	initializedManifestPrefix string            = ""
	vaultFolderPath           string            = filepath.Join(appconfig.DefaultDataStorePath, "Vault")
	manifestFileNameSuffix    string            = "Manifest"
	storeFolderPath           string            = filepath.Join(vaultFolderPath, "Store")
)

// Store data.
func Store(manifestFileNamePrefix string, key string, data []byte) (err error) {
	lock.Lock()
	defer lock.Unlock()

	if err = ensureInitialized(manifestFileNamePrefix); err != nil {
		return
	}

	p := filepath.Join(storeFolderPath, key)

	if err = fs.HardenedWriteFile(p, []byte(data)); err != nil {
		return fmt.Errorf("failed to write data file for %s. %v\n", key, err)
	}

	manifest[key] = p
	if err = saveManifest(manifestFileNamePrefix); err != nil {
		delete(manifest, key)
		return fmt.Errorf("failed to save manifest when storing %s. %v\n", key, err)
	}

	return
}

func IsManifestExists(manifestFileNamePrefix string) bool {
	isInitialized := initializedManifestPrefix == manifestFileNamePrefix && len(manifest) != 0
	return isInitialized || fs.Exists(getManifestPath(manifestFileNamePrefix))
}

// Retrieve data.
func Retrieve(manifestFileNamePrefix string, key string) (data []byte, err error) {
	lock.Lock()
	defer lock.Unlock()

	if err = ensureInitialized(manifestFileNamePrefix); err != nil {
		return
	}

	p := manifest[key] // path to the stored value

	if p == "" {
		return nil, fmt.Errorf("%s does not exist", key)
	}

	if !fs.Exists(p) {
		return nil, fmt.Errorf("data file of %s is missing", key)
	}

	if data, err = fs.ReadFile(p); err != nil {
		return nil, fmt.Errorf("failed to read data file for %s. %v", key, err)
	}

	return
}

// Remove data.
func Remove(manifestFileNamePrefix string, key string) (err error) {
	lock.Lock()
	defer lock.Unlock()

	if err = ensureInitialized(manifestFileNamePrefix); err != nil {
		return
	}

	if _, ok := manifest[key]; !ok {
		return
	}

	bkpKey := key
	bkpData := manifest[key]
	delete(manifest, key)
	if err = saveManifest(manifestFileNamePrefix); err != nil {
		manifest[bkpKey] = bkpData
		err = fmt.Errorf("failed to save manifest when removing %s. %v\n", key, err)
		return
	}

	if err = fs.Remove(filepath.Join(storeFolderPath, key)); err != nil {
		err = fmt.Errorf("failed to remove value file for %s. %v", key, err)
		return
	}

	return
}

// ensureInitialized hardens the folders and files on start. Having this outside
// of init() allows us to override filesystem interface for testing.
var ensureInitialized = func(manifestFileNamePrefix string) (err error) {

	if initialized && initializedManifestPrefix == manifestFileNamePrefix {
		return
	}

	// store folder is under vault folder, creating the deepest folder and
	// harden the top-level one.
	if err = fs.MakeDirs(storeFolderPath); err != nil {
		return fmt.Errorf("failed to create vault folder. %v", err)
	}

	// vault contains sensitive data, we have to make sure each of them are set
	// with correct permission. In Linux, setting permission for folders does
	// not guarantee child files' permission. In Windows, even though permission
	// inheritance is default, it can be turned off.
	if err = fs.RecursivelyHarden(vaultFolderPath); err != nil {
		return fmt.Errorf("failed to set permission for vault folder or its content. %v", err)
	}

	// initialize manifest file
	manifestFilePath := getManifestPath(manifestFileNamePrefix)
	if fs.Exists(manifestFilePath) {
		var content []byte
		var err error
		if content, err = fs.ReadFile(manifestFilePath); err != nil {
			return fmt.Errorf("failed to load vault from file system. %v", err)
		}
		if err = jh.Unmarshal(content, &manifest); err != nil {
			return fmt.Errorf("failed to unmarshal vault manifest. %v", err)
		}
	}

	initialized = true
	initializedManifestPrefix = manifestFileNamePrefix
	return nil
}

// saveManifest to file system.
var saveManifest = func(manifestFileNamePrefix string) (err error) {
	var data []byte
	if data, err = jh.Marshal(manifest); err != nil {
		return fmt.Errorf("failed to marshal manifest. %v", err)
	}

	if err = fs.HardenedWriteFile(getManifestPath(manifestFileNamePrefix), data); err != nil {
		return fmt.Errorf("failed to save manifest with hardened permission. %v", err)
	}
	return
}

func getManifestPath(manifestFileNamePrefix string) string {
	manifestFileName := fmt.Sprintf("%s%s", manifestFileNamePrefix, manifestFileNameSuffix)
	return filepath.Join(vaultFolderPath, manifestFileName)
}
