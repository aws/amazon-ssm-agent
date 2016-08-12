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
	"time"

	"src/github.com/aws/aws-sdk-go/service/ssm"
)

//TODO: add all inventory types here
//TODO: if all attributes of inventory types become strong typed then we can directly refer to aws-sdk rather
//than defining everything here

const (
	// AWSInstanceInformation is inventory type of instance information
	AWSInstanceInformation = "AWS:InstanceInformation"
)

// Item encapsulates an inventory item
type Item struct {
	name          string
	content       string
	contentHash   string
	schemaVersion string
	captureTime   time.Time
}

// BaseInventoryFields captures all basic attributes present in all inventory types
type BaseInventoryFields struct {
	SchemaVersion string
	CaptureTime   string
}

// InstanceInformation captures all attributes present in AWS:InstanceInformation inventory type
type InstanceInformation struct {
	BaseInventoryFields
	ssm.UpdateInstanceInformationInput
}
