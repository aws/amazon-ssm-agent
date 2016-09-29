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

// Package custom contains a gatherer for collection custom inventory items.

package custom

import (
	"regexp"
	"time"

	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"reflect"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/inventory/model"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

//TODO: add unit tests.

const (
	// Gatherer Name
	GathererName = "CustomInventory"
	// Custom inventory file extension
	FileSuffix = ".json"
	// Custom inventory typename prefix
	CustomInventoryTypeNamePrefix = "CUSTOM:"
	// Custom inventory typename length limit
	TypeNameLengthLimit = 32
	// Custom inventory item size limit
	CustomInventorySizeLimitBytes = 16 * 1024 // 16 KB
	// Custom inventory type count limit
	CustomInventoryCountLimit = 20
	// Custom inventory entry's attribute count limit
	AttributeCountLimit = 50
	// Custom inventory entry's attribute name length limit
	AttributeNameLengthLimit = 64
	// Custom inventory entry's attribute value length limit
	AttributeValueLengthLimit = 1024
)

type T struct{}

func Gatherer(_ context.T) (*T, error) {
	return new(T), nil
}

func (t *T) Name() string {
	return GathererName
}

func (t *T) Run(context context.T, configuration inventory.Config) (items []inventory.Item, err error) {

	var log = context.Log()

	// Get custom inventory folder, fall back if not specified
	customFolder := configuration.Location
	if customFolder == "" {
		customFolder = appconfig.DefaultCustomInventoryFolder
	}

	// Get custom inventory files' path
	fileList, err := getFilePaths(log, customFolder, FileSuffix)
	if err != nil {
		log.Errorf("Failed to get inventory files from folder %v, error %v", customFolder, err)
		return
	}

	// Get custom inventory item
	setTypeName := make(map[string]bool)
	for _, filePath := range fileList {

		if customItem, err := getItemFromFile(log, filePath); err == nil {

			if _, ok := setTypeName[customItem.Name]; ok {
				err = log.Errorf("Custom inventory typeName (%v) from file (%v) already exists,"+
					" i.e., other file under the same folder contains the same typeName,"+
					" please remove duplicate custom inventory file.",
					customItem.Name, filePath)
				return items, err
			}
			setTypeName[customItem.Name] = true
			items = append(items, customItem)

		} else {

			log.Errorf("Failed to get item from file %v, error %v. continue...", filePath, err)
			continue

		}
	}

	count := len(items)
	log.Debugf("Count of custom inventory items : %v.", count)
	if count == 0 {
		log.Infof("No custom inventory item found under folder: %v", customFolder)
	}
	return
}

func (t *T) RequestStop(stopType contracts.StopType) error {
	//TODO: set a stop flag so Run thread would stop when flag is set to true
	var err error
	return err
}

// getItemFromFile Reads one custom inventory file
func getItemFromFile(log log.T, file string) (result inventory.Item, err error) {

	var content []byte
	content, err = ioutil.ReadFile(file)
	if err != nil {
		log.Errorf("Failed to read file: %v, error: %v", file, err)
		return
	}

	result, err = validateSchema(log, content)
	if err != nil {
		log.Errorf("Failed to validate the schema of file (%v), error: %v",
			file, err)
	}
	return
}

// validateSchema Validates custom inventory content's schema
func validateSchema(log log.T, content []byte) (item inventory.Item, err error) {

	var customInventoryItem inventory.CustomInventoryItem

	// Deserialize custom inventory item content
	if err = json.Unmarshal(content, &customInventoryItem); err != nil {
		log.Error(err)
		return
	}

	if err = validateTypeName(log, customInventoryItem); err != nil {
		return
	}

	if err = validateSchemaVersion(log, customInventoryItem); err != nil {
		return
	}

	var attributes map[string]interface{}
	if attributes, err = validateContentEntrySchema(log, customInventoryItem); err != nil {
		return
	}

	// CaptureTime must be in UTC so that formatting to RFC3339
	// Example: 2016-07-30T18:15:37Z
	currentTime := time.Now().UTC()
	captureTime := currentTime.Format(time.RFC3339)

	// Convert content into array
	var entryArray = []map[string]interface{}{}
	if len(attributes) > 0 {
		entryArray = append(entryArray, attributes)
		entryArray[0] = attributes
	}
	item = inventory.Item{
		Name:          customInventoryItem.TypeName,
		SchemaVersion: customInventoryItem.SchemaVersion,
		Content:       entryArray,
		CaptureTime:   captureTime,
	}
	return
}

// validateTypeName validates custom inventory item TypeName
func validateTypeName(log log.T, customInventoryItem inventory.CustomInventoryItem) (err error) {
	typeName := customInventoryItem.TypeName
	typeNameLength := len(typeName)
	if typeNameLength == 0 {
		err = log.Error("Custom inventory item has missed TypeName")
		return
	}
	if typeNameLength > TypeNameLengthLimit {
		err = log.Errorf("Custom inventory item TypeName (%v) exceeded length limit (%v)",
			typeName, typeNameLength)
		return
	}
	// validate TypeName prefix
	if !strings.HasPrefix(customInventoryItem.TypeName, CustomInventoryTypeNamePrefix) {
		err = log.Errorf("Custom inventory item's TypeName (%v) has to start with %v",
			customInventoryItem.TypeName, CustomInventoryTypeNamePrefix)
	}
	return
}

// validateContentEntrySchema validates custom inventory item SchemaVersion
func validateSchemaVersion(log log.T, customInventoryItem inventory.CustomInventoryItem) (err error) {
	schemaVersion := customInventoryItem.SchemaVersion
	if len(schemaVersion) == 0 {
		err = log.Error("Custom inventory item has missed SchemaVersion")
		return
	}

	//validate schema version format
	var validSchemaVersion = regexp.MustCompile(`^([0-9]{1,6})(\.[0-9]{1,6})$`)
	if !validSchemaVersion.MatchString(schemaVersion) {
		err = log.Errorf("Custom inventory item (%v) has invalid SchemaVersion (%v),"+
			" the valid schema version has to be like 1.0, 1.1, 2.0, 3.9, etc.",
			customInventoryItem.TypeName, schemaVersion)
	}
	return
}

// validateContentEntrySchema validates attribute name and value
func validateContentEntrySchema(log log.T, customInventoryItem inventory.CustomInventoryItem) (
	attributes map[string]interface{},
	err error) {

	if customInventoryItem.Content == nil {
		err = log.Error("Custom inventory item missed Content property.")
		return
	}

	contentValue := customInventoryItem.Content
	log.Debugf("Content type of %v: %v", customInventoryItem.TypeName, reflect.TypeOf(contentValue))
	var ok bool
	if attributes, ok = contentValue.(map[string]interface{}); !ok {
		err = log.Errorf("Custom inventory item %v's Content is not a valid json",
			customInventoryItem.TypeName)
		return
	}
	if attributes == nil {
		err = log.Errorf("Custom inventory item %v's Content is not a valid json",
			customInventoryItem.TypeName)
		return
	}
	if len(attributes) > AttributeCountLimit {
		err = log.Errorf("Custom inventory item content cannot have more than %v attributes",
			AttributeCountLimit)
		return
	}
	for a, v := range attributes {
		aLen := len(a)
		if aLen > AttributeNameLengthLimit {
			err = log.Errorf("Custom inventory item key length (%v) exceeded limit (%v)",
				aLen, AttributeNameLengthLimit)
			return
		}

		if vStr, ok := v.(string); ok {
			vLen := len(vStr)
			if vLen > AttributeValueLengthLimit {
				err = log.Errorf("custom inventory item value length (%v) exceeded limit (%v)",
					vLen, AttributeValueLengthLimit)
				return
			}
		} else {
			err = log.Errorf("Attribute (%v)'s value (%v) is not a string type. It's type is (%v)",
				a, v, reflect.TypeOf(v))
			return
		}
	}
	return
}

// getFilePaths reads all files with specified suffix under the given folder
func getFilePaths(log log.T, folder string, fileSuffix string) (fileFullPathList []string, err error) {

	var totalSize int64
	var totalSizeLimit int64 = CustomInventorySizeLimitBytes

	// Read all files that ended with json
	files, readDirError := ioutil.ReadDir(folder)
	if readDirError != nil {
		log.Errorf("Read directory %v failed, error: %v", folder, readDirError)
		return nil, readDirError
	}

	for _, f := range files {

		if filepath.Ext(f.Name()) == fileSuffix {

			fileFullPath := filepath.Join(folder, f.Name())
			fileFullPath = filepath.Clean(fileFullPath)
			fileFullPathList = append(fileFullPathList, fileFullPath)
			totalSize += f.Size()
			if totalSize > totalSizeLimit {
				err = log.Errorf("Total size (%v) exceed limit (%v)", totalSize, totalSizeLimit)
				return nil, err
			}
		}
	}

	// Check custom inventory file count
	if len(fileFullPathList) > CustomInventoryCountLimit {
		err = log.Errorf("Total custom inventory file count (%v) exceed limit (%v)",
			len(fileFullPathList), CustomInventoryCountLimit)
		return nil, err
	}

	log.Debugf("Total custom inventory file count %v", len(fileFullPathList))
	return
}
