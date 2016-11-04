// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the AWS Customer Agreement (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/agreement/

// Package platform provides instance information
package platform

import (
	"fmt"
	"strings"
	"sync"
)

var cachedRegion string
var lock sync.RWMutex

const errorMessage = "Failed to fetch %s. Data from vault is empty. %v"

// InstanceID returns the current instance id
func InstanceID() (string, error) {
	lock.RLock()
	defer lock.RUnlock()

	return fetchInstanceID()
}

// SetInstanceID overrides the platform instanceID
func SetInstanceID(instanceID string) error {
	lock.Lock()
	defer lock.Unlock()
	if instanceID == "" {
		return fmt.Errorf("invalid instanceID")
	}
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

// fetchRegion fetches the region with the following preference order.
// 1. managed instance registration
// 2. EC2 Instance Metadata
// 3. EC2 Instance Dynamic Data
func fetchRegion() (string, error) {
	var err error
	var region string

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
