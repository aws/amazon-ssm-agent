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
	"errors"
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
	// Omit Epoch, InstalledTime and URL which are omitempty
	// Include CompType which should be omitted in all cases
	items = append(items, model.Item{
		Name: "RandomInventoryItem",
		Content: model.ApplicationData{
			Name:            "Test1",
			Publisher:       "Pub1",
			Release:         "1",
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
	assert.Equal(t, "{\"Name\":\"Test1\",\"Publisher\":\"Pub1\",\"Version\":\"\",\"Release\":\"1\",\"ApplicationType\":\"Foo\",\"Architecture\":\"Brutalism\"}", string(bytes[:]))
}

func TestGetDirtySsmInventoryItems_empty(t *testing.T) {
	var items []model.Item
	var dirtyInventoryItems []*ssm.InventoryItem
	var err error

	c := context.NewMockDefault()
	u := MockInventoryUploader()

	//setting up inventory.Item
	dirtyInventoryItems, err = u.GetDirtySsmInventoryItems(c, items)

	assert.Nil(t, err, "Error shouldn't be thrown if there's no inventory items are given to check")
	assert.Equal(t, len(dirtyInventoryItems), 0, "Dirty inventory items should be empty when no inventory items are given to check")
}

func TestGetDirtySsmInventoryItems_dirtyItemFound(t *testing.T) {
	var items []model.Item
	var dirtyInventoryItems []*ssm.InventoryItem
	var err error

	c := context.NewMockDefault()
	u := MockInventoryUploader()

	//setting up inventory.Item
	items = append(items, ApplicationInventoryItem()...)
	dirtyInventoryItems, err = u.GetDirtySsmInventoryItems(c, items)

	assert.Nil(t, err, "Error shouldn't be thrown if there's an dirty inventory item found")
	assert.Equal(t, len(dirtyInventoryItems), 1, "Dirty inventory item found")
}

// Mock stands for a mocked service.
type MockSSMCaller struct {
	mock.Mock
}

func NewMockSSMCaller() *MockSSMCaller {
	service := new(MockSSMCaller)
	return service
}

func (m *MockSSMCaller) PutInventory(input *ssm.PutInventoryInput) (output *ssm.PutInventoryOutput, err error) {
	args := m.Called(input)
	return args.Get(0).(*ssm.PutInventoryOutput), args.Error(1)
}

func TestGetRandomBackOffTime(t *testing.T) {
	c := context.NewMockDefault()
	instanceID := "i-12345678"
	backoffTime := getRandomBackOffTime(c, instanceID)
	assert.Equal(t, backoffTime <= Max_Time_TO_Back_Off, true)

	instanceID = "i-MockID"
	backoffTime = getRandomBackOffTime(c, instanceID)
	assert.Equal(t, backoffTime <= Max_Time_TO_Back_Off, true)
}

func TestSendDataToSSM(t *testing.T) {
	testSendData(t, true)
	testSendData(t, false)
}

func testSendData(t *testing.T, putInventorySucceeds bool) {

	var items []model.Item
	var inventoryItems []*ssm.InventoryItem

	//setting up inventory.Item
	item := ApplicationInventoryItem()[0]
	items = append(items, item)

	inventoryItem, _ := ConvertToSSMInventoryItem(item)
	hash := "aHash"
	inventoryItem.ContentHash = &hash
	inventoryItems = append(inventoryItems, inventoryItem)

	// create mocks and setup expectations
	machineIDProvider = func() (string, error) { return "i-12345678", nil }
	mockSSM := NewMockSSMCaller()
	output := &ssm.PutInventoryOutput{}
	if putInventorySucceeds {
		mockSSM.On("PutInventory", mock.AnythingOfType("*ssm.PutInventoryInput")).Return(output, nil)
	} else {
		err := errors.New("some error")
		mockSSM.On("PutInventory", mock.AnythingOfType("*ssm.PutInventoryInput")).Return(output, err)
	}

	mockOptimizer := NewMockDefault()
	if putInventorySucceeds {
		for _, item := range inventoryItems {
			mockOptimizer.On("UpdateContentHash", *item.TypeName, hash).Return(nil)
		}
	}

	c := context.NewMockDefault()

	// call method
	u := &InventoryUploader{
		ssm:       mockSSM,
		optimizer: mockOptimizer,
	}
	u.SendDataToSSM(c, inventoryItems)

	// assert that the expectations were met
	mockSSM.AssertExpectations(t)
	mockOptimizer.AssertExpectations(t)
}
