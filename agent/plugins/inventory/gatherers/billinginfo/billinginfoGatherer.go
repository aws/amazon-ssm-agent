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

// Package billinginfo contains a billinginfo gatherer.
package billinginfo

import (
	"errors"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
)

const (
	// GathererName captures name of billinginfo gatherer
	GathererName = "AWS:BillingInfo"
	// SchemaVersionOfApplication represents schema version of billinginfo gatherer
	SchemaVersionOfBillingInfo = "1.0"
)

var collectData = CollectBillingInfoData

// T represents billinginfo gatherer which implements all contracts for gatherers.
type T struct{}

// Gatherer returns new billinginfo gatherer
func Gatherer(context context.T) *T {
	return new(T)
}

// Name returns name of billinginfo gatherer
func (t *T) Name() string {
	return GathererName
}

// Run executes billinginfo gatherer and returns list of inventory.Item comprising of billinginfo data
func (t *T) Run(context context.T, configuration model.Config) (items []model.Item, err error) {

	var result model.Item

	//CaptureTime must comply with format: 2016-07-30T18:15:37Z to comply with regex at SSM.
	currentTime := time.Now().UTC()
	captureTime := currentTime.Format(time.RFC3339)

	result = model.Item{
		Name:          t.Name(),
		SchemaVersion: SchemaVersionOfBillingInfo,
		Content:       collectData(context),
		CaptureTime:   captureTime,
	}

	items = append(items, result)
	return
}

// RequestStop stops the execution of BillingInfo gatherer.
func (t *T) RequestStop(stopType contracts.StopType) error {
	return errors.New("gatherer stop not supported")
}
