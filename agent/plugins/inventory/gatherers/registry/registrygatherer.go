// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package registry collects information about windows registry
package registry

import (
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
)

const (
	// GathererName captures name of Registry gatherer
	GathererName = "AWS:WindowsRegistry"
	// SchemaVersionOfRegistryGatherer represents schema version of Registry gatherer
	SchemaVersionOfRegistryGatherer = "1.0"
)

type T struct{}

// Gatherer returns new Process gatherer
func Gatherer(context context.T) *T {
	return new(T)
}

var collectData = collectRegistryData

// Name returns name of Process gatherer
func (t *T) Name() string {
	return GathererName
}

// Run executes Registry gatherer and returns list of inventory.Item comprising of registry data
func (t *T) Run(context context.T, configuration model.Config) (items []model.Item, err error) {
	var result model.Item

	//CaptureTime must comply with format: 2016-07-30T18:15:37Z to comply with regex at SSM.
	currentTime := time.Now().UTC()
	captureTime := currentTime.Format(time.RFC3339)
	var data []model.RegistryData
	data, err = collectData(context, configuration)

	result = model.Item{
		Name:          t.Name(),
		SchemaVersion: SchemaVersionOfRegistryGatherer,
		Content:       data,
		CaptureTime:   captureTime,
	}

	items = append(items, result)
	return
}

// RequestStop stops the execution of registry gatherer.
func (t *T) RequestStop(stopType contracts.StopType) error {
	var err error
	return err
}
