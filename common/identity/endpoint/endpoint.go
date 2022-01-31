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
	"regexp"
	"strings"
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

// Map defining region prefixes and the default service domain for prefix, keys are processed in random order
var regionPrefixServiceDomain = map[string]string{
	"cn-":      "amazonaws.com.cn",
	"us-iso-":  "c2s.ic.gov",
	"us-isob-": "sc2s.sgov.gov",
}

// default service domain if prefix does not exist in awsFallbackServiceDomain map
const (
	defaultServiceDomain = "amazonaws.com"

	regionMaxLength = 100
)

// The following regex only allows a-z upper/lower case characters, digits and dashes for region strings
var supportedRegionReg = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

type IEndpointHelper interface {
	// GetServiceEndpoint returns the endpoint for a service in a specific region
	GetServiceEndpoint(service, region string) string
}

func GetServiceDomainByPrefix(region string) string {
	for regionPrefix, serviceDomain := range regionPrefixServiceDomain {
		if strings.HasPrefix(region, regionPrefix) {
			return serviceDomain
		}
	}

	return defaultServiceDomain
}

type endpointImpl struct {
	log    log.T
	config appconfig.SsmagentConfig

	cacheLock                  sync.RWMutex
	regionServiceEndpointCache map[string]map[string]string
}

func (e *endpointImpl) isRegionValid(region string) bool {
	return len(region) <= regionMaxLength && supportedRegionReg.Match([]byte(region))
}

func (e *endpointImpl) endpointCacheLookup(service, region string) string {
	e.cacheLock.RLock()
	defer e.cacheLock.RUnlock()
	if serviceEndpointCache, ok := e.regionServiceEndpointCache[region]; ok {
		if endpoint, ok := serviceEndpointCache[service]; ok {
			return endpoint
		}
	}
	return ""
}

func (e *endpointImpl) setEndpointCache(service, region, endpoint string) {
	e.cacheLock.Lock()
	defer e.cacheLock.Unlock()
	if _, ok := e.regionServiceEndpointCache[region]; !ok {
		e.regionServiceEndpointCache[region] = map[string]string{}
	}

	e.regionServiceEndpointCache[region][service] = endpoint
}

func (e *endpointImpl) GetServiceEndpoint(service, region string) string {
	e.log.Debugf("Determining endpoint for service %s in region %s", service, region)
	var serviceDomain string
	if region == "" {
		// If region is not defined, we are unable to determine endpoint for the service
		e.log.Errorf("Cannot get endpoint for service %s due to unspecified region.", service)
		return ""
	} else if service == "" {
		// If service is not defined, we are unable to determine endpoint
		e.log.Errorf("Cannot get endpoint for service in region %s due to unspecified service.", service)
		return ""
	} else if !e.isRegionValid(region) {
		e.log.Errorf("Unable to determine endpoint because region %s has invalid characters", region)
		return ""
	} else if endpoint := e.endpointCacheLookup(service, region); endpoint != "" {
		return endpoint
	}

	if e.config.Agent.ServiceDomain != "" {
		serviceDomain = e.config.Agent.ServiceDomain
	} else {
		serviceDomain = GetServiceDomainByPrefix(region)
	}

	// Build the full endpoint for the service in the region
	endpoint := service + "." + region + "." + serviceDomain
	e.setEndpointCache(service, region, endpoint)
	return endpoint
}

func NewEndpointHelper(log log.T, config appconfig.SsmagentConfig) *endpointImpl {
	return &endpointImpl{
		log,
		config,
		sync.RWMutex{},
		map[string]map[string]string{},
	}
}
