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

// Package model contains contracts for inventory
package model

import "strings"

const (
	// AWSInstanceInformation is inventory type of instance information
	AWSInstanceInformation = "AWS:InstanceInformation"
	// Enabled represents constant string used to enable various components of inventory plugin
	Enabled = "Enabled"
	// ErrorThreshold represents error threshold for inventory plugin
	ErrorThreshold = 10
	// InventoryPolicyDocName represents name of inventory policy doc
	InventoryPolicyDocName = "policy.json"
	// SizeLimitKBPerInventoryType represents size limit in KB for 1 inventory data type
	SizeLimitKBPerInventoryType = 200
	// TotalSizeLimitKB represents size limit in KB for 1 PutInventory API call
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
	PlatformName    string
	PlatformType    string
	PlatformVersion string
	// SSM Inventory expects it InstanceId and not InstanceID
	InstanceId string
	// SSM Inventory expects it IpAddress and not IPAddress
	IpAddress string
}

// ApplicationData captures all attributes present in AWS:Application inventory type
type ApplicationData struct {
	Name            string
	Publisher       string
	Version         string
	InstalledTime   string `json:",omitempty"`
	ApplicationType string `json:",omitempty"`
	Architecture    string
	URL             string `json:",omitempty"`
}

// NetworkData captures all attributes present in AWS:Network inventory type
type NetworkData struct {
	Name       string
	SubnetMask string `json:",omitempty"`
	Gateway    string `json:",omitempty"`
	DHCPServer string `json:",omitempty"`
	DNSServer  string `json:",omitempty"`
	MacAddress string
	IPV4       string
	IPV6       string
}

// WindowsUpdateData captures all attributes present in AWS:WindowsUpdate inventory type
type WindowsUpdateData struct {
	// SSM Inventory expects it HotFixId and not HotFixID
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

//ByName defines sort functionality for application data
type ByName []ApplicationData

func (s ByName) Len() int {
	return len(s)
}

func (s ByName) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s ByName) Less(i, j int) bool {
	//we need to compare string by ignoring it's case
	return strings.ToLower(s[i].Name) < strings.ToLower(s[j].Name)
}
