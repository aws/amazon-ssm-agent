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

// Package custom contains a gatherer for collecting custom inventory items
package custom

import (
	"testing"

	"encoding/json"
	"os"
	"time"

	"errors"
	"fmt"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/stretchr/testify/assert"
)

type MockFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

var mockMachineIDProvider = func() (string, error) { return "i-12345678", nil }

func (m MockFileInfo) Name() string {
	return m.name
}

func (m MockFileInfo) Size() int64 {
	return m.size
}

func (m MockFileInfo) Mode() os.FileMode {
	return m.mode
}

func (m MockFileInfo) ModTime() time.Time {
	return m.modTime
}

func (m MockFileInfo) IsDir() bool {
	return m.isDir
}

func (m MockFileInfo) Sys() interface{} {
	return nil
}

func MockCustomInventoryItem() model.CustomInventoryItem {

	var attributes = map[string]interface{}{
		"Name": "filename",
		"Size": "100",
		"Time": "2016-10-10T01:01:01Z",
	}

	return model.CustomInventoryItem{
		TypeName:      "Custom:MyFile",
		SchemaVersion: "1.0",
		Content:       attributes,
	}
}

func MockItemNoTypeName() model.CustomInventoryItem {

	var attributes = map[string]interface{}{
		"WebServer":  "corp.amazing.com",
		"PortNumber": "8081",
		"Version":    "1.2.3.4",
	}
	return model.CustomInventoryItem{
		SchemaVersion: "1.0",
		Content:       attributes,
	}
}

func MockItemLongTypeName() model.CustomInventoryItem {

	var attributes = map[string]interface{}{
		"WebServer":  "corp.amazing.com",
		"PortNumber": "8081",
		"Version":    "1.2.3.4",
	}
	return model.CustomInventoryItem{
		TypeName:      strings.Repeat("a", TypeNameLengthLimit+1),
		SchemaVersion: "1.0",
		Content:       attributes,
	}
}

func MockItemTypeNameInvalidPrefix() model.CustomInventoryItem {
	var attributes = map[string]interface{}{
		"WebServer":  "corp.amazing.com",
		"PortNumber": "8081",
		"Version":    "1.2.3.4",
	}
	return model.CustomInventoryItem{
		TypeName:      "Test:123",
		SchemaVersion: "1.0",
		Content:       attributes,
	}
}

func MockItemMissedSchemaVersion() model.CustomInventoryItem {
	var attributes = map[string]interface{}{
		"WebServer":  "corp.amazing.com",
		"PortNumber": "8081",
		"Version":    "1.2.3.4",
	}
	return model.CustomInventoryItem{
		TypeName: "Custom:123",
		Content:  attributes,
	}
}

func MockItemInvalidSchemaVersion() model.CustomInventoryItem {
	var attributes = map[string]interface{}{
		"WebServer":  "corp.amazing.com",
		"PortNumber": "8081",
		"Version":    "1.2.3.4",
	}
	return model.CustomInventoryItem{
		TypeName:      "Custom:123",
		SchemaVersion: "123",
		Content:       attributes,
	}
}

func MockItemNoContent() model.CustomInventoryItem {
	return model.CustomInventoryItem{
		TypeName:      "Custom:123",
		SchemaVersion: "1.1",
	}
}

func MockItemContentIsArray() model.CustomInventoryItem {
	var attributes1 = map[string]interface{}{
		"WebServer":  "corp.amazing.com",
		"PortNumber": "8081",
		"Version":    "1.2.3.4",
	}
	var attributes2 = map[string]interface{}{
		"WebServer":  "corp.amazing.com",
		"PortNumber": "8080",
		"Version":    "1.2.3.5",
	}
	var entries = []map[string]interface{}{
		attributes1, attributes2,
	}
	return model.CustomInventoryItem{
		TypeName:      "Custom:123",
		SchemaVersion: "1.0",
		Content:       entries,
	}
}

func MockItemContentAttributeCountExceedLimit() model.CustomInventoryItem {
	var attributes = map[string]interface{}{
		"WebServer":  "corp.amazing.com",
		"PortNumber": "8081",
		"Version":    "1.2.3.4",
	}
	for i := 0; i < AttributeCountLimit; i++ {
		attributeName := fmt.Sprintf("attr%d", i)
		attributes[attributeName] = "1"
	}
	return model.CustomInventoryItem{
		TypeName:      "Custom:123",
		SchemaVersion: "1.0",
		Content:       attributes,
	}
}

func MockItemEmptyAttributeName() model.CustomInventoryItem {
	var attributes = map[string]interface{}{
		"":           "corp.amazing.com",
		"PortNumber": "8081",
		"Version":    "1.2.3.4",
	}
	return model.CustomInventoryItem{
		TypeName:      "Custom:123",
		SchemaVersion: "1.0",
		Content:       attributes,
	}
}

func MockItemLongAttributeName() model.CustomInventoryItem {
	longAttrName := strings.Repeat("a", AttributeNameLengthLimit+1)
	var attributes = map[string]interface{}{
		longAttrName: "corp.amazing.com",
		"PortNumber": "8081",
		"Version":    "1.2.3.4",
	}
	return model.CustomInventoryItem{
		TypeName:      "Custom:123",
		SchemaVersion: "1.0",
		Content:       attributes,
	}
}

func MockItemIntTypeAttributeValue() model.CustomInventoryItem {
	var attributes = map[string]interface{}{
		"PortNumber": "8081",
		"Version":    "1.2.3.4",
		"MyName":     1234,
	}
	return model.CustomInventoryItem{
		TypeName:      "Custom:123",
		SchemaVersion: "1.0",
		Content:       attributes,
	}
}

func MockItemLongAttributeValue() model.CustomInventoryItem {
	longAttrValue := strings.Repeat("a", AttributeValueLengthLimit+1)
	var attributes = map[string]interface{}{
		"PortNumber": "",
		"Version":    "1.2.3.4",
		"MyName":     longAttrValue,
	}
	return model.CustomInventoryItem{
		TypeName:      "Custom:123",
		SchemaVersion: "1.0",
		Content:       attributes,
	}
}

func MockReadFile(filename string) ([]byte, error) {
	return json.Marshal(MockCustomInventoryItem())
}

func MockReadFileAccessDenied(filename string) ([]byte, error) {
	return nil, errors.New("Access Denied")
}

func MockReadFileInvalidJSON(filename string) ([]byte, error) {
	return []byte("{Invalid JSON}"), nil
}

func MockReadFileNoTypeName(filename string) ([]byte, error) {
	return json.Marshal(MockItemNoTypeName())
}

func MockReadFileLongTypeName(filename string) ([]byte, error) {
	return json.Marshal(MockItemLongTypeName())
}

func MockReadFileTypeNameInvalidPrefix(filename string) ([]byte, error) {
	return json.Marshal(MockItemTypeNameInvalidPrefix())
}

func MockReadFileNoSchemaVersion(filename string) ([]byte, error) {
	return json.Marshal(MockItemMissedSchemaVersion())
}

func MockReadFileInvalidSchemaVersion(filename string) ([]byte, error) {
	return json.Marshal(MockItemInvalidSchemaVersion())
}

func MockReadFileNoContentProperty(filename string) ([]byte, error) {
	return json.Marshal(MockItemNoContent())
}

func MockReadFileContentIsArray(filename string) ([]byte, error) {
	return json.Marshal(MockItemContentIsArray())
}

func MockReadFileContentAttributeCountExceedLimit(filename string) ([]byte, error) {
	return json.Marshal(MockItemContentAttributeCountExceedLimit())
}

func MockReadFileContentEmptyAttributeName(filename string) ([]byte, error) {
	return json.Marshal(MockItemEmptyAttributeName())
}

func MockReadFileContentLongAttributeName(filename string) ([]byte, error) {
	return json.Marshal(MockItemLongAttributeName())
}

func MockReadFileContentNonStringTypeAttributeValue(filename string) ([]byte, error) {
	return json.Marshal(MockItemIntTypeAttributeValue())
}

func MockReadFileContentLongAttributeValue(filename string) ([]byte, error) {
	return json.Marshal(MockItemLongAttributeValue())
}

func MockReadDir(dirname string) (files []os.FileInfo, err error) {
	mfi := MockFileInfo{
		name:    "abc.json",
		size:    1024,
		mode:    0,
		modTime: time.Now(),
		isDir:   true,
	}
	files = append(files, mfi)
	return
}

func MockReadDirNotExist(dirname string) (files []os.FileInfo, err error) {
	err = errors.New("Folder " + dirname + " does not exist.")
	return
}

func MockReadDirCustomInventoryFileCountExceed(dirname string) (files []os.FileInfo, err error) {
	for i := 0; i < CustomInventoryCountLimit+1; i++ {
		filename := fmt.Sprintf("abc%d.json", i)
		files = append(files, MockFileInfo{
			name:    filename,
			size:    1024,
			mode:    0,
			modTime: time.Now(),
			isDir:   true,
		})
	}
	return
}

func MockReadDirDuplicateType(dirname string) (files []os.FileInfo, err error) {
	for i := 0; i < 2; i++ {
		filename := fmt.Sprintf("abc%d.json", i)
		files = append(files, MockFileInfo{
			name:    filename,
			size:    1024,
			mode:    0,
			modTime: time.Now(),
			isDir:   true,
		})
	}
	return
}

func TestGatherer(t *testing.T) {
	c := context.NewMockDefault()
	g := Gatherer(c)
	readFileFunc = MockReadFile
	readDirFunc = MockReadDir
	machineIDProvider = mockMachineIDProvider

	items, err := g.Run(c, model.Config{})
	assert.Nil(t, err, "Unexpected error thrown")
	assert.Equal(t, 1, len(items), "Custom Gather should return 1 inventory type data.")
}

func TestReadFileAccessDenied(t *testing.T) {
	c := context.NewMockDefault()
	g := Gatherer(c)
	readFileFunc = MockReadFileAccessDenied
	readDirFunc = MockReadDir
	machineIDProvider = mockMachineIDProvider

	items, err := g.Run(c, model.Config{})
	assert.Nil(t, err, "err shoud be nil as gather continues to load other custom inventory files")
	assert.Nil(t, items, "Items should be nil")
}

func TestCustomInventoryDirNotExist(t *testing.T) {
	c := context.NewMockDefault()
	g := Gatherer(c)
	readFileFunc = MockReadFile
	readDirFunc = MockReadDirNotExist
	machineIDProvider = mockMachineIDProvider

	items, err := g.Run(c, model.Config{})
	assert.Nil(t, err, "err shoud be nil as we should ignore if folder does not exist")
	assert.Nil(t, items, "Items should be nil")
}

func TestCustomInventoryCountExceed(t *testing.T) {
	c := context.NewMockDefault()
	g := Gatherer(c)
	readFileFunc = MockReadFile
	readDirFunc = MockReadDirCustomInventoryFileCountExceed
	machineIDProvider = mockMachineIDProvider

	items, err := g.Run(c, model.Config{})
	assert.NotNil(t, err, "err shoud be nil as we should ignore if folder does not exist")
	assert.Contains(t, err.Error(), "Total custom inventory file count")
	assert.Nil(t, items, "Items should be nil")
}

func TestDuplicateTypeName(t *testing.T) {
	c := context.NewMockDefault()
	g := Gatherer(c)
	readFileFunc = MockReadFile
	readDirFunc = MockReadDirDuplicateType
	machineIDProvider = mockMachineIDProvider

	items, err := g.Run(c, model.Config{})
	assert.Nil(t, err, "err shoud be nil")
	assert.NotNil(t, items, "Items should not be nil")
}

func TestInvalidJson(t *testing.T) {
	c := context.NewMockDefault()
	g := Gatherer(c)
	readFileFunc = MockReadFileInvalidJSON
	readDirFunc = MockReadDir
	machineIDProvider = mockMachineIDProvider

	items, err := g.Run(c, model.Config{})
	assert.Nil(t, err, "err shoud be nil as gather continues to load other custom inventory files")
	assert.Nil(t, items, "Items should be nil")
}

func TestMissingTypeName(t *testing.T) {
	c := context.NewMockDefault()
	g := Gatherer(c)
	readFileFunc = MockReadFileNoTypeName
	readDirFunc = MockReadDir
	machineIDProvider = mockMachineIDProvider

	items, err := g.Run(c, model.Config{})
	assert.Nil(t, err, "err shoud be nil as gather continues to load other custom inventory files")
	assert.Nil(t, items, "Items should be nil")
}

func TestLongTypeName(t *testing.T) {
	c := context.NewMockDefault()
	g := Gatherer(c)
	readFileFunc = MockReadFileLongTypeName
	readDirFunc = MockReadDir
	machineIDProvider = mockMachineIDProvider

	items, err := g.Run(c, model.Config{})
	assert.Nil(t, err, "err shoud be nil as gather continues to load other custom inventory files")
	assert.Nil(t, items, "Items should be nil")
}

func TestTypeNameInvalidPrefix(t *testing.T) {
	c := context.NewMockDefault()
	g := Gatherer(c)
	readFileFunc = MockReadFileTypeNameInvalidPrefix
	readDirFunc = MockReadDir
	machineIDProvider = mockMachineIDProvider

	items, err := g.Run(c, model.Config{})
	assert.Nil(t, err, "err shoud be nil as gather continues to load other custom inventory files")
	assert.Nil(t, items, "Items should be nil")
}

func TestMissingSchemaVersion(t *testing.T) {
	c := context.NewMockDefault()
	g := Gatherer(c)
	readFileFunc = MockReadFileNoSchemaVersion
	readDirFunc = MockReadDir
	machineIDProvider = mockMachineIDProvider

	items, err := g.Run(c, model.Config{})
	assert.Nil(t, err, "err shoud be nil as gather continues to load other custom inventory files")
	assert.Nil(t, items, "Items should be nil")
}

func TestInvalidSchemaVersion(t *testing.T) {
	c := context.NewMockDefault()
	g := Gatherer(c)
	readFileFunc = MockReadFileInvalidSchemaVersion
	readDirFunc = MockReadDir
	machineIDProvider = mockMachineIDProvider

	items, err := g.Run(c, model.Config{})
	assert.Nil(t, err, "err shoud be nil as gather continues to load other custom inventory files")
	assert.Nil(t, items, "Items should be nil")
}

func TestMissingContent(t *testing.T) {
	c := context.NewMockDefault()
	g := Gatherer(c)
	readFileFunc = MockReadFileNoContentProperty
	readDirFunc = MockReadDir
	machineIDProvider = mockMachineIDProvider

	items, err := g.Run(c, model.Config{})
	assert.Nil(t, err, "err shoud be nil as gather continues to load other custom inventory files")
	assert.Nil(t, items, "Items should be nil")
}

func TestContentIsArray(t *testing.T) {
	c := context.NewMockDefault()
	g := Gatherer(c)
	readFileFunc = MockReadFileContentIsArray
	readDirFunc = MockReadDir
	machineIDProvider = mockMachineIDProvider

	items, err := g.Run(c, model.Config{})
	assert.Nil(t, err, "err shoud be nil as gather continues to load other custom inventory files")
	assert.Equal(t, 1, len(items), "Custom Gather should return 1 inventory type data.")
	entries, ok := items[0].Content.([]map[string]interface{})
	assert.Equal(t, ok, true)
	assert.Equal(t, 2, len(entries), "Custom Gather should return 2 entries.")
}

func TestContentAttributeCountExceedLimit(t *testing.T) {
	c := context.NewMockDefault()
	g := Gatherer(c)
	readFileFunc = MockReadFileContentAttributeCountExceedLimit
	readDirFunc = MockReadDir
	machineIDProvider = mockMachineIDProvider

	items, err := g.Run(c, model.Config{})
	assert.Nil(t, err, "err shoud be nil as gather continues to load other custom inventory files")
	assert.Nil(t, items, "Items should be nil")
}

func TestContentEmptyAttributeName(t *testing.T) {
	c := context.NewMockDefault()
	g := Gatherer(c)
	readFileFunc = MockReadFileContentEmptyAttributeName
	readDirFunc = MockReadDir
	machineIDProvider = mockMachineIDProvider

	items, err := g.Run(c, model.Config{})
	assert.Nil(t, err, "err shoud be nil as gather continues to load other custom inventory files")
	assert.Nil(t, items, "Items should be nil")
}

func TestContentLongAttributeName(t *testing.T) {
	c := context.NewMockDefault()
	g := Gatherer(c)
	readFileFunc = MockReadFileContentLongAttributeName
	readDirFunc = MockReadDir
	machineIDProvider = mockMachineIDProvider

	items, err := g.Run(c, model.Config{})
	assert.Nil(t, err, "err shoud be nil as gather continues to load other custom inventory files")
	assert.Nil(t, items, "Items should be nil")
}

func TestContentNonStringTypeAttributeValue(t *testing.T) {
	c := context.NewMockDefault()
	g := Gatherer(c)
	readFileFunc = MockReadFileContentNonStringTypeAttributeValue
	readDirFunc = MockReadDir
	machineIDProvider = mockMachineIDProvider

	items, err := g.Run(c, model.Config{})
	assert.Nil(t, err, "err shoud be nil as gather continues to load other custom inventory files")
	assert.Nil(t, items, "Items should be nil")
}

func TestContentLongAttributeValue(t *testing.T) {
	c := context.NewMockDefault()
	g := Gatherer(c)
	readFileFunc = MockReadFileContentLongAttributeValue
	readDirFunc = MockReadDir
	machineIDProvider = mockMachineIDProvider

	items, err := g.Run(c, model.Config{})
	assert.Nil(t, err, "err shoud be nil as gather continues to load other custom inventory files")
	assert.Nil(t, items, "Items should be nil")
}
