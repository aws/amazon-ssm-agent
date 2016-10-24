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

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/inventory/gatherers"
	"github.com/aws/amazon-ssm-agent/agent/inventory/model"
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
		p.supportedGatherers[name] = gatherers.NewMockDefault()
	}

	//Creating installed gatherers
	for _, name := range installedGatherers {
		p.installedGatherers[name] = gatherers.NewMockDefault()
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
	errorFreeGatherer := gatherers.NewMockDefault()
	errorProneGatherer := gatherers.NewMockDefault()

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
	largeItem = LargeInventoryItem(1024 * 1024)

	//TESTING
	//testing normal scenario when both item and items are within size limits
	items = MockInventoryItems()
	result = p.VerifyInventoryDataSize(smallItem, items)

	assert.Equal(t, true, result, "Expected to return true when both item and items are within size limits")

	//testing when size of 1 item is small enough but total size exceeds the limit
	items = append(items, largeItem)
	result = p.VerifyInventoryDataSize(smallItem, items)

	assert.Equal(t, false, result, "Expected to return false when items size is greater than 1024")
}
