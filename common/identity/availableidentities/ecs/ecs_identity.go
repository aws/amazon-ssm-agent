// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package ecs

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders"
	"github.com/aws/aws-sdk-go/aws/credentials"
)

// InstanceID returns the ecs Instance ID
func (e *Identity) InstanceID() (string, error) {
	clusterName, taskID, err := fetchClusterNameAndTaskID()
	if err != nil {
		return "", err
	}
	containerID, err := fetchContainerID()
	if err != nil {
		return "", err
	}
	return "ecs:" + clusterName + "_" + taskID + "_" + containerID, nil
}

// Region returns the ecs region
func (e *Identity) Region() (string, error) { return fetchRegion() }

// AvailabilityZone returns the managed instance availabilityZone
func (e *Identity) AvailabilityZone() (string, error) { return fetchAvailabilityZone() }

// AvailabilityZoneId returns empty if the managed instance is not EC2
func (e *Identity) AvailabilityZoneId() (string, error) { return "", nil }

// InstanceType returns the managed instance instanceType
func (e *Identity) InstanceType() (string, error) { return ecsInstanceType, nil }

// Credentials returns the managed instance credentials
func (e *Identity) Credentials() *credentials.Credentials {
	return credentialproviders.GetRemoteCreds()
}

// IsIdentityEnvironment returns if instance has managed instance registration
func (e *Identity) IsIdentityEnvironment() bool {
	_, err := taskMetadataResponse()
	if err != nil {
		e.Log.Infof("Agent not taking ECS identity: %v", err)
	}
	return err == nil
}

// VpcPrimaryCIDRBlock returns ipv4, ipv6 VPC CIDR block addresses if exists
func (e *Identity) VpcPrimaryCIDRBlock() (map[string][]string, error) {
	return fetchCIDRBlock()
}

// IdentityType returns the identity type of the managed instance
func (e *Identity) IdentityType() string { return IdentityType }

// NewECSIdentity initializes the ecs identity
func NewECSIdentity(log log.T) *Identity {
	log = log.WithContext("[ECSIdentity]")
	return &Identity{
		Log: log,
	}
}
