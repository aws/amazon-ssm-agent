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

	"github.com/aws/amazon-ssm-agent/common/identity/endpoint"
	"github.com/aws/aws-sdk-go/aws/credentials"
)

const (
	MaxInstanceIDLen = 40
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

func (c *agentIdentityCacher) Credentials() *credentials.Credentials {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.creds != nil {
		return c.creds
	}

	c.creds = c.client.Credentials()
	return c.creds
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

func (c *agentIdentityCacher) GetDefaultEndpoint(service string) string {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	var region, serviceDomain string
	var err error

	if region, err = c.regionInner(); err != nil {
		c.log.Warnf("failed to get region with err: %v", err)
	}

	serviceDomain, _ = c.client.ServiceDomain()

	return endpoint.GetDefaultEndpoint(c.log, service, region, serviceDomain)
}
