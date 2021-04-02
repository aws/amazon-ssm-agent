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

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
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

func newAgentIdentityInner(log log.T, config *appconfig.SsmagentConfig, selector IAgentIdentitySelector, identitySelectionOrder []string, identityGenerators map[string]createIdentityFunc) (IAgentIdentity, error) {
	var agentIdentity IAgentIdentityInner

	var selectedIdentityFunc createIdentityFunc
	var found bool

	if config.Agent.ContainerMode {
		if selectedIdentityFunc, found = identityGenerators["ECS"]; !found {
			return nil, fmt.Errorf("ECS identity does not exist")
		}

		log.Info("Agent will take identity from ECS")
		agentIdentity = selector.selectAgentIdentity(selectedIdentityFunc(log, config), "ECS")
		if agentIdentity == nil {
			return nil, fmt.Errorf("failed to get identity from ECS metadata")
		}

		log.Info("Agent will take identity from ECS")
		return &agentIdentityCacher{
			log:    log,
			client: agentIdentity,
		}, nil
	}

	// Loop over all identity options and select the one that matches first
	for _, identityKey := range identitySelectionOrder {
		log.Debugf("Checking if agent has %s identity", identityKey)
		if selectedIdentityFunc, found = identityGenerators[identityKey]; !found {
			log.Warnf("Identity '%s' does not exist", identityKey)
			continue
		}

		// Testing if identity can be assumed
		agentIdentity = selector.selectAgentIdentity(selectedIdentityFunc(log, config), identityKey)
		if agentIdentity != nil {
			log.Infof("Agent will take identity from %s", identityKey)
			return &agentIdentityCacher{
				log:    log,
				client: agentIdentity,
			}, nil
		}
	}

	log.Errorf("Agent failed to assume any identity")
	return nil, fmt.Errorf("failed to find agent identity")
}

func NewAgentIdentity(log log.T, config *appconfig.SsmagentConfig, selector IAgentIdentitySelector) (identity IAgentIdentity, err error) {
	// TODO: move order to config after removing container mode flag
	var identitySelectionOrder = []string{"OnPrem", "EC2"}

	for i := 0; i < MaxRetriesIdentitySelector; i++ {
		identity, err = newAgentIdentityInner(log, config, selector, identitySelectionOrder, allIdentityGenerators)
		if err == nil {
			break
		}
		if i+1 < MaxRetriesIdentitySelector {
			log.Errorf("failed to find identity, retrying: %v", err)
		}
	}
	return
}

func (d *defaultAgentIdentitySelector) selectAgentIdentity(agentIdentities []IAgentIdentityInner, identityKey string) IAgentIdentityInner {
	for _, agentIdentity := range agentIdentities {
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
