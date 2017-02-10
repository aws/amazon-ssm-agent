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
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func MockInventoryUploader() *InventoryUploader {
	var uploader InventoryUploader

	optimizer := NewMockDefault()
	optimizer.On("GetContentHash", mock.AnythingOfType("string")).Return("RandomInventoryItem")
	optimizer.On("UpdateContentHash", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)

	uploader.optimizer = optimizer
	return &uploader
}

func FakeInventoryItems(count int) (items []model.Item) {
	i := 0

	for i < count {
		items = append(items, model.Item{
			Name:          "RandomInventoryItem",
			Content:       FakeStructForTesting(),
			SchemaVersion: "1.0",
			CaptureTime:   "time",
		})
		i++
	}

	return
}

func ApplicationInventoryItem() (items []model.Item) {
	// Omit Version which is not omitempty
	// Omit InstalledTime and URL which are omitempty
	// Include CompType which should be omitted in all cases
	items = append(items, model.Item{
		Name: "RandomInventoryItem",
		Content: model.ApplicationData{
			Name:            "Test1",
			Publisher:       "Pub1",
			ApplicationType: "Foo",
			Architecture:    "Brutalism",
			CompType:        model.AWSComponent,
		},
		SchemaVersion: "1.0",
		CaptureTime:   "time",
	})

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
	inventoryItems, _, err = u.ConvertToSsmInventoryItems(c, items)

	assert.Nil(t, err, "Error shouldn't be thrown for multiple inventory items")
	assert.Equal(t, len(items), len(inventoryItems), "Count of inventory items should be equal to input")

	//testing negative scenario
	item := model.Item{
		Name:    "RandomInvalidInventoryType",
		Content: "RandomStringIsNotSupported",
	}
	items = append(items, item)

	inventoryItems, _, err = u.ConvertToSsmInventoryItems(c, items)
	assert.NotNil(t, err, "Error should be thrown for unsupported Item.Content")
}

func TestConvertExcludedAndEmptyToSsmInventoryItems(t *testing.T) {

	var items []model.Item
	var inventoryItems []*ssm.InventoryItem
	var err error

	c := context.NewMockDefault()
	u := MockInventoryUploader()

	//testing positive scenario

	//setting up inventory.Item
	items = append(items, ApplicationInventoryItem()...)
	inventoryItems, _, err = u.ConvertToSsmInventoryItems(c, items)

	assert.Nil(t, err, "Error shouldn't be thrown for application inventory item")
	assert.Equal(t, len(items), len(inventoryItems), "Count of inventory items should be equal to input")

	bytes, err := json.Marshal(items[0].Content)
	assert.Nil(t, err, "Error shouldn't be thrown when marshalling content")
	// CompType not present even though it has value.  Version should be present even though it doesn't.  InstallTime and Url should not be present because they have no value.
	assert.Equal(t, "{\"Name\":\"Test1\",\"Publisher\":\"Pub1\",\"Version\":\"\",\"ApplicationType\":\"Foo\",\"Architecture\":\"Brutalism\"}", string(bytes[:]))
}
