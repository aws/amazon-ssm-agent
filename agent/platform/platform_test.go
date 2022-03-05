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

// Package platform contains platform specific utilities.
package platform

import (
	"fmt"
	"net"
	"testing"

	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

func TestInvalidPlatform(t *testing.T) {
	getPlatformNameFn = func(log logger.T) (value string, err error) {
		return "Microsoft \xa9 sample R2 Server", nil
	}
	logObj := logger.NewMockLog()
	name, err := PlatformName(logObj)
	assert.Equal(t, "Microsoft  sample R2 Server", name)
	assert.Nil(t, err)
}

func TestValidPlatform(t *testing.T) {
	getPlatformNameFn = func(log logger.T) (value string, err error) {
		return "Microsoft sample R2 \u00a9 Server", nil
	}
	logObj := logger.NewMockLog()
	name, err := PlatformName(logObj)
	assert.Equal(t, "Microsoft sample R2 © Server", name)
	assert.Nil(t, err)
}

func TestSimpleValidUnixPlatform(t *testing.T) {
	getPlatformNameFn = func(log logger.T) (value string, err error) {
		return "Amazon Linux", nil
	}
	logObj := logger.NewMockLog()
	name, err := PlatformName(logObj)
	assert.Equal(t, "Amazon Linux", name)
	assert.Nil(t, err)
}

func TestPlatformWithErr(t *testing.T) {
	getPlatformNameFn = func(log logger.T) (value string, err error) {
		return "Microsoft \xa9 sample R2 Server", fmt.Errorf("test")
	}
	logObj := logger.NewMockLog()
	name, err := PlatformName(logObj)
	assert.Equal(t, "Microsoft \xa9 sample R2 Server", name)
	assert.NotNil(t, err)
}

func TestSelectIp_NoAddresses_ReturnsError(t *testing.T) {
	actual, err := selectIp([]net.IP{})
	assert.NotNil(t, err)
	assert.Nil(t, actual)
}

func TestSelectIp_SingleAddress_ReturnsTheAddress(t *testing.T) {
	candidates := []net.IP{
		net.IPv4(10, 0, 0, 1),
	}
	actual, err := selectIp(candidates)
	assert.Nil(t, err)
	assert.Equal(t, candidates[0], actual)
}

func TestSelectIp_V4AndV6_ReturnsV4(t *testing.T) {
	candidates := []net.IP{
		{0x20, 0x01, 0, 0, 0, 0, 0, 0, 0, 0},
		net.IPv4(10, 0, 0, 1),
	}
	actual, err := selectIp(candidates)
	assert.Nil(t, err)
	assert.Equal(t, candidates[1], actual)
}

func TestSelectIp_LinkLocalAndNonLinkLocal_ReturnsNonLinkLocal(t *testing.T) {
	candidates := []net.IP{
		net.IPv4(169, 254, 0, 1),
		net.IPv4(10, 0, 0, 1),
	}
	actual, err := selectIp(candidates)
	assert.Nil(t, err)
	assert.Equal(t, candidates[1], actual)
}

func TestSelectIp_LoopbackAndNonLoopback_ReturnsNonLoopback(t *testing.T) {
	candidates := []net.IP{
		net.IPv6loopback,
		{0x20, 0x01, 0, 0, 0, 0, 0, 0, 0, 0},
		net.IPv4(127, 0, 0, 1),
	}
	actual, err := selectIp(candidates)
	assert.Nil(t, err)
	assert.Equal(t, candidates[1], actual)
}

func TestSelectIp_OnlyLinkLocalAndLoopback_ReturnsFirstOne(t *testing.T) {
	candidates := []net.IP{
		net.IPv4(169, 254, 0, 1),
		net.IPv4(169, 254, 0, 2),
		net.IPv6linklocalallnodes,
		net.IPv6loopback,
	}
	actual, err := selectIp(candidates)
	assert.Nil(t, err)
	assert.Equal(t, candidates[0], actual)
}

func TestSelectIp_IgnoresNils(t *testing.T) {
	candidates := []net.IP{
		net.IPv4(169, 254, 0, 1),
		nil,
		net.IPv4(10, 0, 0, 1),
	}
	actual, err := selectIp(candidates)
	assert.Nil(t, err)
	assert.Equal(t, candidates[2], actual)
}
