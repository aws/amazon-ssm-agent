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
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/customidentity"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ecs"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/onprem"
)

func init() {
	allIdentityGenerators = make(map[string]createIdentityFunc)

	allIdentityGenerators[ec2.IdentityType] = newEC2Identity
	allIdentityGenerators[ecs.IdentityType] = newECSIdentity
	allIdentityGenerators[onprem.IdentityType] = newOnPremIdentity
	allIdentityGenerators[customidentity.IdentityType] = newCustomIdentity
}

func newEC2Identity(log log.T, config *appconfig.SsmagentConfig) []IAgentIdentityInner {
	id := ec2.NewEC2Identity(log)
	if id == nil {
		return nil
	}

	return []IAgentIdentityInner{id}
}

func newECSIdentity(log log.T, _ *appconfig.SsmagentConfig) []IAgentIdentityInner {
	return []IAgentIdentityInner{&ecs.Identity{
		Log: log,
	}}
}

func newOnPremIdentity(log log.T, config *appconfig.SsmagentConfig) []IAgentIdentityInner {
	return []IAgentIdentityInner{onprem.NewOnPremIdentity(log, config)}
}

func newCustomIdentity(log log.T, config *appconfig.SsmagentConfig) []IAgentIdentityInner {
	var customIdentities []IAgentIdentityInner

	for index, customIdentityEntry := range config.Identity.CustomIdentities {
		if customIdentityEntry.InstanceID == "" {
			log.Warnf("The InstanceID provided as part of CustomIdentity cannot be empty. Skipping custom identity #%d", index)
			continue
		}
		if customIdentityEntry.Region == "" {
			log.Warnf("The Region provided as part of CustomIdentity cannot be empty. Skipping custom identity #%d", index)
			continue
		}
		log.Debugf("Creating custom identity object for instance id '%s' in region '%s'",
			customIdentityEntry.InstanceID,
			customIdentityEntry.Region)
		customIdentities = append(customIdentities, &customidentity.Identity{
			CustomIdentity: *customIdentityEntry,
			Log:            log,
		})
	}

	return customIdentities
}
