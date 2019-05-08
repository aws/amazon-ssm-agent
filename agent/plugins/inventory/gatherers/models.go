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

// Package gatherers contains routines for different types of inventory gatherers
package gatherers

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/application"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/awscomponent"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/billinginfo"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/custom"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/file"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/instancedetailedinformation"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/network"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/registry"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/role"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/service"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/windowsUpdate"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
)

// T defines operations that all inventory gatherers support
type T interface {
	//returns the Name of the gatherer
	Name() string
	//runs the gatherer with a given configuration
	//returns array of inventory.Item as custom gatherer collects multiple
	//inventory items at a time
	Run(context context.T, configuration model.Config) ([]model.Item, error)
	//stops the execution of a gatherer
	RequestStop(stopType contracts.StopType) error
}

// SupportedGatherer is a map of supported gatherer on current platform
type SupportedGatherer map[string]T

// InstalledGatherer is a map of gatherers of all platforms
type InstalledGatherer map[string]T

// InitializeGatherers collects supported and installed gatherers
func InitializeGatherers(context context.T) (SupportedGatherer, InstalledGatherer) {
	log := context.Log()
	var installedGathererNames []string

	installedGatherer := InstalledGatherer{
		application.GathererName:                 application.Gatherer(context),
		awscomponent.GathererName:                awscomponent.Gatherer(context),
		custom.GathererName:                      custom.Gatherer(context),
		network.GathererName:                     network.Gatherer(context),
		billinginfo.GathererName:                 billinginfo.Gatherer(context),
		windowsUpdate.GathererName:               windowsUpdate.Gatherer(context),
		file.GathererName:                        file.Gatherer(context),
		instancedetailedinformation.GathererName: instancedetailedinformation.Gatherer(context),
		role.GathererName:                        role.Gatherer(context),
		service.GathererName:                     service.Gatherer(context),
		registry.GathererName:                    registry.Gatherer(context),
	}

	for key := range installedGatherer {
		installedGathererNames = append(installedGathererNames, key)
	}

	log.Infof("Installed Gatherer: %v", installedGathererNames)
	supportedGatherer := SupportedGatherer{}

	for _, name := range supportedGathererNames {
		supportedGatherer[name] = installedGatherer[name]
	}

	log.Infof("Supported Gatherer: %v", supportedGathererNames)

	return supportedGatherer, installedGatherer
}
