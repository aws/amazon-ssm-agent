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

package runtimeconfig

import (
	"encoding/json"
	"fmt"
	"time"

	rch "github.com/aws/amazon-ssm-agent/common/runtimeconfig/runtimeconfighandler"
)

const (
	identityConfig = "identity_config.json"
)

type IdentityRuntimeConfig struct {
	InstanceId             string
	IdentityType           string
	ShareFile              string
	ShareProfile           string
	CredentialsExpiresAt   time.Time
	CredentialsRetrievedAt time.Time
}

func (i IdentityRuntimeConfig) Equal(config IdentityRuntimeConfig) bool {
	sameId := i.InstanceId == config.InstanceId
	sameProfile := i.ShareProfile == config.ShareProfile
	sameFile := i.ShareFile == config.ShareFile
	sameType := i.IdentityType == config.IdentityType

	return sameId && sameProfile && sameFile && sameType
}

func NewIdentityRuntimeConfigClient() IIdentityRuntimeConfigClient {
	return &identityRuntimeConfigClient{
		configHandler: rch.NewRuntimeConfigHandler(identityConfig),
	}
}

type IIdentityRuntimeConfigClient interface {
	ConfigExists() (bool, error)
	GetConfig() (IdentityRuntimeConfig, error)
	SaveConfig(IdentityRuntimeConfig) error
}

type identityRuntimeConfigClient struct {
	configHandler rch.IRuntimeConfigHandler
}

func (i *identityRuntimeConfigClient) ConfigExists() (bool, error) {
	return i.configHandler.ConfigExists()
}

func (i *identityRuntimeConfigClient) GetConfig() (IdentityRuntimeConfig, error) {
	var config IdentityRuntimeConfig

	bytesContent, err := i.configHandler.GetConfig()
	if err != nil {
		return config, err
	}

	err = json.Unmarshal(bytesContent, &config)
	if err != nil {
		return config, fmt.Errorf("error decoding identity runtime config: %v", err)
	}

	return config, nil
}

func (i *identityRuntimeConfigClient) SaveConfig(config IdentityRuntimeConfig) error {
	bytesContent, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("error encoding identity runtime config: %v", err)
	}

	err = i.configHandler.SaveConfig(bytesContent)

	if err != nil {
		return err
	}

	// Because of the importance of identityRuntimeConfig, we want to make sure the file is readable after writing
	savedConfig, err := i.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to validate config is readable after writing: %v", err)
	}

	// verify saved config and config to be saved are equivalent
	if !savedConfig.Equal(config) {
		return fmt.Errorf("failed to verify config on disk is equivalent to the config that was saved")
	}

	return nil
}
