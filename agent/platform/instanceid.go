// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package platform provides instance information
package platform

import (
	"sync"

	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
)

var cachedInstanceID string
var cachedRegion string
var lock sync.RWMutex

// SetInstanceID fetches the instance id with the following preference order.
// 1. Commandline
// 2. Instance Metadata
// 3. For onprem, we need to have different provider (todo)
func SetInstanceID(log logger.T, defaultID string) (instanceID string, err error) {
	lock.Lock()
	defer lock.Unlock()
	if cachedInstanceID != "" {
		return cachedInstanceID, nil
	}

	if defaultID != "" {
		cachedInstanceID = defaultID
		return cachedInstanceID, nil
	}

	log.Info("trying to get instanceid from metadata...")

	c := ec2metadata.New(session.New(aws.NewConfig().WithMaxRetries(5)))
	instanceID, err = c.GetMetadata("instance-id")
	if err != nil {
		log.Error("failed to get instanceid from metadata.", err)
		return
	}
	log.Info("found instanceid from metadata = ", instanceID)
	cachedInstanceID = instanceID
	return
}

// InstanceID returns the current instance id
func InstanceID() string {
	lock.RLock()
	defer lock.RUnlock()
	return cachedInstanceID
}

// SetRegion sets the instance region
func SetRegion(log logger.T, defaultRegion string) (region string, err error) {
	lock.Lock()
	defer lock.Unlock()
	if cachedRegion != "" {
		return cachedRegion, nil
	}

	if defaultRegion != "" {
		cachedRegion = defaultRegion
		return cachedRegion, nil
	}

	log.Info("trying to get region from metadata...")

	c := ec2metadata.New(session.New(aws.NewConfig().WithMaxRetries(5)))
	region, err = c.Region()
	if err != nil {
		log.Error("failed to get region from metadata.", err)
		return
	}
	log.Info("found region from metadata = ", region)
	cachedRegion = region
	return
}

// Region returns the instance region
func Region() string {
	lock.RLock()
	defer lock.RUnlock()
	return cachedRegion
}
