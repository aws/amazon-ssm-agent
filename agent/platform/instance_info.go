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

// Package platform provides instance information
package platform

import (
	"fmt"
	"strings"
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/platform/containers"
)

var cachedRegion, cachedAvailabilityZone, cachedInstanceType, cachedInstanceID, cachedTargetID string
var lock sync.RWMutex

const errorMessage = "Failed to fetch %s. Data from vault is empty. %v"

var getConfig = appconfig.Config

// InstanceID returns the current instance id
func InstanceID() (string, error) {
	lock.RLock()
	defer lock.RUnlock()
	if cachedInstanceID != "" {
		return cachedInstanceID, nil
	} else {
		var err error
		cachedInstanceID, err = fetchInstanceID()
		return cachedInstanceID, err
	}
}

// TargetID returns the current target id for container
func TargetID() (string, error) {
	lock.RLock()
	defer lock.RUnlock()
	if cachedTargetID != "" {
		return cachedTargetID, nil
	} else {
		var err error
		cachedTargetID, err = fetchTargetID()
		return cachedTargetID, err
	}
}

func SetTargetID(targetID string) error {
	lock.Lock()
	defer lock.Unlock()
	if targetID == "" {
		return fmt.Errorf("invalid target id")
	}
	cachedTargetID = targetID
	return nil
}

// SetInstanceID overrides the platform instanceID
func SetInstanceID(instanceID string) error {
	lock.Lock()
	defer lock.Unlock()
	if instanceID == "" {
		return fmt.Errorf("invalid instanceID")
	}
	cachedInstanceID = instanceID
	return nil
}

// InstanceType returns the current instance type
func InstanceType() (string, error) {
	lock.RLock()
	defer lock.RUnlock()
	if cachedInstanceType != "" {
		return cachedInstanceType, nil
	} else {
		var err error
		cachedInstanceType, err = fetchInstanceType()
		return cachedInstanceType, err
	}
}

// SetInstanceType overrides the platform instance type
func SetInstanceType(instanceType string) error {
	lock.Lock()
	defer lock.Unlock()
	if instanceType == "" {
		return fmt.Errorf("invalid instance type")
	}
	cachedInstanceType = instanceType
	return nil
}

// Region returns the instance region
func Region() (string, error) {
	var err error
	lock.RLock()
	defer lock.RUnlock()
	if cachedRegion != "" {
		return cachedRegion, nil
	}

	cachedRegion, err = fetchRegion()
	return cachedRegion, err

}

// SetRegion overrides the platform region
func SetRegion(region string) error {
	lock.Lock()
	defer lock.Unlock()
	if region == "" {
		return fmt.Errorf("invalid region")
	}
	cachedRegion = region
	return nil
}

// AvailabilityZone returns the instance availability zone
func AvailabilityZone() (string, error) {
	var err error
	lock.RLock()
	defer lock.RUnlock()
	if cachedAvailabilityZone != "" {
		return cachedAvailabilityZone, nil
	}

	cachedAvailabilityZone, err = fetchAvailabilityZone()
	return cachedAvailabilityZone, err
}

// SetAvailabilityZone overrides the platform availability zone
func SetAvailabilityZone(availabilityZone string) error {
	lock.Lock()
	defer lock.Unlock()
	if availabilityZone == "" {
		return fmt.Errorf("invalid availability zone")
	}
	cachedAvailabilityZone = availabilityZone
	return nil
}

// IsManagedInstance returns if the current instance is managed instance
func IsManagedInstance() (bool, error) {
	instanceId, err := InstanceID()
	if err != nil {
		return false, err
	}

	if strings.Contains(instanceId, "mi-") {
		return true, nil
	}
	return false, nil
}

// fetchInstanceID fetches the instance id with the following preference order.
// 1. managed instance registration
// 2. EC2 Instance Metadata
func fetchInstanceID() (string, error) {
	var err error
	var instanceID string

	config, _ := getConfig(false)
	if config.Agent.ContainerMode {
		container := &containers.Container{}
		targetID, err := container.TargetID()
		infoArray := strings.Split(targetID, "_")
		containerID := infoArray[len(infoArray)-1]
		return containerID, err
	}

	// trying to get instance id from managed instance registration
	if instanceID = managedInstance.InstanceID(); instanceID != "" {
		return instanceID, nil
	}

	// trying to get instance id from ec2 metadata
	if instanceID, err = metadata.GetMetadata("instance-id"); instanceID != "" && err == nil {
		return instanceID, nil
	}

	// return combined error messages
	return "", fmt.Errorf(errorMessage, "instance ID", err)
}

func fetchTargetID() (string, error) {
	config, _ := getConfig(false)
	if config.Agent.ContainerMode {
		return container.TargetID()
	} else {
		return "", nil
	}
}

// fetchInstanceType fetches the instance type with the following preference order.
// 1. managed instance registration
// 2. EC2 Instance Metadata
func fetchInstanceType() (string, error) {
	var err error
	var instanceType string

	// trying to get region from managed instance registration
	if instanceType = managedInstance.InstanceType(); instanceType != "" {
		return instanceType, nil
	}

	// trying to get instance id from ec2 metadata
	if instanceType, err = metadata.GetMetadata("instance-type"); instanceType != "" && err == nil {
		return instanceType, nil
	}

	// return combined error messages
	return "", fmt.Errorf(errorMessage, "instance Type", err)
}

// fetchRegion fetches the region with the following preference order.
// 1. managed instance registration
// 2. EC2 Instance Metadata
// 3. EC2 Instance Dynamic Data
func fetchRegion() (string, error) {
	var err error
	var region string

	config, err := getConfig(false)
	if err == nil && config.Agent.ContainerMode {
		container := &containers.Container{}
		return container.Region()
	}

	// trying to get region from managed instance registration
	if region = managedInstance.Region(); region != "" {
		return region, nil
	}

	// trying to get region from metadata
	if region, err = metadata.Region(); region != "" && err == nil {
		return region, nil
	}

	// trying to get region from dynamic data
	if region, err = dynamicData.Region(); region != "" && err == nil {
		return region, nil
	}

	// return combined error messages
	return "", fmt.Errorf(errorMessage, "region", err)
}

// fetchAvailabilityZone fetches the  availability zone with the following preference order.
// 1. managed instance registration
// 2. EC2 Instance Metadata
// 3. EC2 Instance Dynamic Data
func fetchAvailabilityZone() (string, error) {
	var err error
	var availabilityZone string

	// trying to get region from managed instance registration
	if availabilityZone = managedInstance.AvailabilityZone(); availabilityZone != "" {
		return availabilityZone, nil
	}

	// trying to get instance id from ec2 metadata
	if availabilityZone, err = metadata.GetMetadata("placement/availability-zone"); availabilityZone != "" && err == nil {
		return availabilityZone, nil
	}

	// trying to get region from dynamic data
	if availabilityZone, err = dynamicData.Region(); availabilityZone != "" && err == nil {
		return availabilityZone, nil
	}

	// return combined error messages
	return "", fmt.Errorf(errorMessage, "availability zone", err)
}
