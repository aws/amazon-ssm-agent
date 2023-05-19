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
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/ssm/authregister"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ec2roleprovider"
	"github.com/aws/amazon-ssm-agent/common/identity/endpoint"
	"github.com/aws/amazon-ssm-agent/common/runtimeconfig"
	"github.com/aws/aws-sdk-go/aws/credentials"
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

// IEC2Identity defines the functions for the EC2 identity
type IEC2Identity interface {
	InstanceID() (string, error)
	Region() (string, error)
	AvailabilityZone() (string, error)
	AvailabilityZoneId() (string, error)
	InstanceType() (string, error)
	IsIdentityEnvironment() bool
	Credentials() *credentials.Credentials
	IdentityType() string
	Register()
}

// Identity is the struct implementing the IAgentIdentityInner interface for the EC2 identity
type Identity struct {
	Log                   log.T
	Client                iEC2MdsSdkClient
	Config                *appconfig.SsmagentConfig
	credentials           *credentials.Credentials
	credentialsProvider   ec2roleprovider.IEC2RoleProvider
	authRegisterService   authregister.IClient
	shareLock             *sync.RWMutex
	registrationReadyChan chan *authregister.RegistrationInfo
	endpointHelper        endpoint.IEndpointHelper
	runtimeConfigClient   runtimeconfig.IIdentityRuntimeConfigClient
}
