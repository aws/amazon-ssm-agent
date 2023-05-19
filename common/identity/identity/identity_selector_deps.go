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
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/aws/amazon-ssm-agent/common/runtimeconfig"
)

const (
	maxRetriesIdentitySelector = 3
	sleepBeforeRetry           = 500 * time.Millisecond
)

type defaultAgentIdentitySelector struct {
	log   log.T
	mutex sync.Mutex
}

type instanceIDRegionAgentIdentitySelector struct {
	log        log.T
	instanceID string
	region     string
	mutex      sync.Mutex
}

type runtimeConfigIdentitySelector struct {
	log                 log.T
	configClient        runtimeconfig.IIdentityRuntimeConfigClient
	config              runtimeconfig.IdentityRuntimeConfig
	isConfigInitialized bool
}

// IAgentIdentitySelector abstracts logic to select an agent identity
type IAgentIdentitySelector interface {
	SelectAgentIdentity([]identity.IAgentIdentityInner, string) identity.IAgentIdentityInner
}
