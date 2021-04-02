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

package onprem

import (
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/onprem/rsaauth"
	"github.com/aws/aws-sdk-go/aws/credentials"
)

// IdentityType is the identity type for OnPrem
const IdentityType = "OnPrem"

const (
	// ProviderName provides a name of managed instance Role provider
	ProviderName = "managedInstancesRoleProvider"

	// EarlyExpiryTimeWindow set a short amount of time that will mark the credentials as expired, this can avoid
	// calls being made with expired credentials. This value should not be too big that's greater than the default token
	// expiry time. For example, the token expires after 30 min and we set it to 40 min which expires the token
	// immediately. The value should also not be too small that it should trigger credential rotation before it expires.
	EarlyExpiryTimeWindow = 1 * time.Minute
)

var (
	emptyCredential = credentials.Value{ProviderName: ProviderName}
	shareLock       sync.RWMutex
	shareCreds      bool
	shareProfile    string
)

// managedInstancesRoleProvider implements the AWS SDK credential provider, and is used to the create AWS client.
// It retrieves credentials from the SSM Auth service, and keeps track if those credentials are expired.
type managedInstancesRoleProvider struct {
	credentials.Expiry

	// ExpiryWindow will allow the credentials to trigger refreshing prior to
	// the credentials actually expiring. This is beneficial so race conditions
	// with expiring credentials do not cause request to fail unexpectedly
	// due to ExpiredTokenException exceptions.
	//
	// So a ExpiryWindow of 10s would cause calls to IsExpired() to return true
	// 10 seconds before the credentials are actually expired.
	//
	// If ExpiryWindow is 0 or less it will be ignored.
	ExpiryWindow time.Duration

	// client is the required SSM managed instance service client to use when connecting to SSM Auth service.
	client rsaauth.RsaSignedService
	config *appconfig.SsmagentConfig
	log    log.T
}

// Identity is the struct defining the IAgentIdentityInner for OnPrem
type Identity struct {
	Log                  log.T
	Config               *appconfig.SsmagentConfig
	credentialsSingleton *credentials.Credentials
}
