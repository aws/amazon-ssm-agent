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
	"hash/fnv"
	"math/rand"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
)

const (
	// Name represents name of this component that uploads data to SSM
	Name = "InventoryUploader"
	// The maximum time window range for random back off before call PutInventory API
	Max_Time_TO_Back_Off = 30
)

// T represents contracts for SSM Inventory data uploader
type T interface {
	SendDataToSSM(context context.T, items []*ssm.InventoryItem) (err error)
	ConvertToSsmInventoryItems(context context.T, items []model.Item) (optimizedInventoryItems, nonOptimizedInventoryItems []*ssm.InventoryItem, err error)
	GetDirtySsmInventoryItems(context context.T, items []model.Item) (dirtyInventoryItems []*ssm.InventoryItem, err error)
}

type SSMCaller interface {
	PutInventory(input *ssm.PutInventoryInput) (output *ssm.PutInventoryOutput, err error)
}

// InventoryUploader implements functionality to upload data to SSM Inventory.
type InventoryUploader struct {
	ssm       SSMCaller
	optimizer Optimizer //helps inventory plugin to optimize PutInventory calls
}

// NewInventoryUploader creates a new InventoryUploader (which sends data to SSM Inventory)
func NewInventoryUploader(context context.T) (*InventoryUploader, error) {
	var uploader = InventoryUploader{}
	var appCfg appconfig.SsmagentConfig
	var err error

	c := context.With("[" + Name + "]")
	log := c.Log()

	// setting ssm client config
	cfg := sdkutil.AwsConfig()

	// overrides ssm client config from appconfig if applicable
	if appCfg, err = appconfig.Config(false); err == nil {

		if appCfg.Ssm.Endpoint != "" {
			cfg.Endpoint = &appCfg.Ssm.Endpoint
		} else {
			if region, err := platform.Region(); err == nil {
				if defaultEndpoint := appconfig.GetDefaultEndPoint(region, "ssm"); defaultEndpoint != "" {
					cfg.Endpoint = &defaultEndpoint
				}
			} else {
				log.Errorf("error fetching the region, %v", err)
			}
		}
		if appCfg.Agent.Region != "" {
			cfg.Region = &appCfg.Agent.Region
		}
	}
	sess := session.New(cfg)
	sess.Handlers.Build.PushBack(request.MakeAddToUserAgentHandler(appCfg.Agent.Name, appCfg.Agent.Version))

	uploader.ssm = ssm.New(sess)

	if uploader.optimizer, err = NewOptimizerImpl(context); err != nil {
		log.Errorf("Unable to load optimizer for inventory uploader because - %v", err.Error())
		return &uploader, err
	}

	return &uploader, nil
}

// SendDataToSSM uploads given inventory items to SSM
func (u *InventoryUploader) SendDataToSSM(context context.T, items []*ssm.InventoryItem) (err error) {
	log := context.Log()
	log.Debugf("Uploading following inventory data to SSM - %v", items)

	var instanceID string

	log.Debugf("Inventory Items: %v", items)
	log.Infof("Number of Inventory Items: %v", len(items))

	if instanceID, err = machineIDProvider(); err != nil {
		log.Errorf("Unable to fetch InstanceId, instance information will not be sent to Inventory")
		return
	}

	//setting up input for PutInventory API call
	params := &ssm.PutInventoryInput{
		InstanceId: &instanceID,
		Items:      items,
	}
	var resp *ssm.PutInventoryOutput

	// random back off before call PutInventory API
	time.Sleep(time.Duration(getRandomBackOffTime(context, instanceID)) * time.Second)
	log.Debugf("Calling PutInventory API with parameters - %v", params)
	if u.ssm != nil {
		resp, err = u.ssm.PutInventory(params)

		if err != nil {
			log.Errorf("the following error occured while calling PutInventory API: %v", err)
		} else {
			log.Debugf("PutInventory was called successfully with response - %v", resp)
			u.updateContentHash(context, items)
		}
	}

	return
}

// Get one random jitter time before calling PutInventory API to prevent huge number of request come to
// the backend service in the same time.
// Use current Time stamp + Hashcode of instance ID as random key
// The jitter window is in 0-30 seconds.
func getRandomBackOffTime(context context.T, instanceID string) (sleepTime int) {
	log := context.Log()

	hash := fnv.New32a()
	hash.Write([]byte(instanceID))
	rand.Seed(time.Now().Unix() + int64(hash.Sum32()))
	sleepTime = rand.Intn(Max_Time_TO_Back_Off)
	log.Debugf("Random back off: %v seconds before call put inventory", sleepTime)
	return sleepTime
}

func (u *InventoryUploader) updateContentHash(context context.T, items []*ssm.InventoryItem) {
	log := context.Log()
	log.Debugf("Updating cache")
	for _, item := range items {
		if err := u.optimizer.UpdateContentHash(*item.TypeName, *item.ContentHash); err != nil {
			err = fmt.Errorf("failed to update content hash cache because of - %v", err.Error())
			log.Error(err.Error())
		}
	}
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

	//NOTE: There can be multiple inventory type data.
	//Each inventory type data => 1 inventory Item. Each inventory type, can contain multiple items

	log.Debugf("Transforming collected inventory data to expected format")

	//iterating over multiple inventory data types.
	for _, item := range items {

		var dataB []byte
		var optimizedItem, nonOptimizedItem *ssm.InventoryItem

		newHash := ""
		oldHash := ""
		itemName := item.Name

		//we should only calculate checksum using content & not include capture time - because that field will always change causing
		//the checksum to change again & again even if content remains same.

		if dataB, err = json.Marshal(item.Content); err != nil {
			return
		}

		newHash = calculateCheckSum(dataB)
		log.Debugf("Item being converted - %v with data - %v with checksum - %v", itemName, string(dataB), newHash)

		//construct non-optimized inventory item
		if nonOptimizedItem, err = ConvertToSSMInventoryItem(item); err != nil {
			err = fmt.Errorf("formatting inventory data of %v failed due to %v", itemName, err.Error())
			return
		}

		//add contentHash too
		nonOptimizedItem.ContentHash = &newHash

		log.Debugf("NonOptimized item - %+v", nonOptimizedItem)

		nonOptimizedInventoryItems = append(nonOptimizedInventoryItems, nonOptimizedItem)

		//populate optimized item - if content hash matches with earlier collected data.
		oldHash = u.optimizer.GetContentHash(itemName)

		log.Debugf("old hash - %v, new hash - %v for the inventory type - %v", oldHash, newHash, itemName)

		if newHash == oldHash {
			log.Debugf("Inventory data for %v is same as before - we can just send content hash", itemName)

			//set the inventory item accordingly
			optimizedItem = &ssm.InventoryItem{
				CaptureTime:   &item.CaptureTime,
				TypeName:      &itemName,
				SchemaVersion: &item.SchemaVersion,
				ContentHash:   &oldHash,
			}

			log.Debugf("Optimized item - %v", optimizedItem)

			optimizedInventoryItems = append(optimizedInventoryItems, optimizedItem)

		} else {
			log.Debugf("New inventory data for %v has been detected - can't optimize here", itemName)
			log.Debugf("Adding item - %v to the optimizedItems (since its new data)", nonOptimizedItem)

			optimizedInventoryItems = append(optimizedInventoryItems, nonOptimizedItem)
		}
	}

	return
}

// GetDirtySsmInventoryItems get the inventory item data for items that have changes since last successful report to SSM.
func (u InventoryUploader) GetDirtySsmInventoryItems(context context.T, items []model.Item) (dirtyInventoryItems []*ssm.InventoryItem, err error) {
	log := context.Log()

	//NOTE: There can be multiple inventory type data.
	//Each inventory type data => 1 inventory Item. Each inventory type, can contain multiple items

	//iterating over multiple inventory data types.
	for _, item := range items {
		var dataB []byte
		var rawItem *ssm.InventoryItem

		newHash := ""
		oldHash := ""
		itemName := item.Name

		//we should only calculate checksum using content & not include capture time - because that field will always change causing
		//the checksum to change again & again even if content remains same.

		if dataB, err = json.Marshal(item.Content); err != nil {
			return
		}

		newHash = calculateCheckSum(dataB)
		log.Debugf("Item being converted - %v with data - %v with checksum - %v", itemName, string(dataB), newHash)

		//construct non-optimized inventory item
		if rawItem, err = ConvertToSSMInventoryItem(item); err != nil {
			err = fmt.Errorf("Formatting inventory data of %v failed due to %v, rawItem : %#v", itemName, err.Error(), rawItem)
			return
		}

		//add contentHash too
		rawItem.ContentHash = &newHash

		//populate optimized item - if content hash matches with earlier collected data.
		oldHash = u.optimizer.GetContentHash(itemName)

		log.Infof("Get Dirty inventory items, old hash - %v, new hash - %v for the inventory type - %v", oldHash, newHash, itemName)

		if strings.Compare(newHash, oldHash) != 0 {
			log.Infof("Dirty inventory type found. Change has been detected for inventory type: %v", itemName)
			dirtyInventoryItems = append(dirtyInventoryItems, rawItem)
		} else {
			log.Infof("Content hash is the same with the old for %v", itemName)
		}
	}

	return
}
