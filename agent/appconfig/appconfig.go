// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package appconfig manages the configuration of the agent.
package appconfig

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/aws-sdk-go/aws/credentials"
)

var loadedConfig *SsmagentConfig
var lock sync.RWMutex

// Config loads the app configuration for amazon-ssm-agent.
// If reload is true, it loads the config afresh,
// otherwise it returns a previous loaded version, if any.
func Config(reload bool) (SsmagentConfig, error) {
	if reload || !isLoaded() {
		var agentConfig SsmagentConfig
		agentConfig = DefaultConfig()
		path, pathErr := getAppConfigPath()
		if pathErr != nil {
			return agentConfig, nil
		}

		// Process config override
		fmt.Printf("Applying config override from %s.\n", path)

		if err := jsonutil.UnmarshalFile(path, &agentConfig); err != nil {
			fmt.Println("Failed to unmarshal config override. Fall back to default.")
			return agentConfig, err
		}
		agentConfig.Os.Name = runtime.GOOS
		agentConfig.Agent.Version = version.Version
		parser(&agentConfig)
		cache(agentConfig)
	}
	return getCached(), nil
}

func isLoaded() bool {
	lock.RLock()
	defer lock.RUnlock()
	return loadedConfig != nil
}

func cache(config SsmagentConfig) {
	lock.Lock()
	defer lock.Unlock()
	loadedConfig = &config
}

func getCached() SsmagentConfig {
	lock.RLock()
	defer lock.RUnlock()
	return *loadedConfig
}

// ProfileCredentials checks to see if specific profile is being asked to use
func (config SsmagentConfig) ProfileCredentials() (credsInConfig *credentials.Credentials, err error) {
	// the credentials file location and profile to load
	credsInConfig = credentials.NewSharedCredentials(config.Profile.Path, config.Profile.Name)
	_, err = credsInConfig.Get()
	if err != nil {
		return nil, err
	}
	fmt.Printf("Using AWS credentials configured under %v user profile \n", config.Profile.Name)
	return
}

// looks for appconfig in working directory first and then the platform specific folder
func getAppConfigPath() (path string, err error) {
	// looking for appconfig in the platform specific folder
	if _, err = os.Stat(AppConfigPath); err != nil {
		return "", err
	}

	log.Printf("Found config file at %s.\n", AppConfigPath)
	return AppConfigPath, err
}

// DefaultConfig returns default ssm agent configuration
func DefaultConfig() SsmagentConfig {

	var credsProfile = CredentialProfile{
		ShareCreds: true,
	}
	var s3 S3Cfg
	var mds = MdsCfg{
		CommandWorkersLimit: 5,
		StopTimeoutMillis:   20000,
		CommandRetryLimit:   15,
	}
	var ssm = SsmCfg{
		HealthFrequencyMinutes:         5,
		AssociationFrequencyMinutes:    10,
		AssociationRetryLimit:          5,
		CustomInventoryDefaultLocation: DefaultCustomInventoryFolder,
	}
	var agent = AgentInfo{
		Name:                 "amazon-ssm-agent",
		OrchestrationRootDir: defaultOrchestrationRootDirName,
	}
	var os = OsInfo{
		Lang:    "en-US",
		Version: "1",
	}

	var ssmagentCfg = SsmagentConfig{
		Profile: credsProfile,
		Mds:     mds,
		Ssm:     ssm,
		Agent:   agent,
		Os:      os,
		S3:      s3,
	}

	return ssmagentCfg
}
