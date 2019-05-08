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

import (
	"sort"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/versionutil"
)

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
	// Bump up to 3MB for agent. We have more strict size limit rule in the micro service.
	SizeLimitKBPerInventoryType = 3072
	// TotalSizeLimitKB represents size limit in KB for 1 PutInventory API call
	TotalSizeLimitKB = 10240
	// Standard name for 64-bit architecture
	Arch64Bit = "x86_64"
	// Standard name for 32-bit architecture
	Arch32Bit = "i386"
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

// ComponentType is a flags enum that data providers can set and gatherers can filter on
type ComponentType uint

const (
	AWSComponent ComponentType = 1 << iota
)

// ApplicationData captures all attributes present in AWS:Application inventory type
type ApplicationData struct {
	Name            string
	Publisher       string
	Version         string
	Release         string `json:",omitempty"`
	Epoch           string `json:",omitempty"`
	InstalledTime   string `json:",omitempty"`
	ApplicationType string `json:",omitempty"`
	Architecture    string
	URL             string        `json:",omitempty"`
	Summary         string        `json:",omitempty"`
	PackageId       string        `json:",omitempty"`
	CompType        ComponentType `json:"-"`
}

// FileData captures all attributes present in AWS:File inventory type
type FileData struct {
	Name             string
	Size             string
	Description      string
	FileVersion      string
	InstalledDate    string
	ModificationTime string
	LastAccessTime   string
	ProductName      string
	InstalledDir     string
	CompanyName      string
	ProductVersion   string
	ProductLanguage  string
}

type RoleData struct {
	Name                      string
	DisplayName               string
	Description               string
	Installed                 string
	InstalledState            string
	FeatureType               string
	Path                      string
	SubFeatures               string
	ServerComponentDescriptor string
	DependsOn                 string
	Parent                    string
}

type ServiceData struct {
	Name               string
	DisplayName        string
	Status             string
	DependentServices  string
	ServicesDependedOn string
	ServiceType        string
	StartType          string
}

type RegistryData struct {
	ValueName string
	ValueType string
	KeyPath   string
	Value     string
}

// BillingInfoData captures all attributes present in AWS:BillingInfo inventory type
type BillingInfoData struct {
	BillingProductId string
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

// InstanceDetailedInformation captures all attributes present in AWS:InstanceDetailedInformation inventory type
type InstanceDetailedInformation struct {
	CPUModel              string
	CPUSpeedMHz           string
	CPUs                  string
	CPUSockets            string
	CPUCores              string
	CPUHyperThreadEnabled string
	OSServicePack         string
}

// Config captures all various properties (including optional) that can be supplied to a gatherer.
// NOTE: Not all properties will be applicable to all gatherers.
// E.g: Applications gatherer uses Collection, Files use Filters, Custom uses Collection & Location.
type Config struct {
	Collection string `json:"Collection"`
	Filters    string `json:"Filters"`
	Location   string `json:"Location"`
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

// FormatArchitecture converts different architecture values to the standard inventory value
func FormatArchitecture(arch string) string {
	arch = strings.ToLower(strings.TrimSpace(arch))
	if arch == "amd64" {
		return Arch64Bit
	}
	if arch == "386" {
		return Arch32Bit
	}
	return arch
}

// ByNamePublisherVersion implements sorting ApplicationData elements by name (case insensitive) then by publisher (case insensitive) then version (by component)
type ByNamePublisherVersion []ApplicationData

func (s ByNamePublisherVersion) Len() int {
	return len(s)
}

func (s ByNamePublisherVersion) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s ByNamePublisherVersion) Less(i, j int) bool {
	//we need to compare string by ignoring it's case
	return compareApplicationData(s[i], s[j], true) < 0
}

func compareApplicationData(this ApplicationData, other ApplicationData, strictSort bool) int {
	if nameResult := compareName(this.Name, other.Name); nameResult != 0 {
		return nameResult
	}
	if publisherResult := comparePublisher(this.Publisher, other.Publisher, strictSort); publisherResult != 0 {
		return publisherResult
	}
	return versionutil.Compare(this.Version, other.Version, strictSort)
}

func compareName(this string, other string) int {
	return strings.Compare(strings.ToLower(this), strings.ToLower(other))
}

func comparePublisher(this string, other string, strictSort bool) int {
	if !strictSort && (this == "" || other == "") { // If either publisher is blank and this isn't a strict sort, assume a match because publisher isn't required
		return 0
	} else {
		return strings.Compare(strings.ToLower(this), strings.ToLower(other))
	}
}

// MergeLists combines a list of application data from a secondary source with a list from a primary source and returns a sorted result
func MergeLists(primary []ApplicationData, secondary []ApplicationData) []ApplicationData {
	//sorts the data based on application-name
	sort.Sort(ByNamePublisherVersion(primary))
	sort.Sort(ByNamePublisherVersion(secondary))

	if len(primary) == 0 {
		return secondary
	}
	if len(secondary) == 0 {
		return primary
	}

	//merge the arrays
	result := make([]ApplicationData, 0)

	indexPrimary := 0
	indexSecondary := 0

	for indexPrimary < len(primary) && indexSecondary < len(secondary) {
		compareResult := compareApplicationData(primary[indexPrimary], secondary[indexSecondary], false)
		switch {
		case compareResult < 0:
			result = append(result, primary[indexPrimary])
			indexPrimary++
		case compareResult > 0:
			result = append(result, secondary[indexSecondary])
			indexSecondary++
		default:
			result = append(result, mergeItems(primary[indexPrimary], secondary[indexSecondary]))
			indexPrimary++
			indexSecondary++
		}
	}
	// append any remaining primary items
	if indexPrimary < len(primary) {
		result = append(result, primary[indexPrimary:]...)
	}
	// append any remaining secondary items
	if indexSecondary < len(secondary) {
		result = append(result, secondary[indexSecondary:]...)
	}

	return result
}

// mergeItems merges values from a secondary source of application data into a matching primary source
func mergeItems(primary ApplicationData, secondary ApplicationData) ApplicationData {
	merged := primary
	if primary.ApplicationType == "" {
		merged.ApplicationType = secondary.ApplicationType
	}
	if primary.Architecture == "" {
		merged.Architecture = secondary.Architecture
	}
	if primary.CompType == ComponentType(0) {
		merged.CompType = secondary.CompType
	}
	if primary.InstalledTime == "" {
		merged.InstalledTime = secondary.InstalledTime
	}
	if primary.Publisher == "" {
		merged.Publisher = secondary.Publisher
	}
	if primary.URL == "" {
		merged.URL = secondary.URL
	}
	return merged
}
