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
	"context"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/ssm/authregister"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2/ec2detector"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ec2roleprovider"
	"github.com/aws/amazon-ssm-agent/common/runtimeconfig"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
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
	SourceType   = "AWS::EC2::Instance"
)

// iEC2MdsSdkClient defines the functions that ec2_identity depends on from the aws sdk
type iEC2MdsSdkClient interface {
	GetMetadata(ctx context.Context, params *imds.GetMetadataInput, optFns ...func(*imds.Options)) (*imds.GetMetadataOutput, error)
	GetInstanceIdentityDocument(ctx context.Context, params *imds.GetInstanceIdentityDocumentInput, optFns ...func(*imds.Options)) (*imds.GetInstanceIdentityDocumentOutput, error)
	GetRegion(ctx context.Context, params *imds.GetRegionInput, optFns ...func(*imds.Options)) (*imds.GetRegionOutput, error)
}

// IEC2Identity defines the functions for the EC2 identity
type IEC2Identity interface {
	InstanceID() (string, error)
	Region() (string, error)
	AvailabilityZone() (string, error)
	AvailabilityZoneId() (string, error)
	InstanceType() (string, error)
	IsIdentityEnvironment() bool
	CredentialsProvider() aws.CredentialsProvider
	IdentityType() string
	Register()
}

// Identity is the struct implementing the IAgentIdentityInner interface for the EC2 identity
type Identity struct {
	Log                 log.T
	Client              iEC2MdsSdkClient
	Config              *appconfig.SsmagentConfig
	credentials         aws.Credentials
	credentialsProvider ec2roleprovider.IEC2RoleProvider
	AuthRegisterService authregister.IClient
	shareLock           *sync.RWMutex
	runtimeConfigClient runtimeconfig.IIdentityRuntimeConfigClient
	ec2Detector         ec2detector.Ec2Detector
}

type IEC2RoleProviderWrapper struct {
	provider aws.CredentialsProvider
	expiry   time.Time
}

func (w *IEC2RoleProviderWrapper) Retrieve(ctx context.Context) (aws.Credentials, error) {
	creds, err := w.provider.Retrieve(ctx)
	if err == nil && creds.CanExpire && !creds.Expires.IsZero() {
		// Track the real IMDS credential lifetime so role detach is detected
		// promptly when the credentials actually expire.
		w.expiry = creds.Expires
	}
	return creds, err
}

func (w *IEC2RoleProviderWrapper) ExpiresAt() time.Time {
	return w.expiry
}

func (w *IEC2RoleProviderWrapper) IsExpired() bool {
	return time.Now().After(w.expiry)
}

// SetExpiry allows the outer EC2RoleProvider to propagate its computed
// expiry to this wrapper, keeping the two expiry sources in sync.
func (w *IEC2RoleProviderWrapper) SetExpiry(t time.Time) {
	w.expiry = t
}
