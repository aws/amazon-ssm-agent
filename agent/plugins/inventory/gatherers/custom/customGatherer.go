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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
)

const (
	// GathererName captures name of custom gatherer
	GathererName = "CustomInventory"
	// FileSuffix represents custom inventory file extension
	FileSuffix = ".json"
	// CustomInventoryTypeNamePrefix represents custom inventory typename prefix
	CustomInventoryTypeNamePrefix = "Custom:"
	// TypeNameLengthLimit represents custom inventory typename length limit
	TypeNameLengthLimit = 100
	// CustomInventoryCountLimit represents custom inventory type count limit
	CustomInventoryCountLimit = 20
	// AttributeCountLimit represents custom inventory entry's attribute count limit
	AttributeCountLimit = 50
	// AttributeNameLengthLimit represents custom inventory entry's attribute name length limit
	AttributeNameLengthLimit = 64
	// AttributeValueLengthLimit represents custom inventory entry's attribute value length limit
	AttributeValueLengthLimit = 4096
)

// T represents custom gatherer
type T struct{}

// Gatherer returns a new custom gatherer
func Gatherer(context context.T) *T {
	return new(T)
}

// Name returns name of custom gatherer
func (t *T) Name() string {
	return GathererName
}

// decoupling for easy testability
var readDirFunc = ReadDir
var readFileFunc = ReadFile
var machineIDProvider = machineInfoProvider

// ReadDir is a wrapper on ioutil.ReadDir for easy testability
func ReadDir(dirname string) ([]os.FileInfo, error) {
	return ioutil.ReadDir(dirname)
}

// ReadFile is a wrapper on ioutil.ReadFile for easy testability
func ReadFile(filename string) ([]byte, error) {
	return ioutil.ReadFile(filename)
}

func machineInfoProvider() (name string, err error) {
	return platform.InstanceID()
}

// LogError is a wrapper on log.Error for easy testability
func LogError(log log.T, err error) {
	// To debug unit test, please uncomment following line
	// fmt.Println(err)
	log.Error(err)
}

// Run executes custom gatherer and returns list of inventory.Item
func (t *T) Run(context context.T, configuration model.Config) (items []model.Item, err error) {

	log := context.Log()

	// Get custom inventory folder, fall back if not specified
	customFolder := configuration.Location
	if customFolder == "" {
		var machineID string

		//get machineID - return if not able to detect machineID
		if machineID, err = machineIDProvider(); err != nil {
			log.Infof("Unable to detect machineID because of %v", err.Error())
			log.Infof("Custom gatherer's location will be agent's execution location")
		} else {
			customFolder = filepath.Join(appconfig.DefaultDataStorePath,
				machineID,
				appconfig.InventoryRootDirName,
				appconfig.CustomInventoryRootDirName)
		}
	}

	// Get custom inventory files' path
	fileList, err := getFilePaths(log, customFolder, FileSuffix)
	if err != nil {
		LogError(
			log,
			fmt.Errorf("Failed to get inventory files from folder %v, error %v", customFolder, err))
		return
	}

	// Get custom inventory item
	setTypeName := make(map[string]bool)
	for _, filePath := range fileList {

		if customItem, err := getItemFromFile(log, filePath); err == nil {

			if _, ok := setTypeName[customItem.Name]; ok {
				err = fmt.Errorf("Custom inventory typeName (%v) from file (%v) already exists,"+
					" i.e., other file under the same folder contains the same typeName,"+
					" please remove duplicate custom inventory file.",
					customItem.Name, filePath)
				LogError(log, err)
			} else {
				// Only append if current TypeName is not duplicate
				setTypeName[customItem.Name] = true
				items = append(items, customItem)
			}
		} else {
			LogError(log,
				fmt.Errorf("Failed to get item from file %v, error %v. continue...", filePath, err))
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

// RequestStop stops the execution of custom gatherer
func (t *T) RequestStop(stopType contracts.StopType) error {
	//TODO: set a stop flag so Run thread would stop when flag is set to true
	var err error
	return err
}

// getItemFromFile Reads one custom inventory file
func getItemFromFile(log log.T, file string) (result model.Item, err error) {

	var content []byte
	content, err = readFileFunc(file)
	if err != nil {
		LogError(log, fmt.Errorf("Failed to read file: %v, error: %v", file, err))
		return
	}

	result, err = convertToItem(log, content)
	if err != nil {
		LogError(log, fmt.Errorf("Failed to convert file (%v) to inventory item, error: %v",
			file, err))
	}
	return
}

// convertToItem Validates custom inventory content's schema and convert to inventory.Item
func convertToItem(log log.T, content []byte) (item model.Item, err error) {

	var customInventoryItem model.CustomInventoryItem

	// Deserialize custom inventory item content
	if err = json.Unmarshal(content, &customInventoryItem); err != nil {
		LogError(log, err)
		return
	}

	if err = validateTypeName(log, customInventoryItem); err != nil {
		return
	}

	if err = validateSchemaVersion(log, customInventoryItem); err != nil {
		return
	}

	var entryArray []map[string]interface{}
	if entryArray, err = validateContentEntrySchema(log, customInventoryItem); err != nil {
		return
	}

	// CaptureTime must be in UTC so that formatting to RFC3339
	// Example: 2016-07-30T18:15:37Z
	currentTime := time.Now().UTC()
	captureTime := currentTime.Format(time.RFC3339)

	item = model.Item{
		Name:          customInventoryItem.TypeName,
		SchemaVersion: customInventoryItem.SchemaVersion,
		Content:       entryArray,
		CaptureTime:   captureTime,
	}
	return
}

// validateTypeName validates custom inventory item TypeName
func validateTypeName(log log.T, customInventoryItem model.CustomInventoryItem) (err error) {
	typeName := customInventoryItem.TypeName
	typeNameLength := len(typeName)
	if typeNameLength == 0 {
		err = errors.New("Custom inventory item has missed or empty TypeName")
		LogError(log, err)
		return
	} else if typeNameLength > TypeNameLengthLimit {
		err = fmt.Errorf("Custom inventory item TypeName (%v)'s length %v exceeded the limit: %v",
			typeName,
			typeNameLength,
			TypeNameLengthLimit)
		LogError(log, err)
		return
	}

	// validate TypeName prefix
	if !strings.HasPrefix(customInventoryItem.TypeName, CustomInventoryTypeNamePrefix) {
		err = fmt.Errorf("Custom inventory item's TypeName (%v) has to start with %v",
			customInventoryItem.TypeName, CustomInventoryTypeNamePrefix)
		LogError(log, err)
	}
	return
}

// validateContentEntrySchema validates custom inventory item SchemaVersion
func validateSchemaVersion(log log.T, customInventoryItem model.CustomInventoryItem) (err error) {
	schemaVersion := customInventoryItem.SchemaVersion
	if len(schemaVersion) == 0 {
		err = errors.New("Custom inventory item has missed SchemaVersion")
		LogError(log, err)
		return
	}

	//validate schema version format
	var validSchemaVersion = regexp.MustCompile(`^([0-9]{1,6})(\.[0-9]{1,6})$`)
	if !validSchemaVersion.MatchString(schemaVersion) {
		err = fmt.Errorf("Custom inventory item (%v) has invalid SchemaVersion (%v),"+
			" the valid schema version has to be like 1.0, 1.1, 2.0, 3.9, etc.",
			customInventoryItem.TypeName, schemaVersion)
		LogError(log, err)
	}
	return
}

func validateContentEntryAttributes(log log.T, attributes map[string]interface{}) (err error) {
	if attributes == nil {
		err = fmt.Errorf("Custom inventory item content is not a valid json")
		LogError(log, err)
		return
	}
	if len(attributes) > AttributeCountLimit {
		err = fmt.Errorf("One of custom inventory item (%v)'s content has %v attributes, exceed the limit %v",
			attributes,
			len(attributes),
			AttributeCountLimit)
		LogError(log, err)
		return
	}
	for a, v := range attributes {
		aLen := len(a)
		if aLen > AttributeNameLengthLimit {
			err = fmt.Errorf("Custom inventory attribute name (%v) length: %v, exceeded the limit: %v",
				a,
				aLen,
				AttributeNameLengthLimit)
			LogError(log, err)
			return
		} else if aLen == 0 {
			err = fmt.Errorf("Custom inventory contains empty attribute name, which is illegal")
			LogError(log, err)
			return
		}

		if vStr, ok := v.(string); ok {
			vLen := len(vStr)
			if vLen > AttributeValueLengthLimit {
				err = fmt.Errorf("Custom inventory attribute (%v)'s value length (%v) "+
					"exceeded the limit (%v). Please either reduce the length or split the attribute "+
					"into multiple attributes.",
					a,
					vLen,
					AttributeValueLengthLimit)
				LogError(log, err)
				return
			}
		} else {
			err = fmt.Errorf("Custom inventory attribute (%v)'s value (%v)'s type (%v) is not supported. "+
				"Only string type is supported, suggest to use empty string for Nil or Null value.",
				a,
				v,
				reflect.TypeOf(v))
			LogError(log, err)
			return
		}
	}
	return
}

// validateContentEntrySchema validates attribute name and value
func validateContentEntrySchema(log log.T, customInventoryItem model.CustomInventoryItem) (
	entryArray []map[string]interface{},
	err error) {

	if customInventoryItem.Content == nil {
		err = errors.New("Custom inventory item missed Content property.")
		LogError(log, err)
		return
	}

	contentValue := customInventoryItem.Content
	log.Debugf("Content type of %v: %v", customInventoryItem.TypeName, reflect.TypeOf(contentValue))
	// custom inventory gatherer supports both map and array of map. For array of map, it goes through
	// each map to validate content
	if attributes, ok := contentValue.(map[string]interface{}); ok {
		if err = validateContentEntryAttributes(log, attributes); err == nil {
			entryArray = append(entryArray, attributes)
		}
	} else if content, ok := contentValue.([]interface{}); ok {
		for _, rawAttributes := range content {
			if attributes, ok = rawAttributes.(map[string]interface{}); ok {
				if err = validateContentEntryAttributes(log, attributes); err == nil {
					entryArray = append(entryArray, attributes)
				}
			} else {
				err = errors.New("Custom inventory entry is not valid")
				LogError(log, err)
				return
			}
		}
	} else {
		err = fmt.Errorf("Custom inventory item %v's Content is not a valid json",
			customInventoryItem.TypeName)
		LogError(log, err)
	}
	return
}

// getFilePaths reads all files with specified suffix under the given folder
func getFilePaths(log log.T, folder string, fileSuffix string) (fileFullPathList []string, err error) {

	var totalSize int64

	// Read all files that ended with json
	files, readDirError := readDirFunc(folder)
	if readDirError != nil {
		LogError(
			log,
			fmt.Errorf("Read directory %v failed, error: %v", folder, readDirError))
		// In case of directory not found error, ignore
		return []string{}, nil
	}

	for _, f := range files {

		if filepath.Ext(f.Name()) == fileSuffix {

			fileFullPath := filepath.Join(folder, f.Name())
			fileFullPath = filepath.Clean(fileFullPath)
			fileFullPathList = append(fileFullPathList, fileFullPath)
			totalSize += f.Size()
		}
	}

	// Check custom inventory file count
	if len(fileFullPathList) > CustomInventoryCountLimit {
		err = fmt.Errorf("Total custom inventory file count (%v) exceed limit (%v)",
			len(fileFullPathList), CustomInventoryCountLimit)
		LogError(log, err)
		return nil, err
	}

	log.Debugf("Total custom (%v) inventory file, total bytes: %v",
		len(fileFullPathList), totalSize)
	return
}
