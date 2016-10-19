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
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/inventory/model"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/assert"
)

func MockInventoryUploader() *InventoryUploader {
	var uploader InventoryUploader

	uploader.isOptimizerEnabled = false
	return &uploader
}

func FakeInventoryItems(count int) (items []model.Item) {
	i := 0

	for i < count {
		items = append(items, model.Item{
			Name:          "RandomInventoryItem",
			Content:       FakeStructForTesting(),
			SchemaVersion: "1.0",
		})
		i++
	}

	return
}

//TODO: add unit tests for ShouldUpdate scenario once content hash is implemented

func TestConvertToSsmInventoryItems(t *testing.T) {

	var items []model.Item
	var inventoryItems []*ssm.InventoryItem
	var err error

	c := context.NewMockDefault()
	u := MockInventoryUploader()

	//testing positive scenario

	//setting up inventory.Item
	items = append(items, FakeInventoryItems(2)...)
	inventoryItems, err = u.ConvertToSsmInventoryItems(c, items)

	assert.Nil(t, err, "Error shouldn't be thrown for multiple inventory items")
	assert.Equal(t, len(items), len(inventoryItems), "Count of inventory items should be equal to input")

	//testing negative scenario
	item := model.Item{
		Name:    "RandomInvalidInventoryType",
		Content: "RandomStringIsNotSupported",
	}
	items = append(items, item)

	inventoryItems, err = u.ConvertToSsmInventoryItems(c, items)
	assert.NotNil(t, err, "Error should be thrown for unsupported Item.Content")
}
