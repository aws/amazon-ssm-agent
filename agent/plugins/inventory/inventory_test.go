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

// Package inventory contains routines that periodically updates basic instance inventory to Inventory service
package inventory

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/mocks/context"
	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers"
	gatherers2 "github.com/aws/amazon-ssm-agent/agent/plugins/inventory/mocks/gatherers"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/assert"
)

// MockInventoryPlugin returns mock inventory plugin
func MockInventoryPlugin(supportedGatherers, installedGatherers []string) (*Plugin, error) {

	var p = Plugin{}

	//setting up mock context
	p.context = context.NewMockDefault()
	p.supportedGatherers = gatherers.SupportedGatherer{}
	p.installedGatherers = gatherers.InstalledGatherer{}

	//Creating supported gatherers
	for _, name := range supportedGatherers {
		p.supportedGatherers[name] = gatherers2.NewMockDefault()
	}

	//Creating installed gatherers
	for _, name := range installedGatherers {
		p.installedGatherers[name] = gatherers2.NewMockDefault()
	}

	return &p, nil
}

// NewInventoryPolicy returns inventory policy for given list of named gatherers
func NewInventoryPolicy(nameArr ...string) model.Policy {
	var p model.Policy
	//setup policy
	m := make(map[string]model.Config)

	for i := range nameArr {
		m[nameArr[i]] = model.Config{
			Collection: "Enabled",
		}
	}

	p.InventoryPolicy = m

	return p
}

func MockInventoryItems() (items []model.Item) {
	items = append(items, model.Item{
		Name:    "Fake:Name",
		Content: "Fake:Content",
	})
	return
}

func MockInventoryOptimizedItem() (items []*ssm.InventoryItem) {
	check1 := "AWS:File"
	SchemaVersion1 := "1.0"
	CaptureTime1 := "2020-05-22T19:32:34Z"
	ContentHashMock1 := LargeString(1024 * 1024)

	check2 := "AWS:Network"
	SchemaVersion2 := "1.0"
	CaptureTime2 := "2020-05-22T19:32:34Z"
	ContentHashMock2 := LargeString(1024 * 1024)

	items = append(items, &ssm.InventoryItem{
		TypeName:      &check1,
		ContentHash:   &ContentHashMock1,
		SchemaVersion: &SchemaVersion1,
		CaptureTime:   &CaptureTime1,
	})

	items = append(items, &ssm.InventoryItem{
		TypeName:      &check2,
		ContentHash:   &ContentHashMock2,
		SchemaVersion: &SchemaVersion2,
		CaptureTime:   &CaptureTime2,
	})

	return
}

func MockInventorySmallOptimizedItem() (items []*ssm.InventoryItem) {
	check1 := "AWS:File"
	SchemaVersion1 := "1.0"
	CaptureTime1 := "2020-05-22T19:32:34Z"
	ContentHashMock1 := LargeString(1024)

	check2 := "AWS:Network"
	SchemaVersion2 := "1.0"
	CaptureTime2 := "2020-05-22T19:32:34Z"
	ContentHashMock2 := LargeString(1024)

	items = append(items, &ssm.InventoryItem{
		TypeName:      &check1,
		ContentHash:   &ContentHashMock1,
		SchemaVersion: &SchemaVersion1,
		CaptureTime:   &CaptureTime1,
	})

	items = append(items, &ssm.InventoryItem{
		TypeName:      &check2,
		ContentHash:   &ContentHashMock2,
		SchemaVersion: &SchemaVersion2,
		CaptureTime:   &CaptureTime2,
	})

	return
}

func MockInventoryLargeFileItem() (items []*ssm.InventoryItem) {
	check1 := "AWS:File"
	SchemaVersion1 := "1.0"
	CaptureTime1 := "2020-05-22T19:32:34Z"
	ContentHashMock1 := LargeString(1024 * 1030)

	items = append(items, &ssm.InventoryItem{
		TypeName:      &check1,
		ContentHash:   &ContentHashMock1,
		SchemaVersion: &SchemaVersion1,
		CaptureTime:   &CaptureTime1,
	})

	return
}

// LargeString returns a string of length greater than the given input
func LargeString(sizeInBytes int) string {
	var dataB bytes.Buffer
	str := "VeryLargeStringVeryLargeStringVeryLargeStringVeryLargeStringVeryLargeStringVeryLargeStringVeryLargeStringVeryLargeString"

	for dataB.Len() <= sizeInBytes {
		dataB.WriteString(str)
	}

	return dataB.String()
}

// LargeInventoryItem returns a fairly large inventory Item
func LargeInventoryItem(sizeInBytes int) model.Item {
	return model.Item{
		Name:          "Fake:InventoryType",
		Content:       LargeString(sizeInBytes),
		SchemaVersion: "1.0",
	}
}

func TestRunGatherers(t *testing.T) {

	var err error
	var sGatherers, iGatherers []string
	var items []model.Item
	errorFreeGathererName := "ErrorFree-1"
	errorProneGathererName := "ErrorProne-1"

	//diff types of gatherers
	iGatherers = append(iGatherers, errorFreeGathererName, errorProneGathererName)
	sGatherers = iGatherers

	//setup

	//mock inventory plugin
	p, _ := MockInventoryPlugin(sGatherers, iGatherers)

	//mock errorFree gatherer
	errorFreeGatherer := gatherers2.NewMockDefault()
	errorProneGatherer := gatherers2.NewMockDefault()

	//mock Config for gatherers
	config := model.Config{
		Collection: "Enabled",
	}

	//mock inventory items
	data := MockInventoryItems()

	//setting up configs of gatherers
	testGathererConfig := make(map[gatherers.T]model.Config)
	testGathererConfig[errorFreeGatherer] = config

	//TESTING
	//testing running a gatherer which doesn't throw any error

	//set expectations for errorFree gatherer.
	errorFreeGatherer.On("Name").Return(errorFreeGathererName)
	errorFreeGatherer.On("Run", p.context, config).Return(data, nil)
	items, err = p.RunGatherers(testGathererConfig)

	assert.Nil(t, err, "%v shouldn't throw errors", errorFreeGatherer)
	assert.NotEqual(t, 0, len(items), "%v is expected to return at least few inventory items", errorFreeGatherer)

	//testing running multiple gatherers out of which one throws an error

	//adding error prone gatherer to list of executors
	testGathererConfig[errorProneGatherer] = config

	//set expectations for errorProne gatherer.
	errorProneGatherer.On("Name").Return(errorProneGathererName)
	e := fmt.Errorf("Fake error executing %v", errorProneGatherer)
	errorProneGatherer.On("Run", p.context, config).Return(data, e)
	items, err = p.RunGatherers(testGathererConfig)

	assert.NotNil(t, err, "%v should throw errors", errorProneGatherer)
}

func TestVerifyInventoryDataSize(t *testing.T) {
	var smallItem, largeItem model.Item
	var items []model.Item
	var result bool
	var gatherers []string

	gatherers = append(gatherers, "RandomGatherer")

	//setup
	//mock inventory plugin
	p, _ := MockInventoryPlugin(gatherers, gatherers)

	//small inventory item
	items = MockInventoryItems()
	smallItem = items[0]
	largeItem = LargeInventoryItem(1024 * 10240)

	//TESTING
	//testing normal scenario when both item and items are within size limits
	items = MockInventoryItems()
	result = p.VerifyInventoryDataSize(smallItem, items)

	assert.Equal(t, true, result, "Expected to return true when both item and items are within size limits")

	//testing when size of 1 item is small enough but total size exceeds the limit
	items = append(items, largeItem)
	result = p.VerifyInventoryDataSize(smallItem, items)

	assert.Equal(t, false, result, "Expected to return false when items size is greater than the limit")
}

// Test to verify splitting of putInventory calls when inventoryItem content is >1MB
// expected flagTest return value should be true
func TestVerifyPutInventoryCall(t *testing.T) {

	var gatherers []string

	gatherers = append(gatherers, "RandomGatherer")

	p, _ := MockInventoryPlugin(gatherers, gatherers)

	itemIndex := -1
	itemIndex, _ = p.getLargeItemIndex(MockInventoryOptimizedItem(), "AWS:File")

	assert.NotEqual(t, -1, itemIndex)
}

// Test to verify splitting of putInventory calls when inventoryItem content is <1MB
// expected flagTest return value should be false
func TestVerifyNoPutInventoryCall(t *testing.T) {

	var gatherers []string

	gatherers = append(gatherers, "RandomGatherer")

	p, _ := MockInventoryPlugin(gatherers, gatherers)

	itemIndex := -1
	itemIndex, _ = p.getLargeItemIndex(MockInventorySmallOptimizedItem(), "AWS:File")

	assert.Equal(t, -1, itemIndex)
}

// Test to verify when there's only one collected inventoryItem
// expected flagTest return value should be false as it should go through default putInventory behavior
func TestVerifyOneItemNoPutInventoryCall(t *testing.T) {

	var gatherers []string

	gatherers = append(gatherers, "RandomGatherer")

	p, _ := MockInventoryPlugin(gatherers, gatherers)

	itemIndex := -1
	itemIndex, _ = p.getLargeItemIndex(MockInventoryLargeFileItem(), "AWS:File")

	assert.Equal(t, -1, itemIndex)
}

func TestSplitItemsList(t *testing.T) {
	var optimizedNewInventoryItemsList, nonOptimizedNewInventoryItemsList []*ssm.InventoryItem
	nonOptimizedInventoryItems := MockInventorySmallOptimizedItem()
	optimizedInventoryItems := MockInventorySmallOptimizedItem()

	// before split length of the list will be all inventory items
	assert.Equal(t, len(nonOptimizedInventoryItems), 2)
	assert.Equal(t, len(optimizedInventoryItems), 2)

	nonOptimizedNewInventoryItemsList, optimizedNewInventoryItemsList, nonOptimizedInventoryItems, optimizedInventoryItems =
		extractFileItems(MockInventorySmallOptimizedItem(), MockInventorySmallOptimizedItem(), 0)

	// after split total length should be same as before.
	assert.Equal(t, len(nonOptimizedNewInventoryItemsList)+len(nonOptimizedInventoryItems), 2)
	assert.Equal(t, len(optimizedNewInventoryItemsList)+len(optimizedInventoryItems), 2)

}

func TestPlugin_IsMulitpleAssociationPresent(t *testing.T) {
	var gatherers []string

	gatherers = append(gatherers, "RandomGatherer")

	//setup
	//mock inventory plugin
	p, _ := MockInventoryPlugin(gatherers, gatherers)
	config := contracts.Configuration{
		CurrentAssociations: []string{"testAssociationID", "testAssociationID2"},
	}
	status, other := p.IsMulitpleAssociationPresent("testAssociationID", config)
	assert.True(t, status)
	assert.Equal(t, "testAssociationID2", other)
}

func TestShouldRetryWithNonOptimizedData(t *testing.T) {
	type testCase struct {
		err         error
		shouldRetry bool
	}
	var testCases = []testCase{
		{awserr.New("ItemContentMismatchException", "content hash does not match", nil), true},
		{awserr.New("InvalidItemContentException", "invalid content", nil), true},
		{awserr.New("ItemSizeLimitExceededException", "size limit exceeded", nil), false},
		{fmt.Errorf("SomeOtherError"), false},
	}

	log := log.NewMockLog()
	for _, testCase := range testCases {
		assert.Equal(t, testCase.shouldRetry, shouldRetryWithNonOptimizedData(testCase.err, log))
	}
}
