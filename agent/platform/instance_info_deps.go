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
	"runtime"

	"github.com/aws/amazon-ssm-agent/agent/managedInstances/registration"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
)

// dependency for managed instance registration
var managedInstance instanceRegistration = instanceInfo{}

type instanceRegistration interface {
	InstanceID() string
	Region() string
	InstanceType() string
	AvailabilityZone() string
}

type instanceInfo struct{}

// ServerID returns the managed instance ID
func (instanceInfo) InstanceID() string { return registration.InstanceID() }

// Region returns the managed instance region
func (instanceInfo) Region() string { return registration.Region() }

// InstanceType returns the managed instance instanceType
func (instanceInfo) InstanceType() string { return registration.InstanceType() }

// AvailabilityZone returns the managed instance availabilityZone
func (instanceInfo) AvailabilityZone() string { return registration.AvailabilityZone() }

// dependency for metadata
var metadata metadataClient = instanceMetadata{
	Client: ec2metadata.New(session.New(aws.NewConfig().WithMaxRetries(10).WithEC2MetadataDisableTimeoutOverride(false))),
}

type metadataClient interface {
	GetMetadata(p string) (string, error)
	Region() (string, error)
}

// Alter our behavior based on where we're running.
func init() {

	// Set our retries to just one and make sure we're using the shorter overrides
	// Macs don't have instance metadata
	if runtime.GOOS == "darwin" {
		metadata = instanceMetadata{
			Client: ec2metadata.New(session.New(aws.NewConfig().WithMaxRetries(0).WithEC2MetadataDisableTimeoutOverride(true))),
		}
	}
}

type instanceMetadata struct {
	Client *ec2metadata.EC2Metadata
}

// GetMetadata uses the path provided to request
func (c instanceMetadata) GetMetadata(p string) (string, error) {
	return c.Client.GetMetadata(p)
}

// Region returns the region the instance is running in.
func (c instanceMetadata) Region() (string, error) { return c.Client.Region() }

// dependency for metadata
var dynamicData dynamicDataClient = instanceDynamicData{
	Client: NewEC2MetadataClient(),
}

type dynamicDataClient interface {
	Region() (string, error)
}

type instanceDynamicData struct {
	Client *EC2MetadataClient
}

// Region returns the region from dynamic data
func (d instanceDynamicData) Region() (string, error) {
	var instanceIdentityDocument *InstanceIdentityDocument
	var err error

	if instanceIdentityDocument, err = d.Client.InstanceIdentityDocument(); err == nil &&
		instanceIdentityDocument != nil && instanceIdentityDocument.Region != "" {
		return instanceIdentityDocument.Region, nil
	}
	return "", err
}
