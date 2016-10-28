// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package inventory contains routines that periodically updates basic instance inventory to Inventory service
package inventory

import (
	"crypto/md5"
	"encoding/base64"
	"sync"
)

var (
	lock  sync.RWMutex
	store map[string]string
)

const (
	// AgentStatus is agent's status which is sent as an instance information inventory data
	AgentStatus = "Active"
)

func init() {
	store = make(map[string]string)
}

// ShouldUpdate returns if given inventoryItem should be updated or not, if yes - then it also updates the checksum for future calls
func ShouldUpdate(inventoryItemName, data string) bool {
	//TODO: currently everything is in memory - we should have persistence for reboot scenarios as well
	//so that we can optimize PutInventory across reboots

	//TODO: abstract this into memory-vault kind of utility to make it consistent with our codebase.

	//calculate new checksum of given data
	sum := md5.Sum([]byte(data))
	newCheckSum := base64.StdEncoding.EncodeToString(sum[:])

	lock.Lock()
	defer lock.Unlock()

	oldChecksum, isInventoryItemPresent := store[inventoryItemName]
	if isInventoryItemPresent && newCheckSum == oldChecksum {
		//no need to update given inventory data
		return false
	}

	store[inventoryItemName] = newCheckSum
	return true
}
