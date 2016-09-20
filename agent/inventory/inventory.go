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
	"path"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"

	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/carlescere/scheduler"
)

//TODO: integration with on-demand plugin - so that associate plugin can invoke this plugin

// BasicInventoryProvider encapsulates the logic of configuring, starting and stopping basic inventory plugin
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
}

// NewBasicInventoryProvider creates a new basic inventory provider core plugin.
func NewPlugin(context context.T) (*Plugin, error) {
	var appCfg appconfig.SsmagentConfig
	var err error
	var p = Plugin{}

	c := context.With("[" + InventoryPluginName + "]")
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
	p.isEnabled = appCfg.Ssm.InventoryPlugin == Enabled

	p.context = c
	p.stopPolicy = sdkutil.NewStopPolicy(InventoryPluginName, ErrorThreshold)
	p.ssm = ssm.New(session.New(cfg))

	//location - path where inventory policy doc is stored. (Note: this is temporary till we integrate with
	//associate plugin)
	p.location = appconfig.DefaultProgramFolder

	//for now we are using the same frequency as that of health plugin to look & apply new inventory policy
	p.frequencyInMinutes = appCfg.Ssm.HealthFrequencyMinutes

	return &p, nil
}

// ApplyInventoryPolicy applies basic instance information inventory data in SSM
func (p *Plugin) ApplyInventoryPolicy() {
	log := p.context.Log()
	log.Infof("Looking for SSM Inventory policy in %v", p.location)

	d := path.Join(p.location, InventoryPolicyDocName)
	//get latest instanceInfo inventory item
	if fileutil.Exists(d) {
		log.Infof("Applying Inventory policy ")

		//TODO: read inventory policy doc and act accordingly
		log.Debugf("Missing implementation of applying inventory policy")
	} else {
		log.Infof("No inventory policy to apply")
	}

	return
}

// ICorePlugin implementation

// Name returns Plugin Name
func (p *Plugin) Name() string {
	return InventoryPluginName
}

// Execute starts the scheduling of inventory plugin
func (p *Plugin) Execute(context context.T) (err error) {

	log := context.Log()
	log.Infof("Starting %v plugin", InventoryPluginName)

	if p.isEnabled {
		if p.job, err = scheduler.Every(p.frequencyInMinutes).Minutes().Run(p.ApplyInventoryPolicy); err != nil {
			err = log.Errorf("Unable to schedule %v plugin. %v", InventoryPluginName, err)
		}
	} else {
		log.Debugf("Skipping execution of %s plugin since its disabled", InventoryPluginName)
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
