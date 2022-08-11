// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package runtimeconfiginit

import (
	"github.com/aws/amazon-ssm-agent/agent/backoffconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/aws/amazon-ssm-agent/common/runtimeconfig"
	"github.com/cenkalti/backoff/v4"
)

var backoffRetry = backoff.Retry

type IRuntimeConfigInit interface {
	Init() error
}

type runtimeConfigInit struct {
	log           log.T
	backoffConfig *backoff.ExponentialBackOff

	agentIdentity        identity.IAgentIdentity
	identityConfigClient runtimeconfig.IIdentityRuntimeConfigClient
}

func (r *runtimeConfigInit) saveIdentityConfigWithRetry(currentConfig runtimeconfig.IdentityRuntimeConfig) error {
	return backoffRetry(func() error {
		r.log.Debugf("Trying to save identity runtime config")
		err := r.identityConfigClient.SaveConfig(currentConfig)
		if err != nil {
			r.log.Warnf("Failed to save runtime config: %v", err)
		}
		return err
	}, r.backoffConfig)
}

func (r *runtimeConfigInit) getCurrentIdentityRuntimeConfig() (runtimeconfig.IdentityRuntimeConfig, error) {
	var currentConfig runtimeconfig.IdentityRuntimeConfig
	var err error

	currentConfig.IdentityType = r.agentIdentity.IdentityType()
	currentConfig.InstanceId, err = r.agentIdentity.InstanceID()

	if err != nil {
		return currentConfig, err
	}

	if credentialsRefresherIdentity, ok := identity.GetRemoteProvider(r.agentIdentity); ok {
		currentConfig.ShareFile = credentialsRefresherIdentity.ShareFile()
		currentConfig.ShareProfile = credentialsRefresherIdentity.ShareProfile()
	}

	return currentConfig, nil
}

func (r *runtimeConfigInit) initIdentityRuntimeConfig() error {
	currentConfig, err := r.getCurrentIdentityRuntimeConfig()
	if err != nil {
		return err
	}

	var savedConfig runtimeconfig.IdentityRuntimeConfig
	if ok, err := r.identityConfigClient.ConfigExists(); err != nil {
		// failed to check if runtime config exists, initialize new one anyways
		r.log.Warnf("failed to check identity runtime config during init: %v", err)
	} else if ok {
		if savedConfig, err = r.identityConfigClient.GetConfig(); err != nil {
			r.log.Warnf("failed to read identity runtime config during init: %v", err)
		}
	} // else config does not exist

	// If saved config and current config are not equal, save the current runtime config
	if !savedConfig.Equal(currentConfig) {
		return r.saveIdentityConfigWithRetry(currentConfig)
	}

	return nil
}

func (r *runtimeConfigInit) Init() error {
	var err error
	r.backoffConfig, err = backoffconfig.GetDefaultExponentialBackoff()
	if err != nil {
		return err
	}

	return r.initIdentityRuntimeConfig()
}

func New(log log.T, identity identity.IAgentIdentity) IRuntimeConfigInit {
	return &runtimeConfigInit{
		log,
		nil, // initialized in Init
		identity,
		runtimeconfig.NewIdentityRuntimeConfigClient(),
	}
}
