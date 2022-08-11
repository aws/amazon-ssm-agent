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
	"fmt"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity/endpoint"
	"github.com/aws/amazon-ssm-agent/common/runtimeconfig"
)

// NewDefaultAgentIdentitySelector creates a instance of a default agent identity selector
func NewDefaultAgentIdentitySelector(log log.T) IAgentIdentitySelector {
	return &defaultAgentIdentitySelector{
		log: log,
	}
}

// NewInstanceIDRegionAgentIdentitySelector creates a instance of a default agent identity selector
func NewInstanceIDRegionAgentIdentitySelector(log log.T, instanceID, region string) IAgentIdentitySelector {
	return &instanceIDRegionAgentIdentitySelector{
		log:        log,
		instanceID: instanceID,
		region:     region,
	}
}

func NewRuntimeConfigIdentitySelector(log log.T) IAgentIdentitySelector {
	return &runtimeConfigIdentitySelector{
		log:                 log,
		configClient:        runtimeconfig.NewIdentityRuntimeConfigClient(),
		config:              runtimeconfig.IdentityRuntimeConfig{},
		isConfigInitialized: false,
	}
}

func newAgentIdentityInner(log log.T, config *appconfig.SsmagentConfig, selector IAgentIdentitySelector, identitySelectionOrder []string, identityGenerators map[string]createIdentityFunc) (IAgentIdentity, error) {
	var agentIdentity IAgentIdentityInner

	var selectedIdentityFunc createIdentityFunc
	var found bool

	// For backwards compatibility, if container mode is enabled and the identity consumption order is not overwritten in the config, default to ECS identity only
	if config.Agent.ContainerMode && isDefaultIdentityConsumptionOrder(identitySelectionOrder, appconfig.DefaultIdentityConsumptionOrder) {
		if selectedIdentityFunc, found = identityGenerators["ECS"]; !found {
			return nil, fmt.Errorf("ECS identity does not exist")
		}

		agentIdentity = selector.selectAgentIdentity(selectedIdentityFunc(log, config), "ECS")
		if agentIdentity == nil {
			return nil, fmt.Errorf("failed to get identity from ECS metadata")
		}

		log.Info("Agent will take identity from ECS")
		return &agentIdentityCacher{
			log:            log,
			client:         agentIdentity,
			endpointHelper: endpoint.NewEndpointHelper(log, *config),
		}, nil
	}

	// Loop over all identity options and select the one that matches first
	for _, identityKey := range identitySelectionOrder {
		log.Debugf("Checking if agent has %s identity type", identityKey)
		if selectedIdentityFunc, found = identityGenerators[identityKey]; !found {
			log.Warnf("Identity type '%s' does not exist", identityKey)
			continue
		}

		// Testing if identity can be assumed
		log.Infof("Checking if agent identity type %s can be assumed", identityKey)
		agentIdentity = selector.selectAgentIdentity(selectedIdentityFunc(log, config), identityKey)
		if agentIdentity != nil {
			log.Infof("Agent will take identity from %s", identityKey)
			return &agentIdentityCacher{
				log:            log,
				client:         agentIdentity,
				endpointHelper: endpoint.NewEndpointHelper(log, *config),
			}, nil
		}
	}

	log.Errorf("Agent failed to assume any identity")
	return nil, fmt.Errorf("failed to find agent identity")
}

func NewAgentIdentity(log log.T, config *appconfig.SsmagentConfig, selector IAgentIdentitySelector) (identity IAgentIdentity, err error) {
	for i := 0; i < maxRetriesIdentitySelector; i++ {
		identity, err = newAgentIdentityInner(log, config, selector, config.Identity.ConsumptionOrder, allIdentityGenerators)
		if err == nil {
			break
		}
		if i+1 < maxRetriesIdentitySelector {
			log.Errorf("failed to find identity, retrying: %v", err)

			// Sleep 500ms in case of IMDS not being online yet
			time.Sleep(sleepBeforeRetry)
		}

	}
	return
}

func (d *defaultAgentIdentitySelector) selectAgentIdentity(agentIdentities []IAgentIdentityInner, identityKey string) IAgentIdentityInner {
	for _, agentIdentity := range agentIdentities {
		if agentIdentity == nil {
			d.log.Errorf("'%s' identity failed to create", identityKey)
			continue
		}

		if agentIdentity.IsIdentityEnvironment() {
			return agentIdentity
		}
	}
	d.log.Debugf("'%s' identity is not available on this instance", identityKey)
	return nil
}

func (d *instanceIDRegionAgentIdentitySelector) selectAgentIdentity(agentIdentities []IAgentIdentityInner, identityKey string) IAgentIdentityInner {
	var instanceID, region string
	var err error

	for _, agentIdentity := range agentIdentities {
		if !agentIdentity.IsIdentityEnvironment() {
			d.log.Debugf("'%s' identity is not available on this instance", identityKey)
			continue
		}

		if d.instanceID != "" {
			instanceID, err = agentIdentity.InstanceID()
			if err != nil {
				d.log.Warnf("Failed to get instance id from '%s' identity: %v", identityKey, err)
				continue
			}

			if instanceID != d.instanceID {
				d.log.Debugf("'%s' identity has instance ID '%s' and not '%s'", identityKey, instanceID, d.instanceID)
				continue
			}
		}

		if d.region != "" {
			region, err = agentIdentity.Region()
			if err != nil {
				d.log.Warnf("Failed to get region from '%s' identity: %v", identityKey, err)
				continue
			}

			if region != d.region {
				d.log.Debugf("'%s' identity has region '%s' and not '%s'", identityKey, instanceID, d.instanceID)
				continue
			}
		}
		return agentIdentity
	}
	return nil
}

func isDefaultIdentityConsumptionOrder(identitySelectionOrder, defaultIdentitySelectionOrder []string) bool {
	// If lengths are not the same, the identity selection order is not default
	if len(identitySelectionOrder) != len(defaultIdentitySelectionOrder) {
		return false
	}

	// If any element does not match in the two lists, the identity selection order is not default
	for i := 0; i < len(identitySelectionOrder); i++ {
		if identitySelectionOrder[i] != defaultIdentitySelectionOrder[i] {
			return false
		}
	}

	return true
}

func (d *runtimeConfigIdentitySelector) selectAgentIdentity(agentIdentities []IAgentIdentityInner, identityKey string) IAgentIdentityInner {
	var err error
	if !d.isConfigInitialized {
		d.config, err = d.configClient.GetConfig()
		if err != nil {
			d.log.Warnf("%v", err)
			return nil
		}

		d.isConfigInitialized = true
	}

	if identityKey != d.config.IdentityType {
		return nil
	}

	for _, agentIdentity := range agentIdentities {
		instanceId, err := agentIdentity.InstanceID()
		if err != nil || instanceId != d.config.InstanceId {
			// Failed to get instance id or instance id is not the same as stored in runtime config
			continue
		}

		return agentIdentity
	}

	return nil
}
