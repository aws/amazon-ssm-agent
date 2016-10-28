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
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/inventory/model"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
)

const (
	// Name represents name of this component that uploads data to SSM
	Name = "InventoryUploader"
)

// T represents contracts for SSM Inventory data uploader
type T interface {
	SendDataToSSM(context context.T, items []*ssm.InventoryItem) (err error)
	ConvertToSsmInventoryItems(context context.T, items []model.Item) (optimizedInventoryItems, nonOptimizedInventoryItems []*ssm.InventoryItem, err error)
}

// InventoryUploader implements functionality to upload data to SSM Inventory.
type InventoryUploader struct {
	ssm       *ssm.SSM
	optimizer Optimizer //helps inventory plugin to optimize PutInventory calls
}

// NewInventoryUploader creates a new InventoryUploader (which sends data to SSM Inventory)
func NewInventoryUploader(context context.T) (*InventoryUploader, error) {
	var uploader = InventoryUploader{}
	var appCfg appconfig.SsmagentConfig
	var err error

	c := context.With("[" + Name + "]")
	log := c.Log()

	// reading agent appconfig
	if appCfg, err = appconfig.Config(false); err != nil {
		log.Errorf("Could not load config file %v", err.Error())
		return &uploader, err
	}

	// setting ssm client config
	cfg := sdkutil.AwsConfig()
	cfg.Region = &appCfg.Agent.Region
	cfg.Endpoint = &appCfg.Ssm.Endpoint

	uploader.ssm = ssm.New(session.New(cfg))

	if uploader.optimizer, err = NewOptimizerImpl(context); err != nil {
		log.Errorf("Unable to load optimizer for inventory uploader because - %v", err.Error())
		return &uploader, err
	}

	return &uploader, nil
}

// SendDataToSSM uploads given inventory items to SSM
func (u *InventoryUploader) SendDataToSSM(context context.T, items []*ssm.InventoryItem) (err error) {
	log := context.Log()
	log.Infof("Uploading following inventory data to SSM - %v", items)

	var instanceID string

	log.Infof("Inventory Items: %v", items)
	log.Infof("Number of Inventory Items: %v", len(items))

	if instanceID, err = platform.InstanceID(); err != nil {
		log.Errorf("Unable to fetch InstanceId, instance information will not be sent to Inventory")
		return
	}

	//setting up input for PutInventory API call
	params := &ssm.PutInventoryInput{
		InstanceId: &instanceID,
		Items:      items,
	}
	var resp *ssm.PutInventoryOutput

	log.Debugf("Calling PutInventory API with parameters - %v", params)
	if u.ssm != nil {
		resp, err = u.ssm.PutInventory(params)

		if err != nil {
			log.Errorf("Encountered error while calling PutInventory API %v", err)
		} else {
			log.Debugf("PutInventory was called successfully with response - %v", resp)
		}
	}

	return
}

func calculateCheckSum(data []byte) (checkSum string) {
	sum := md5.Sum(data)
	checkSum = base64.StdEncoding.EncodeToString(sum[:])
	return
}

// ConvertToSsmInventoryItems converts given array of inventory.Item into an array of *ssm.InventoryItem. It returns 2 such arrays - one is optimized array
// which contains only contentHash for those inventory types where the dataset hasn't changed from previous collection. The other array is non-optimized array
// which contains both contentHash & content. This is done to avoid iterating over the inventory data twice. It throws error when it encounters error during
// conversion process.
func (u *InventoryUploader) ConvertToSsmInventoryItems(context context.T, items []model.Item) (optimizedInventoryItems, nonOptimizedInventoryItems []*ssm.InventoryItem, err error) {

	log := context.Log()
	var dataB []byte
	var oldHash, newHash string
	var optimizedItem, nonOptimizedItem *ssm.InventoryItem

	//NOTE: There can be multiple inventory type data.
	//Each inventory type data => 1 inventory Item. Each inventory type, can contain multiple items

	//iterating over multiple inventory data types.
	for _, item := range items {

		//we should only calculate checksum using content & not include capture time - because that field will always change causing
		//the checksum to change again & again even if content remains same.

		dataB, _ = json.Marshal(item.Content)
		newHash = calculateCheckSum(dataB)

		//construct non-optimized inventory item
		if nonOptimizedItem, err = ConvertToSSMInventoryItem(item); err != nil {
			err = fmt.Errorf("formatting inventory data of %v failed due to %v", item.Name, err.Error())
			return
		}

		//add contentHash too
		nonOptimizedItem.ContentHash = &newHash

		nonOptimizedInventoryItems = append(nonOptimizedInventoryItems, nonOptimizedItem)

		//populate optimized item - if content hash matches with earlier collected data.
		oldHash = u.optimizer.GetContentHash(item.Name)
		if newHash == oldHash {

			log.Debugf("Inventory data for %v is same as before - we can just send content hash", item.Name)

			//set the inventory item accordingly
			optimizedItem = &ssm.InventoryItem{
				CaptureTime:   &item.CaptureTime,
				TypeName:      &item.Name,
				SchemaVersion: &item.SchemaVersion,
				ContentHash:   &oldHash,
			}

			optimizedInventoryItems = append(optimizedInventoryItems, optimizedItem)

		} else {
			log.Debugf("New inventory data for %v has been detected - can't optimize here", item.Name)

			optimizedInventoryItems = append(optimizedInventoryItems, nonOptimizedItem)

			log.Debugf("Updating cache")

			if err = u.optimizer.UpdateContentHash(item.Name, newHash); err != nil {
				err = fmt.Errorf("failed to update content hash cache because of - %v", err.Error())
				log.Error(err.Error())
				return
			}
		}
	}

	return
}
