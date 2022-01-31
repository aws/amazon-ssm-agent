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
package endpoint

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

var logMock = log.NewMockLog()

var regionServiceDomainMap = map[string]string{
	"af-south-1":     "amazonaws.com",    // CPT
	"ap-east-1":      "amazonaws.com",    // HKG
	"ap-northeast-1": "amazonaws.com",    // NRT
	"ap-northeast-2": "amazonaws.com",    // ICN
	"ap-northeast-3": "amazonaws.com",    // KIX
	"ap-south-1":     "amazonaws.com",    // BOM
	"ap-southeast-1": "amazonaws.com",    // SIN
	"ap-southeast-2": "amazonaws.com",    // SYD
	"ap-southeast-3": "amazonaws.com",    // CGK
	"ca-central-1":   "amazonaws.com",    // YUL
	"cn-north-1":     "amazonaws.com.cn", // BJS
	"cn-northwest-1": "amazonaws.com.cn", // ZHY
	"eu-central-1":   "amazonaws.com",    // FRA
	"eu-north-1":     "amazonaws.com",    // ARN
	"eu-south-1":     "amazonaws.com",    // MXP
	"eu-west-1":      "amazonaws.com",    // DUB
	"eu-west-2":      "amazonaws.com",    // LHR
	"eu-west-3":      "amazonaws.com",    // CDG
	"me-south-1":     "amazonaws.com",    // BAH
	"sa-east-1":      "amazonaws.com",    // GRU
	"us-east-1":      "amazonaws.com",    // IAD
	"us-east-2":      "amazonaws.com",    // CMH
	"us-gov-east-1":  "amazonaws.com",    // OSU
	"us-gov-west-1":  "amazonaws.com",    // PDT
	"us-iso-east-1":  "c2s.ic.gov",       // DCA
	"us-iso-west-1":  "c2s.ic.gov",       // APA
	"us-isob-east-1": "sc2s.sgov.gov",    // LCK
	"us-west-1":      "amazonaws.com",    // SFO
	"us-west-2":      "amazonaws.com",    // PDX

	"us-future-1":      "amazonaws.com",
	"cn-future-1":      "amazonaws.com.cn",
	"us-isob-future-1": "sc2s.sgov.gov",
	"us-iso-future-1":  "c2s.ic.gov",
}

var testServices = []string{
	"ssm",
	"ssmmessages",
	"ec2messages",
	"kms",
	"logs",
	"monitor",
	"s3",
}

func TestGetServiceEndPoint(t *testing.T) {
	config := appconfig.DefaultConfig()
	e := &endpointImpl{
		logMock,
		config,
		sync.RWMutex{},
		map[string]map[string]string{},
	}

	for _, service := range testServices {
		for region, serviceDomain := range regionServiceDomainMap {
			assert.Equal(t, "", e.endpointCacheLookup(service, region))

			expectedEndpoint := service + "." + region + "." + serviceDomain
			endpoint := e.GetServiceEndpoint(service, region)

			assert.Equal(t, expectedEndpoint, endpoint)
			assert.Equal(t, endpoint, e.endpointCacheLookup(service, region))
		}
	}
}

func TestGetServiceEndpoint_ServiceDomainConfigSet(t *testing.T) {
	serviceDomain := "some.service.domain"

	config := appconfig.SsmagentConfig{}
	config.Agent.ServiceDomain = serviceDomain

	e := &endpointImpl{
		logMock,
		config,
		sync.RWMutex{},
		map[string]map[string]string{},
	}

	for _, service := range testServices {
		for region, _ := range regionServiceDomainMap {
			assert.Equal(t, "", e.endpointCacheLookup(service, region))

			expectedEndpoint := service + "." + region + "." + serviceDomain
			endpoint := e.GetServiceEndpoint(service, region)

			assert.Equal(t, expectedEndpoint, endpoint)
			assert.Equal(t, endpoint, e.endpointCacheLookup(service, region))
		}
	}
}

func TestGetServiceEndpoint_RegionNotInPrefixMap(t *testing.T) {
	oldMap := regionPrefixServiceDomain
	regionPrefixServiceDomain = map[string]string{}
	defer func() { regionPrefixServiceDomain = oldMap }()

	e := &endpointImpl{
		logMock,
		appconfig.SsmagentConfig{},
		sync.RWMutex{},
		map[string]map[string]string{},
	}

	for _, service := range testServices {
		for region, _ := range regionServiceDomainMap {
			assert.Equal(t, "", e.endpointCacheLookup(service, region))
			assert.Equal(t, service+"."+region+"."+defaultServiceDomain, e.GetServiceEndpoint(service, region))
			assert.Equal(t, service+"."+region+"."+defaultServiceDomain, e.endpointCacheLookup(service, region))
		}
	}
}

func TestGetServiceDomainByPrefix(t *testing.T) {
	for region, _ := range regionServiceDomainMap {
		serviceDomain := GetServiceDomainByPrefix(region)
		assert.Equal(t, regionServiceDomainMap[region], serviceDomain)
	}
}

func TestGetServiceDomainByPrefix_VerifyNoConflictInPrefixMap(t *testing.T) {
	for prefix1, _ := range regionPrefixServiceDomain {
		for prefix2, _ := range regionPrefixServiceDomain {
			if prefix1 != prefix2 {
				assert.False(t, strings.HasPrefix(prefix2, prefix1), "regionPrefixServiceDomain has conflicting prefixes %s and %s which could lead to nondeterministic behavior", prefix2, prefix1)
			}
		}
	}
}

func TestEndpointCache(t *testing.T) {
	region1 := "some-region-1"
	region2 := "some-other-region-1"
	service1 := "someService"
	service2 := "someOtherService"
	endpoint1 := "SomeEndpoint.com"
	endpoint2 := "SomeOtherEndpoint.com"
	config := appconfig.SsmagentConfig{}
	e := &endpointImpl{
		logMock,
		config,
		sync.RWMutex{},
		map[string]map[string]string{},
	}

	// Test empty cache
	assert.Equal(t, 0, len(e.regionServiceEndpointCache))
	assert.Equal(t, "", e.endpointCacheLookup(service1, ""))
	assert.Equal(t, "", e.endpointCacheLookup("", region1))

	// Test adding to cache
	e.setEndpointCache(service1, region1, endpoint1)
	assert.Equal(t, endpoint1, e.endpointCacheLookup(service1, region1))
	assert.Equal(t, 1, len(e.regionServiceEndpointCache))
	assert.Equal(t, 1, len(e.regionServiceEndpointCache[region1]))

	// Test overriting cache
	e.setEndpointCache(service1, region1, endpoint2)
	assert.Equal(t, endpoint2, e.endpointCacheLookup(service1, region1))
	assert.Equal(t, 1, len(e.regionServiceEndpointCache))
	assert.Equal(t, 1, len(e.regionServiceEndpointCache[region1]))

	// Test adding new service
	e.setEndpointCache(service2, region1, endpoint1)
	assert.Equal(t, endpoint1, e.endpointCacheLookup(service2, region1))
	assert.Equal(t, 1, len(e.regionServiceEndpointCache))
	assert.Equal(t, 2, len(e.regionServiceEndpointCache[region1]))

	// Test adding new region
	e.setEndpointCache(service2, region2, endpoint2)
	assert.Equal(t, endpoint2, e.endpointCacheLookup(service2, region2))
	assert.Equal(t, 2, len(e.regionServiceEndpointCache))
	assert.Equal(t, 2, len(e.regionServiceEndpointCache[region1]))
	assert.Equal(t, 1, len(e.regionServiceEndpointCache[region2]))
}

func TestIsRegionValid(t *testing.T) {
	var tests = map[string]bool{
		"a-a-1":                                 true,
		"us-eastsomethingverylong-10":           true,
		"us-isob-eastsomethingverylong-1000000": true,
		"some-new-format-with-more-sections":    true,
		"a-1":                                   true,
		"a":                                     true,
		"z":                                     true,
		"-":                                     true,
		"us-east-test-extra-1":                  true,
		"us-east-test-extra-1d":                 true,
		"us-east-test-1d":                       true,

		"":                false,
		"us-east-test=1":  false,
		"us-east-test_1":  false,
		"us-east-test/1":  false,
		"us-east-test\\1": false,
	}

	config := appconfig.SsmagentConfig{}
	e := &endpointImpl{
		logMock,
		config,
		sync.RWMutex{},
		map[string]map[string]string{},
	}
	for region, shouldPass := range tests {
		result := e.isRegionValid(region)
		assert.Equal(t, shouldPass, result, fmt.Sprintf("region '%s' should have returned %v but returned %v", region, shouldPass, result))
	}

	tooLongRegion := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-1"
	assert.False(t, e.isRegionValid(tooLongRegion))

	maxLengthRegion := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-1"
	assert.True(t, e.isRegionValid(maxLengthRegion))
}
