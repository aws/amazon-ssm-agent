// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License").
// You may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ec2roleprovider

import (
	"context"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ssmclient"

	"github.com/aws/aws-sdk-go-v2/credentials/ec2rolecreds"
)

const (
	agentName            = "amazon-ssm-agent"
	CredentialSourceNone = "None"
	CredentialSourceSSM  = "SSM"
	CredentialSourceEC2  = "EC2"
	IdentityTypeEC2      = "EC2"
)

var (
	iprEmptyCredential                                 = aws.Credentials{Source: "EC2RoleProvider"}
	newV4ServiceWithCreds        ssmclient.Initializer = ssmclient.NewV4ServiceWithCreds
	timeNowFunc                                        = time.Now
	newCredentials                                     = ec2rolecreds.New
	exceptionsForDefaultHostMgmt                       = map[string]struct{}{
		"AccessDeniedException":        {},
		"EC2RoleRequestError":          {},
		"AssumeRoleUnauthorizedAccess": {},
	}
)

type IInnerProvider interface {
	aws.CredentialsProvider
	Retrieve(ctx context.Context) (aws.Credentials, error)
	ExpiresAt() time.Time
	IsExpired() bool
}

// cachedInnerProvider adapts an aws.CredentialsProvider — typically the
// aws.CredentialsCache wrapper around a network-fetching provider — to the
// IInnerProvider interface. Callers of EC2InnerProviders' fields expect the
// ExpiresAt / IsExpired methods, but aws.CredentialsCache doesn't expose them
// (its expiry state is internal). The current EC2RoleProvider only ever calls
// Retrieve on inner providers that are typed as CredentialsCache — the
// ExpiresAt / IsExpired paths are exercised only by SharedCredentialsProvider
// — so this adapter fulfils the interface with sensible defaults: IsExpired
// defers to the cache's own logic by treating cached creds as valid, and
// ExpiresAt returns the last observed Expires from the most recent Retrieve.
type cachedInnerProvider struct {
	cache        *aws.CredentialsCache
	lastRetrieve time.Time
	lastCreds    aws.Credentials
	mu           sync.Mutex
}

// NewCachedInnerProvider wraps an aws.CredentialsProvider in a cache-backed
// IInnerProvider. See cachedInnerProvider for the rationale.
func NewCachedInnerProvider(inner aws.CredentialsProvider) IInnerProvider {
	return newCachedInnerProvider(inner)
}

func newCachedInnerProvider(inner aws.CredentialsProvider) *cachedInnerProvider {
	return &cachedInnerProvider{cache: aws.NewCredentialsCache(inner)}
}

func (c *cachedInnerProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	creds, err := c.cache.Retrieve(ctx)
	if err != nil {
		return creds, err
	}
	c.mu.Lock()
	c.lastCreds = creds
	c.lastRetrieve = time.Now()
	c.mu.Unlock()
	return creds, nil
}

func (c *cachedInnerProvider) ExpiresAt() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastCreds.Expires
}

func (c *cachedInnerProvider) IsExpired() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.lastCreds.CanExpire {
		return false
	}
	return time.Now().After(c.lastCreds.Expires)
}

type EC2InnerProviders struct {
	IPRProvider               IInnerProvider
	SsmEc2Provider            IInnerProvider
	SharedCredentialsProvider IInnerProvider
}

type IEC2RoleProvider interface {
	aws.CredentialsProvider
	credentialproviders.IRemoteProvider
	GetInnerProvider() IInnerProvider
	Retrieve(ctx context.Context) (aws.Credentials, error)
	ShareFile() string
	ShareProfile() string
	SharesCredentials() bool
	RemoteRetrieve(ctx context.Context) (aws.Credentials, error)
}
