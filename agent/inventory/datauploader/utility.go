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
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/inventory/model"
	"github.com/aws/aws-sdk-go/service/ssm"
)

var (
	lock  sync.RWMutex
	store map[string]string
)

func init() {
	store = make(map[string]string)
}

// ShouldUpdate returns if given inventoryItem should be updated or not, if yes - then it also updates the checksum
// for future calls
func ShouldUpdate(inventoryItemName, data string) bool {
	//TODO: currently everything is in memory - we should have persistence for reboot scenarios as well
	//so that we can optimize PutInventory across reboots

	//TODO: abstract this into memory-vault kind of utility to make it consistent with our codebase.

	//TODO: We should not update checksum - before knowing the data has been uploaded to SSM - or else, if agent crashes
	//right after updating checksum but before uploading to SSM - agent would start getting contentHash mismatch exception.

	//TODO: should update - must not side effects - like updating checksum.

	//TODO: hash calculation should not be done here - since hash needs to be calculated and sent to SSM as per putinventory API call.

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

// ConvertToSSMInventoryItem converts given InventoryItem to []map[string]*string
func ConvertToSSMInventoryItem(item inventory.Item) (inventoryItem *ssm.InventoryItem, err error) {

	var a []interface{}
	var c map[string]*string
	var content = []map[string]*string{}
	var dataB []byte

	dataType := reflect.ValueOf(item.Content)

	switch dataType.Kind() {

	case reflect.Struct:
		//this should be converted to map[string]*string
		c = ConvertToMap(item.Content)
		content = append(content, c)

	case reflect.Array, reflect.Slice:
		//this should be converted to []map[string]*string
		dataB, _ = json.Marshal(item.Content)
		json.Unmarshal(dataB, &a)

		for _, v := range a {
			// convert each item to map[string]*string
			c = ConvertToMap(v)
			content = append(content, c)
		}

	default:
		//NOTE: collected inventory data is expected to be either a struct or an array
		err = fmt.Errorf("Unsupported data format - %v.", dataType.Kind())
		return
	}

	inventoryItem = &ssm.InventoryItem{
		CaptureTime:   &item.CaptureTime,
		TypeName:      &item.Name,
		SchemaVersion: &item.SchemaVersion,
		Content:       content,
	}

	return inventoryItem, nil
}

// ConvertToMap converts given object to map[string]*string
func ConvertToMap(input interface{}) (res map[string]*string) {
	var m map[string]interface{}
	b, _ := json.Marshal(input)
	json.Unmarshal(b, &m)

	res = make(map[string]*string)
	for k, v := range m {
		asString := toString(v)
		res[k] = &asString
	}
	return res
}

// toString converts given input to string
func toString(v interface{}) string {
	if v, isString := v.(string); isString {
		return v
	}
	b, _ := json.Marshal(v)
	return string(b)
}
