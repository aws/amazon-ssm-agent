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

package ec2

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
)

const (
	ec2InstanceIDResource         = "instance-id"
	ec2InstanceTypeResource       = "instance-type"
	ec2AvailabilityZoneResource   = "placement/availability-zone"
	ec2AvailabilityZoneResourceId = "placement/availability-zone-id"
	ec2ServiceDomainResource      = "services/domain"
	ec2MacsResource               = "network/interfaces/macs"
	ec2VpcCidrBlockV4Resource     = "vpc-ipv4-cidr-block"
	ec2VpcCidrBlockV6Resource     = "vpc-ipv6-cidr-blocks"
	// IdentityType is the identity type for EC2
	IdentityType = "EC2"
)

// iEC2MdsSdkClient defines the functions that ec2_identity depends on from the aws sdk
type iEC2MdsSdkClient interface {
	GetMetadata(string) (string, error)
	GetInstanceIdentityDocument() (ec2metadata.EC2InstanceIdentityDocument, error)
	Region() (string, error)
}

// Identity is the struct defining the IAgentIdentityInner for EC2 metadata service
type Identity struct {
	Log    log.T
	Client iEC2MdsSdkClient
}
