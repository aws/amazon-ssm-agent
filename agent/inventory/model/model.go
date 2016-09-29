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

//TODO: add all inventory types here
//TODO: if all attributes of inventory types become strong typed then we can directly refer to aws-sdk rather
//than defining everything here

const (
	// AWSInstanceInformation is inventory type of instance information
	AWSInstanceInformation   = "AWS:InstanceInformation"
	InventoryPluginName      = "Inventory"
	BasicInventoryPluginName = "BasicInventory"
	Enabled                  = "Enabled"
	ErrorThreshold           = 10
	InventoryPolicyDocName   = "policy.json"

	// size limit in KB for 1 inventory data type
	SizeLimitKBPerInventoryType = 200

	// size limit in KB for 1 PutInventory API call
	TotalSizeLimitKB = 1024
)

// Item encapsulates an inventory item
type Item struct {
	Name string
	//content depends on inventory type - hence set as interface{} here.
	//e.g: for application - it will contain []ApplicationData,
	//for instanceInformation - it will contain []InstanceInformation.
	Content       interface{}
	ContentHash   string
	SchemaVersion string
	CaptureTime   string
}

// InstanceInformation captures all attributes present in AWS:InstanceInformation inventory type
type InstanceInformation struct {
	AgentStatus     string
	AgentVersion    string
	ComputerName    string
	IPAddress       string
	InstanceId      string
	PlatformName    string
	PlatformType    string
	PlatformVersion string
}

// ApplicationData captures all attributes present in AWS:Application inventory type
type ApplicationData struct {
	Name            string
	Publisher       string
	Version         string
	InstalledTime   string
	ApplicationType string
	Architecture    string
	Url             string
}

type WindowsUpdateData struct {
	HotFixId      string
	Description   string
	InstalledTime string
	InstalledBy   string
}

// Config captures all various properties (including optional) that can be supplied to a gatherer.
// NOTE: Not all properties will be applicable to all gatherers.
// E.g: Applications gatherer uses Collection, Files use Filters, Custom uses Collection & Location.
type Config struct {
	Collection string
	Filters    []string
	Location   string
}

//TODO: this struct might change depending on the type of data associate plugin provides to inventory plugin
//For e.g: this will incorporate association & runId after integrating with associate plugin.
// Policy defines how an inventory policy document looks like
type Policy struct {
	InventoryPolicy map[string]Config
}

// CustomInventoryItem represents the schema of custom inventory item
type CustomInventoryItem struct {
	TypeName      string
	SchemaVersion string
	Content       interface{}
}
