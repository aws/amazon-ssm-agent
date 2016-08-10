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
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/amazon-ssm-agent/agent/ssm"
	"src/github.com/carlescere/scheduler"
)

// BasicInventoryProvider encapsulates the logic of configuring, starting and stopping basic inventory plugin
type BasicInventoryProvider struct {
	context    context.T
	stopPolicy *sdkutil.StopPolicy
	updateJob  *scheduler.Job
	ssm        ssm.Service
}

const (
	name                       = "BasicInventory"
	updateFrequencyInMinutes   = 5
	errorThresholdForInventory = 10
)

// NewBasicInventoryProvider creates a new basic inventory provider core plugin.
func NewBasicInventoryProvider(context context.T) *BasicInventoryProvider {
	c := context.With("[" + name + "]")
	p := sdkutil.NewStopPolicy(name, errorThresholdForInventory)
	s := ssm.NewService()

	return &BasicInventoryProvider{
		context:    c,
		stopPolicy: p,
		ssm:        s,
	}
}

// updates SSM with the instance health information
func (b *BasicInventoryProvider) updateBasicInventory() {
	log := b.context.Log()
	log.Infof("Updating basic inventory information.")

	//TODO: make a call to ssm putInventory API call
	//TODO: ensure putInventory API is only called if the instance information is not changed
	return
}

// ICorePlugin implementation

// Name returns the Plugin Name
func (b *BasicInventoryProvider) Name() string {
	return name
}

// Execute starts the scheduling of the basic inventory plugin
func (b *BasicInventoryProvider) Execute(context context.T) (err error) {

	//TODO: add a knob in appconfig - allowing customers to disable this plugin

	b.context.Log().Debugf("Starting %s plugin", name)

	if b.updateJob, err = scheduler.Every(updateFrequencyInMinutes).Minutes().Run(b.updateBasicInventory); err != nil {
		context.Log().Errorf("Unable to schedule basic inventory plugin. %v", err)
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
