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

// Package datauploader contains routines upload inventory data to SSM - Inventory service
package datauploader

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
)

var (
	lock             sync.RWMutex
	contentHashStore map[string]string
)

//TODO: add unit tests

// decoupling platform.InstanceID for easy testability
var machineIDProvider = machineInfoProvider

func machineInfoProvider() (name string, err error) {
	return platform.InstanceID()
}

// Optimizer defines operations of content optimizer which inventory plugin makes use of
type Optimizer interface {
	UpdateContentHash(inventoryItemName, hash string) (err error)
	GetContentHash(inventoryItemName string) (hash string)
}

// Impl implements content hash optimizations for inventory plugin
type Impl struct {
	log      log.T
	location string //where the content hash data is persisted in file-systems
}

func NewOptimizerImpl(context context.T) (*Impl, error) {
	return NewOptimizerImplWithLocation(context.Log(), appconfig.InventoryRootDirName, appconfig.InventoryContentHashFileName)
}

func NewOptimizerImplWithLocation(log log.T, rootDir string, fileName string) (*Impl, error) {
	var optimizer = Impl{}
	var machineID, content string
	var err error

	optimizer.log = log

	//get machineID - return if not able to detect machineID
	if machineID, err = machineIDProvider(); err != nil {
		err = fmt.Errorf("Unable to detect machineID because of %v - this will hamper execution of inventory plugin",
			err.Error())
		return &optimizer, err
	}

	optimizer.location = filepath.Join(appconfig.DefaultDataStorePath,
		machineID,
		rootDir,
		fileName)

	contentHashStore = make(map[string]string)

	//read old content hash values from file
	if fileutil.Exists(optimizer.location) {
		optimizer.log.Debugf("Found older set of content hash used by inventory plugin - %v", optimizer.location)

		//read file
		if content, err = fileutil.ReadAllText(optimizer.location); err == nil {
			optimizer.log.Debugf("Found older set of content hash used by inventory plugin at %v - \n%v",
				optimizer.location,
				content)

			if err = json.Unmarshal([]byte(content), &contentHashStore); err != nil {
				optimizer.log.Debugf("Unable to read content hash store of inventory plugin - thereby ignoring any older values")
			}
		}
	}

	return &optimizer, nil
}

func (i *Impl) UpdateContentHash(inventoryItemName, hash string) (err error) {
	lock.Lock()
	defer lock.Unlock()

	contentHashStore[inventoryItemName] = hash

	//persist the data in file system
	dataB, _ := json.Marshal(contentHashStore)

	if _, err = fileutil.WriteIntoFileWithPermissions(i.location, string(dataB), appconfig.ReadWriteAccess); err != nil {
		err = fmt.Errorf("Unable to update content hash in file - %v because - %v", i.location, err.Error())
		return
	}

	return
}

func (i *Impl) GetContentHash(inventoryItemName string) (hash string) {
	lock.RLock()
	defer lock.RUnlock()

	var found bool

	if hash, found = contentHashStore[inventoryItemName]; !found {
		// return empty string - if there is no content hash for given inventory data type
		hash = ""
	}

	return
}
