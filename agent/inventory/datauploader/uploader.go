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
	SendDataToSSM(context context.T, items []*ssm.InventoryItem)
	ConvertToSsmInventoryItems(context context.T, items []model.Item) (inventoryItems []*ssm.InventoryItem, err error)
}

// InventoryUploader implements functionality to upload data to SSM Inventory.
type InventoryUploader struct {
	ssm *ssm.SSM
	//isOptimizerEnabled ensures PutInventory API is not called if same data is sent, if this is false - then even
	//if instanceInfo is same, every 5 mins data will be sent to SSM Inventory.
	isOptimizerEnabled bool
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
	uploader.isOptimizerEnabled = appCfg.Ssm.InventoryOptimizer == model.Enabled

	return &uploader, nil
}

// SendDataToSSM uploads given inventory items to SSM
func (u *InventoryUploader) SendDataToSSM(context context.T, items []*ssm.InventoryItem) {
	log := context.Log()
	log.Infof("Uploading following inventory data to SSM - %v", items)

	var instanceID string
	var err error

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

			//TODO: If API throws ContentHashMismatch Exception -> send the entire data again
			//TODO: If API has other exception -> do reasonable retries and report error.

			log.Errorf("Encountered error while calling PutInventory API %v", err)
		} else {
			log.Debugf("PutInventory was called successfully with response - %v", resp)
		}
	}
}

// ConvertToSsmInventoryItems converts given array of inventory.Item into an array of *ssm.InventoryItem
func (u *InventoryUploader) ConvertToSsmInventoryItems(context context.T, items []model.Item) (inventoryItems []*ssm.InventoryItem, err error) {

	log := context.Log()
	var dataB []byte
	var i *ssm.InventoryItem

	//NOTE: There can be multiple inventory type data.
	//Each inventory type data => 1 inventory Item. Each inventory type, can contain multiple items

	//iterating over multiple inventory data types.
	for _, item := range items {

		dataB, _ = json.Marshal(item)

		//Note - behavior of not sending same data again, is customizable. This is only relevant
		//for integrating with awsconfig for now - later this policy will be changed.

		if u.isOptimizerEnabled && !ShouldUpdate(item.Name, string(dataB)) {

			//TODO: set the content hash accordingly for inventoryItem
			log.Infof("No update of inventory data of type %v", item.Name)

		} else {

			//convert to ssm.InventoryItem

			if i, err = ConvertToSSMInventoryItem(item); err != nil {
				err = fmt.Errorf("Error encountered while formatting data - %v", err.Error())
				return
			}

			inventoryItems = append(inventoryItems, i)
		}

	}

	return
}
