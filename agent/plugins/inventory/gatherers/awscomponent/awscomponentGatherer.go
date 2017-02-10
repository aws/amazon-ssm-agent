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

// Package awscomponent contains a aws component gatherer.
package awscomponent

import (
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/gatherers/application"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
)

const (
	// GathererName captures name of aws component gatherer
	GathererName = "AWS:AWSComponent"
	// SchemaVersionOfApplication represents schema version of aws component gatherer
	SchemaVersionOfApplication = "1.0"
)

// T represents aws component gatherer which implements all contracts for gatherers.
type T struct{}

// decoupling platform.PlatformName for easy testability
var osInfoProvider = platformInfoProvider

func platformInfoProvider(log log.T) (name string, err error) {
	return platform.PlatformName(log)
}

// decoupling for easy testability
var getApplicationData = application.CollectApplicationData

// Gatherer returns new aws component gatherer
func Gatherer(context context.T) *T {
	return new(T)
}

// Name returns name of aws component gatherer
func (t *T) Name() string {
	return GathererName
}

// Run executes aws component gatherer and returns list of inventory.Item containing aws component data
func (t *T) Run(context context.T, configuration model.Config) (items []model.Item, err error) {

	var result model.Item

	//CaptureTime must comply with format: 2016-07-30T18:15:37Z to comply with regex at SSM.
	currentTime := time.Now().UTC()
	captureTime := currentTime.Format(time.RFC3339)

	result = model.Item{
		Name:          t.Name(),
		SchemaVersion: SchemaVersionOfApplication,
		Content:       CollectAWSComponentData(context),
		CaptureTime:   captureTime,
	}

	items = append(items, result)
	return
}

// RequestStop stops the execution of application gatherer.
func (t *T) RequestStop(stopType contracts.StopType) error {
	var err error
	return err
}

// CollectAWSComponentData collects aws component specific inventory data
func CollectAWSComponentData(context context.T) (data []model.ApplicationData) {

	var err error
	var plName string

	log := context.Log()

	//get platform name
	if plName, err = osInfoProvider(log); err != nil {
		log.Infof("Unable to detect platform because of %v - hence no inventory data for %v",
			err.Error(),
			GathererName)
		data = nil
		return
	}

	//based on OS name filter the data
	osName := strings.ToLower(plName)

	log.Infof("Platform name: %v, small case conversion: %v", plName, osName)

	//get application data
	data = getApplicationData(context)

	log.Infof("Number of applications detected by %v - %v", application.GathererName, len(data))
	log.Debugf("Filtering out awscomponents from list of all applications")

	data = FilterAWSComponent(data)

	log.Infof("Number of applications detected by %v - %v", GathererName, len(data))
	log.Debugf("Applications detected by AWSComponents:\n%v", data)

	return
}

// FilterAWSComponent filters aws components data from generic applications data.
func FilterAWSComponent(appData []model.ApplicationData) (awsComponent []model.ApplicationData) {
	//iterate over entire list and select application if it's name is present in our pre-approved list of amazon
	//published applications
	for _, application := range appData {
		if application.CompType&model.AWSComponent == model.AWSComponent {
			awsComponent = append(awsComponent, application)
		}
	}

	return
}
