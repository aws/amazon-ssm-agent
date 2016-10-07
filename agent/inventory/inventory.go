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
	"encoding/json"
	"path"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/inventory/datauploader"
	"github.com/aws/amazon-ssm-agent/agent/inventory/gatherers"
	"github.com/aws/amazon-ssm-agent/agent/inventory/model"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/carlescere/scheduler"
)

//TODO: integration with on-demand plugin - so that associate plugin can invoke this plugin
//TODO: add unit tests.

// Plugin encapsulates the logic of configuring, starting and stopping inventory plugin
type Plugin struct {
	//NOTE: Unless we integrate inventory plugin with associate/mds plugin, the only way to ingest inventory policy
	//document would be through files - where this plugin will periodically monitor for any changes to policy doc.
	context    context.T
	stopPolicy *sdkutil.StopPolicy
	ssm        *ssm.SSM
	//job is a scheduled job, which looks for updated inventory policy at a given location (this will be removed
	//when Plugin will be integrated with associate plugin)
	job                *scheduler.Job
	frequencyInMinutes int
	//location stores inventory policy doc
	location string
	//isEnabled enables inventory plugin, if this is false - then inventory plugin will not run.
	isEnabled bool
	//supportedGatherers is a map of all inventory gatherers supported by current OS
	// (e.g. WindowsUpdateGatherer is not included when running on Unix)
	supportedGatherers gatherers.SupportedGatherer

	//installedGatherers is a map of gatherers of all platforms
	installedGathereres gatherers.InstalledGatherer

	//uploader handles uploading inventory data to SSM.
	uploader datauploader.T
}

// NewPlugin creates a new inventory core plugin.
func NewPlugin(context context.T) (*Plugin, error) {
	var appCfg appconfig.SsmagentConfig
	var err error
	var p = Plugin{}

	c := context.With("[" + inventory.InventoryPluginName + "]")
	log := c.Log()

	// reading agent appconfig
	if appCfg, err = appconfig.Config(false); err != nil {
		log.Errorf("Could not load config file %v", err.Error())
		return &p, err
	}

	// setting ssm client config
	cfg := sdkutil.AwsConfig()
	cfg.Region = &appCfg.Agent.Region
	cfg.Endpoint = &appCfg.Ssm.Endpoint

	//setting inventory config
	p.isEnabled = appCfg.Ssm.InventoryPlugin == inventory.Enabled

	p.context = c
	p.stopPolicy = sdkutil.NewStopPolicy(inventory.InventoryPluginName, inventory.ErrorThreshold)
	p.ssm = ssm.New(session.New(cfg))

	//location - path where inventory policy doc is stored. (Note: this is temporary till we integrate with
	//associate plugin)
	p.location = appconfig.DefaultProgramFolder

	//for now we are using the same frequency as that of health plugin to look & apply new inventory policy
	p.frequencyInMinutes = appCfg.Ssm.HealthFrequencyMinutes

	//loads all registered gatherers (for now only a dummy application gatherer is loaded in memory)
	p.supportedGatherers, p.installedGathereres = gatherers.InitializeGatherers(p.context)
	//initializes SSM Inventory uploader
	if p.uploader, err = datauploader.NewInventoryUploader(context); err != nil {
		err = log.Errorf("Unable to configure SSM Inventory uploader - %v", err.Error())
	}

	return &p, err
}

// ApplyInventoryPolicy applies basic instance information inventory data in SSM
func (p *Plugin) ApplyInventoryPolicy() {
	//NOTE: this will only be used until we integrate with associate plugin
	log := p.context.Log()
	var policy inventory.Policy
	var inventoryItems []*ssm.InventoryItem
	var items []inventory.Item
	var err error
	var content string

	log.Infof("Looking for SSM Inventory policy in %v", p.location)

	doc := path.Join(p.location, inventory.InventoryPolicyDocName)
	//get latest instanceInfo inventory item
	if fileutil.Exists(doc) {
		log.Infof("Applying Inventory policy")

		//read file
		if content, err = fileutil.ReadAllText(doc); err == nil {

			if err = json.Unmarshal([]byte(content), &policy); err != nil {
				log.Infof("Encountered error while reading Inventory policy at %v. Error - %v",
					doc,
					err.Error())
				log.Infof("Skipping execution of inventory policy doc.")
				return
			}

			items, err := p.VerifyAndRunGatherers(policy)
			if err != nil {
				log.Infof("Encountered error while executing inventory policy: %v", err.Error())
				return
			}

			//log collected data before sending
			d, _ := json.Marshal(items)
			log.Infof("Collected Inventory data: %v", string(d))

			if inventoryItems, err = p.uploader.ConvertToSsmInventoryItems(p.context, items); err != nil {
				log.Infof("Encountered error in converting data to SSM InventoryItems - %v. Skipping upload to SSM", err.Error())
			}
			p.uploader.SendDataToSSM(p.context, inventoryItems)

		} else {
			log.Infof("Unable to read inventory policy from : %v because of error - %v", doc, err.Error())
		}
	} else {
		log.Infof("No inventory policy to apply")
	}

	return
}

// VerifyAndRunGatherers verifies if gatherers is installed and supported, and then invokes it to return the result
// (containing inventory data). It returns error if gatherer is not installed or if at any stage the data returned
// breaches size limit
func (p *Plugin) VerifyAndRunGatherers(policy inventory.Policy) (items []inventory.Item, err error) {
	log := p.context.Log()
	log.Infof("Verifying if gatherers are registered and then running them")

	//NOTE:
	//1) if the gatherer is installed but not supported by current platform, we will skip that gatherer. If the
	// gatherer is not installed,  we error out & don't send the data collected from other supported gatherers
	// - this is because we don't send partial inventory data as part of 1 inventory policy.
	//Either we send full set of inventory data as defined in policy - or we send nothing.

	//2) Currently all gatherers will be invoked in synchronous & sequential fashion.
	//Parallel execution of gatherers hinges upon inventory plugin becoming a long running plugin - which will be
	//mainly for custom inventory gatherer to send data independently of associate.

	for name := range policy.InventoryPolicy {
		//find out if the gatherer is indeed registered.
		if gatherer, isGathererSupported := p.supportedGatherers[name]; !isGathererSupported {
			if _, isGathererInstalled := p.installedGathereres[name]; isGathererInstalled {
				log.Infof("Installed but unsupported gatherer - %v", name)
			} else {
				err = log.Errorf("Unrecognized inventory gatherer - %v ", name)
				break
			}
		} else {
			var gItems []inventory.Item
			log.Infof("Invoking gatherer - %v", name)

			if gItems, err = gatherer.Run(p.context, policy.InventoryPolicy[name]); err != nil {
				err = log.Errorf("Encountered error while executing %v. Error - %v", name, err.Error())
				break
			} else {
				items = append(items, gItems...)

				//TODO: Each gather shall check each item's size and stop collecting if size exceed immediately
				//TODO: only check the total item size at this function, whenever total size exceed, stop
				//TODO: immediately and raise association error
				//return error if collected data breaches size limit
				for _, v := range gItems {
					if !p.VerifyInventoryDataSize(v, items) {
						err = log.Errorf("Size limit exceeded for collected data.")
						break
					}
				}
			}
		}
	}

	return items, err
}

// VerifyInventoryDataSize returns true if size of collected inventory data is within size restrictions placed by SSM,
// else false.
func (p *Plugin) VerifyInventoryDataSize(item inventory.Item, items []inventory.Item) bool {
	var itemSize, itemsSize float32
	log := p.context.Log()

	//calculating sizes
	itemB, _ := json.Marshal(item)
	itemSize = float32(len(itemB))

	log.Debugf("Size (Bytes) of %v - %v", item.Name, itemSize)

	itemsSizeB, _ := json.Marshal(items)
	itemsSize = float32(len(itemsSizeB))
	log.Debugf("Total size (Bytes) of inventory items after including %v - %v", item.Name, itemsSize)

	//Refer to https://wiki.ubuntu.com/UnitsPolicy regarding KiB to bytes conversion.
	//TODO: 200 KB limit might be too less for certain inventory types like Patch - we might have to revise that and
	//use different limits for different category.
	if (itemSize/1024) > inventory.SizeLimitKBPerInventoryType || (itemsSize/1024) > inventory.TotalSizeLimitKB {
		return false
	}

	return true
}

// ICorePlugin implementation

// Name returns Plugin Name
func (p *Plugin) Name() string {
	return inventory.InventoryPluginName
}

// Execute starts the scheduling of inventory plugin
func (p *Plugin) Execute(context context.T) (err error) {

	log := context.Log()
	log.Infof("Starting %v plugin", inventory.InventoryPluginName)

	//Note: Currently this plugin is not integrated with associate plugin so in turn
	//it schedules a job - that periodically reads inventory policy doc from a file and applies it.
	//TODO: remove this scheduled job - after integrating with associate plugin
	if p.isEnabled {
		if p.job, err = scheduler.Every(p.frequencyInMinutes).Minutes().Run(p.ApplyInventoryPolicy); err != nil {
			err = log.Errorf("Unable to schedule %v plugin. %v", inventory.InventoryPluginName, err)
		}
	} else {
		log.Debugf("Skipping execution of %s plugin since its disabled", inventory.InventoryPluginName)
	}
	return
}

// RequestStop handles the termination of inventory plugin job
func (p *Plugin) RequestStop(stopType contracts.StopType) (err error) {
	if p.job != nil {
		p.context.Log().Info("Stopping inventory job.")
		p.job.Quit <- true
	}
	return nil
}
