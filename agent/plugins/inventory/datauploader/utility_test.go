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
	"reflect"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/assert"
)

type FakeStruct struct {
	FakeStringKey string
	FakeIntKey    int
}

type StructForTesting struct {
	FakeString  string
	FakeInt     int
	FakeBool    bool
	DummyStruct FakeStruct
}

func FakeStructData() FakeStruct {
	return FakeStruct{
		FakeStringKey: "FakeString",
		FakeIntKey:    1,
	}
}

func FakeStructForTesting() StructForTesting {
	return StructForTesting{
		FakeString:  "FakeString",
		FakeInt:     100,
		FakeBool:    true,
		DummyStruct: FakeStructData(),
	}
}

func TestConvertToMap(t *testing.T) {

	//setup
	singleEntry := FakeStructForTesting()

	//testing conversion of struct to string
	afterConversion := ConvertToMap(singleEntry)

	assert.True(t, reflect.ValueOf(afterConversion).Kind() == reflect.Map, "ConvertToMap must return a map")
	assert.True(t, len(afterConversion) == reflect.TypeOf(singleEntry).NumField(), "ConvertToMap must maintain the struct fields")
}

func TestConvertToSSMInventoryItem(t *testing.T) {

	var item model.Item
	var err error
	var dataAfterConversion *ssm.InventoryItem

	//testing with Item.Content being a struct
	item = model.Item{
		Name:          "RandomInventoryItem",
		Content:       FakeStructForTesting(),
		SchemaVersion: "1.0",
	}

	//setting up expectations
	dataAfterConversion, err = ConvertToSSMInventoryItem(item)
	assert.Nil(t, err, "Shouldn't throw errors for Item.Content being a struct")
	assert.Equal(t, 1, len(dataAfterConversion.Content), "ssm.InventoryItem.Content should have only 1 entry")

	//testing with Item.Content being an array
	//setting up data
	var multipleEntries []StructForTesting
	multipleEntries = append(multipleEntries, FakeStructForTesting(), FakeStructForTesting())

	item = model.Item{
		Name:          "RandomInventoryItem",
		Content:       multipleEntries,
		SchemaVersion: "1.0",
	}

	dataAfterConversion, err = ConvertToSSMInventoryItem(item)
	assert.Nil(t, err, "Shouldn't throw errors for Item.Content being a slice/array")
	assert.Equal(t, len(multipleEntries), len(dataAfterConversion.Content), "ssm.InventoryItem.Content should have only 1 entry")

	//testing with Item.Content with being a non-struct and non-slice data type
	item = model.Item{
		Name:          "RandomInventoryItem",
		Content:       "NotSupportedContent",
		SchemaVersion: "1.0",
	}

	dataAfterConversion, err = ConvertToSSMInventoryItem(item)
	assert.NotNil(t, err, "Should throw errors for Item.Content not being a struct or an array or slice")
}
