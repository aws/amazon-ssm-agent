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
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/aws/amazon-ssm-agent/common/identity"
)

const (
	// 64 is the max length of docker container ids
	MaxInstanceIDLen = 64
)

func (c *agentIdentityCacher) InstanceID() (string, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.instanceID != "" {
		return c.instanceID, nil
	}

	var err error
	c.instanceID, err = c.client.InstanceID()
	return c.instanceID, err
}

func (c *agentIdentityCacher) ShortInstanceID() (string, error) {
	var err error
	var shortInstanceID string

	shortInstanceID, err = c.InstanceID()
	if err != nil {
		return "", err
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.shortInstanceID != "" {
		return c.shortInstanceID, nil
	}

	if len(shortInstanceID) > MaxInstanceIDLen {
		splitInstanceID := strings.Split(shortInstanceID, "_")
		shortInstanceID = splitInstanceID[len(splitInstanceID)-1]
	}

	if len(shortInstanceID) > MaxInstanceIDLen {
		c.log.Warnf("Unable to shorten instance id '%s'", shortInstanceID)
	}

	c.shortInstanceID = shortInstanceID

	return c.shortInstanceID, nil
}

func (c *agentIdentityCacher) Region() (string, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.regionInner()
}

func (c *agentIdentityCacher) regionInner() (string, error) {
	if c.region != "" {
		return c.region, nil
	}

	var err error
	c.region, err = c.client.Region()
	return c.region, err
}

func (c *agentIdentityCacher) AvailabilityZone() (string, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.availabilityZone != "" {
		return c.availabilityZone, nil
	}

	var err error
	c.availabilityZone, err = c.client.AvailabilityZone()
	return c.availabilityZone, err
}

func (c *agentIdentityCacher) AvailabilityZoneId() (string, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.availabilityZoneId != "" {
		return c.availabilityZoneId, nil
	}

	var err error
	c.availabilityZoneId, err = c.client.AvailabilityZoneId()
	return c.availabilityZoneId, err
}

func (c *agentIdentityCacher) InstanceType() (string, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.instanceType != "" {
		return c.instanceType, nil
	}

	var err error
	c.instanceType, err = c.client.InstanceType()
	return c.instanceType, err
}

func (c *agentIdentityCacher) CredentialsProvider() aws.CredentialsProvider {
	// Fast path: read the cached provider under RLock so concurrent callers
	// (InstanceID, Region, etc.) that share this mutex don't serialize behind
	// the network-I/O path below. We check credsProvider — not creds.HasKeys —
	// because the provider itself is what callers use to Retrieve; caching the
	// Retrieve result here would require holding the mutex across the network
	// call to c.client.CredentialsProvider() for on-prem (a full RMIRT chain),
	// which would block every other identity-metadata reader for the duration
	// of that network round-trip.
	c.mutex.Lock()
	if c.credsProvider != nil {
		provider := c.credsProvider
		c.mutex.Unlock()
		return provider
	}
	c.mutex.Unlock()

	// Slow path: resolve the provider without the mutex held. Concurrent
	// callers may race to construct one; the last writer wins. The underlying
	// providers are all safe to construct multiple times (EC2 identity returns
	// a preassigned pointer; on-prem/azure return a fresh CredentialsProvider
	// struct whose network I/O only fires on Retrieve, not on construction).
	provider := c.client.CredentialsProvider()

	c.mutex.Lock()
	// If another goroutine set credsProvider while we were resolving ours,
	// prefer the one already installed so all callers see a stable identity.
	if c.credsProvider == nil {
		c.credsProvider = provider
	}
	provider = c.credsProvider
	c.mutex.Unlock()
	return provider
}

func (c *agentIdentityCacher) IdentityType() string {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.identityType != "" {
		return c.identityType
	}

	c.identityType = c.client.IdentityType()
	return c.identityType
}

func (c *agentIdentityCacher) GetServiceEndpoint(service string) string {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	var region string
	var err error

	if region, err = c.regionInner(); err != nil {
		c.log.Warnf("failed to get region with err: %v", err)
	}

	return c.endpointHelper.GetServiceEndpoint(service, region)
}

func (c *agentIdentityCacher) GetInner() identity.IAgentIdentityInner {
	return c.client
}
