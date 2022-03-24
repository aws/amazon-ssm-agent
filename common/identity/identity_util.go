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
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/onprem"
)

// IsOnPremInstance returns true if the agent identity is onprem
func IsOnPremInstance(agentIdentity IAgentIdentity) bool {
	return agentIdentity != nil && agentIdentity.IdentityType() == onprem.IdentityType
}

// IsEC2Instance return true if the agent identity is ec2
func IsEC2Instance(agentIdentity IAgentIdentity) bool {
	return agentIdentity != nil && agentIdentity.IdentityType() == ec2.IdentityType
}

// GetCredentialsRefresherIdentity returns the credentials refresher interface if the identity supports it
func GetCredentialsRefresherIdentity(agentIdentity IAgentIdentity) (ICredentialRefresherAgentIdentity, bool) {
	var innerGetter iInnerIdentityGetter
	var ok bool

	// Cast to innerIdentityGetter interface that defined getInner
	innerGetter, ok = agentIdentity.(iInnerIdentityGetter)
	if !ok {
		return nil, false
	}

	// Attempt to cast inner identity to CredentialsRefresher
	var credentialIdentity ICredentialRefresherAgentIdentity
	credentialIdentity, ok = innerGetter.getInner().(ICredentialRefresherAgentIdentity)
	return credentialIdentity, ok
}
