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

// Package basicInventory implements basicInventory core plugin which sends instance information to SSM Inventory
package basicInventory

import (
	"fmt"
	"runtime"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/datauploader"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/carlescere/scheduler"
)

const (
	schemaVersionOfInstanceInfoInventoryItem = "1.0"
	pluginName                               = "BasicInventory"
	//agentStatus is agent's status which is sent as instanceInformation inventory type data to ssm inventory
	agentStatus                         = "Active"
	errorMsgForInabilityToSendDataToSSM = "Unable to send basic instance information to SSM Inventory"
)

// BasicInventoryProvider encapsulates the logic of configuring, starting and stopping basic inventory plugin
type BasicInventoryProvider struct {
	context            context.T
	frequencyInMinutes int
	stopPolicy         *sdkutil.StopPolicy
	updateJob          *scheduler.Job
	ssm                *ssm.SSM
	//uploader handles uploading inventory data to SSM.
	uploader datauploader.T
	//isEnabled enables basic inventory gatherer, if this is false - then basic inventory gatherer will not run.
	isEnabled bool
}

// NewBasicInventoryProvider creates a new basic inventory provider core plugin.
func NewBasicInventoryProvider(context context.T) (*BasicInventoryProvider, error) {
	var appCfg appconfig.SsmagentConfig
	var err error
	var provider = BasicInventoryProvider{}

	c := context.With("[" + pluginName + "]")

	// setting ssm client config
	cfg := sdkutil.AwsConfig()

	// overrides ssm client config from appconfig if applicable
	if appCfg, err = appconfig.Config(false); err == nil {

		if appCfg.Ssm.Endpoint != "" {
			cfg.Endpoint = &appCfg.Ssm.Endpoint
		}
		if appCfg.Agent.Region != "" {
			cfg.Region = &appCfg.Agent.Region
		}
	}

	//setting basic inventory config
	provider.isEnabled = appCfg.Ssm.BasicInventoryGatherer == model.Enabled

	provider.context = c
	provider.stopPolicy = sdkutil.NewStopPolicy(pluginName, model.ErrorThreshold)
	provider.ssm = ssm.New(session.New(cfg))
	//for now we are using the same frequency as that of health plugin to send inventory data
	provider.frequencyInMinutes = appCfg.Ssm.HealthFrequencyMinutes

	if provider.uploader, err = datauploader.NewInventoryUploader(c); err != nil {
		err = fmt.Errorf("Unable to initialize uploader for %v core plugin due to - %v, hence failing",
			pluginName, err.Error())
		return &provider, err
	}

	return &provider, nil
}

// GetInstanceInformation returns the latest set of instance information
func GetInstanceInformation(context context.T) (model.InstanceInformation, error) {

	var instanceInfo model.InstanceInformation

	log := context.Log()

	instanceInfo.AgentStatus = *aws.String(agentStatus)
	instanceInfo.AgentVersion = *aws.String(version.Version)

	//TODO: detecting OS can be added as an utility since its used by health plugin & basic inventory plugin
	goOS := runtime.GOOS
	switch goOS {
	case updateutil.PlatformWindows:
		instanceInfo.PlatformType = *aws.String(ssm.PlatformTypeWindows)
	case updateutil.PlatformLinux:
		instanceInfo.PlatformType = *aws.String(ssm.PlatformTypeLinux)
	default:
		return instanceInfo, fmt.Errorf("Cannot report platform type of unrecognized OS. %v", goOS)
	}

	if instID, err := platform.InstanceID(); err == nil {
		instanceInfo.InstanceId = *aws.String(instID)
	} else {
		err = fmt.Errorf("unable to get instanceId due to - %v", err.Error())
		log.Error(err.Error())
		return instanceInfo, err
	}

	if ip, err := platform.IP(); err == nil {
		instanceInfo.IpAddress = *aws.String(ip)
	} else {
		log.Warn(err)
	}

	if h, err := platform.Hostname(); err == nil {
		instanceInfo.ComputerName = *aws.String(h)
	} else {
		log.Warn(err)
	}

	if n, err := platform.PlatformName(log); err == nil {
		instanceInfo.PlatformName = *aws.String(n)
	} else {
		log.Warn(err)
	}

	if v, err := platform.PlatformVersion(log); err == nil {
		instanceInfo.PlatformVersion = *aws.String(v)
	} else {
		log.Warn(err)
	}

	return instanceInfo, nil
}

// instanceInformationInventoryItem returns latest instance information inventory item
func (b *BasicInventoryProvider) instanceInformationInventoryItem() (model.Item, error) {
	var err error
	var data model.InstanceInformation
	var item model.Item

	if data, err = GetInstanceInformation(b.context); err == nil {
		//CaptureTime must comply with format: 2016-07-30T18:15:37Z or else it will throw error
		t := time.Now().UTC()
		time := t.Format(time.RFC3339)

		item = model.Item{
			Name:          model.AWSInstanceInformation,
			Content:       data,
			SchemaVersion: schemaVersionOfInstanceInfoInventoryItem,
			//capture time must be in UTC so that formatting to RFC3339 complies with regex at SSM
			CaptureTime: time,
		}
	} else {
		err = fmt.Errorf("Unable to fetch instance information - %v", err.Error())
	}

	return item, err
}

// updateBasicInventory updates basic instance information inventory data in SSM
func (b *BasicInventoryProvider) updateBasicInventory() {
	log := b.context.Log()
	log.Infof("Updating basic inventory information.")

	var i model.Item
	var items []model.Item
	var err error
	var nonOptimizedInventoryItems, optimizedInventoryItems []*ssm.InventoryItem
	var status, retryWithNonOptimized bool

	//get latest instanceInfo inventory item
	if i, err = b.instanceInformationInventoryItem(); err != nil {
		log.Errorf("Encountered error while fetching instance information - %v", err)
		return
	}

	items = append(items, i)

	//construct non-optimized inventory item
	if optimizedInventoryItems, nonOptimizedInventoryItems, err = b.uploader.ConvertToSsmInventoryItems(b.context, items); err != nil {
		err = fmt.Errorf("formatting inventory data of %v failed due to %v", i.Name, err.Error())
		return
	}

	log.Debugf("Optimized data - \n%v \n Non-optimized data - \n%v",
		optimizedInventoryItems,
		nonOptimizedInventoryItems)

	//first send data in optimized fashion
	if status, retryWithNonOptimized = b.SendDataToInventory(optimizedInventoryItems); !status {

		if retryWithNonOptimized {
			//call putinventory again with non-optimized dataset
			if status, _ = b.SendDataToInventory(nonOptimizedInventoryItems); !status {
				//sending non-optimized data also failed
				log.Info(errorMsgForInabilityToSendDataToSSM)
				return
			}
		} else {
			//some other error happened for which there is no need to retry - upload failed
			log.Info(errorMsgForInabilityToSendDataToSSM)
			return
		}
	}

	return
}

// SendDataToInventory sends data to SSM and returns if data was sent successfully or not. If data is not uploaded successfully,
// it parses the error message and determines if it should be sent again.
func (b *BasicInventoryProvider) SendDataToInventory(items []*ssm.InventoryItem) (status, retryWithFullData bool) {
	var err error
	log := b.context.Log()

	if err = b.uploader.SendDataToSSM(b.context, items); err != nil {
		status = false
		if aerr, ok := err.(awserr.Error); ok {
			//NOTE: awserr.Code -> is not the error code but the exception name itself!!!!
			if aerr.Code() == "ItemContentMismatchException" || aerr.Code() == "InvalidItemContentException" {
				log.Debugf("%v encountered - inventory plugin will try sending data again", aerr.Code())
				retryWithFullData = true
			} else {
				log.Debugf("Unexpected error encountered - %v. No point trying to send data again", aerr.Code())
			}
		}
	} else {
		status = true
	}

	return
}

// ICorePlugin implementation

// Name returns the Plugin Name
func (b *BasicInventoryProvider) Name() string {
	return pluginName
}

// Execute starts the scheduling of the basic inventory plugin
func (b *BasicInventoryProvider) Execute(context context.T) (err error) {

	if b.isEnabled {
		b.context.Log().Debugf("Starting %s plugin", pluginName)
		if b.updateJob, err = scheduler.Every(b.frequencyInMinutes).Minutes().Run(b.updateBasicInventory); err != nil {
			context.Log().Errorf("Unable to schedule basic inventory plugin. %v", err)
		}
	} else {
		b.context.Log().Debugf("Skipping execution of %s plugin since its disabled",
			pluginName)
	}
	return
}

// RequestStop handles the termination of the basic inventory plugin job
func (b *BasicInventoryProvider) RequestStop(stopType contracts.StopType) (err error) {
	if b.updateJob != nil {
		b.context.Log().Info("Stopping basic inventory job.")
		b.updateJob.Quit <- true
	}
	return nil
}
