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
	"reflect"

	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// ConvertToSSMInventoryItem converts given InventoryItem to []map[string]*string
func ConvertToSSMInventoryItem(item model.Item) (inventoryItem *ssm.InventoryItem, err error) {

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

		// If a is empty array, then content has to be empty array
		// instead of nil, as InventoryItem.Content has
		// to be empty array [] after serializing to Json,
		// based on the contract with ssm:PutInventory API.
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
