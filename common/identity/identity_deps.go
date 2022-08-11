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

package identity

import (
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders"
	"github.com/aws/amazon-ssm-agent/common/identity/endpoint"

	"github.com/aws/aws-sdk-go/aws/credentials"
)

// IAgentIdentity defines the interface identity cacher exposes
type IAgentIdentity interface {
	InstanceID() (string, error)
	ShortInstanceID() (string, error)
	Region() (string, error)
	AvailabilityZone() (string, error)
	AvailabilityZoneId() (string, error)
	InstanceType() (string, error)
	Credentials() *credentials.Credentials
	IdentityType() string
	GetServiceEndpoint(string) string
}

// Registrar identity registers the agent on startup
type Registrar interface {
	Register() error
}
type IInnerIdentityGetter interface {
	GetInner() IAgentIdentityInner
}

// IAgentIdentityInner defines the interface each identity needs to expose
type IAgentIdentityInner interface {
	InstanceID() (string, error)
	Region() (string, error)
	AvailabilityZone() (string, error)
	AvailabilityZoneId() (string, error)
	InstanceType() (string, error)
	IsIdentityEnvironment() bool
	Credentials() *credentials.Credentials
	IdentityType() string
}

// ICredentialRefresherAgentIdentity defines the interface for identities that require custom credential refresh (e.g. onprem, ec2)
type ICredentialRefresherAgentIdentity interface {
	CredentialProvider() credentialproviders.IRemoteProvider
}

// ICredentialRefresherProvider defines the interface for credential providers that require custom credential refresh
type ICredentialRefresherProvider interface {
	ShareProfile() string
	ShareFile() string
	SharesCredentials() bool
}

// IMetadataIdentity defines the interface for identities that will query metadata for VPC CIDR block information (ec2, ecs)
type IMetadataIdentity interface {
	VpcPrimaryCIDRBlock() (map[string][]string, error)
}

type agentIdentityCacher struct {
	instanceID         string
	shortInstanceID    string
	region             string
	availabilityZone   string
	availabilityZoneId string
	instanceType       string
	creds              *credentials.Credentials
	identityType       string
	mutex              sync.Mutex
	log                log.T
	client             IAgentIdentityInner
	endpointHelper     endpoint.IEndpointHelper
}

type createIdentityFunc func(log.T, *appconfig.SsmagentConfig) []IAgentIdentityInner

// allIdentityGenerators store all the available identity types and their generator functions. init inside identity definition add to
var allIdentityGenerators map[string]createIdentityFunc
