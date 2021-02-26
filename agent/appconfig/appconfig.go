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
)

var (
	loadedConfig *SsmagentConfig
	lock         sync.RWMutex

	retrieveAppConfigPath = getAppConfigPath
)

// Config loads the app configuration for amazon-ssm-agent.
// If reload is true, it loads the config afresh,
// otherwise it returns a previous loaded version, if any.
func Config(reload bool) (SsmagentConfig, error) {
	if reload || !isLoaded() {
		var agentConfig SsmagentConfig
		agentConfig = DefaultConfig()
		path, pathErr := retrieveAppConfigPath()
		if pathErr != nil {
			return agentConfig, nil
		}
		agentConfig.Os.Name = runtime.GOOS
		agentConfig.Agent.Version = version.Version

		// Process config override
		fmt.Printf("Applying config override from %s.\n", path)

		if err := jsonutil.UnmarshalFile(path, &agentConfig); err != nil {
			fmt.Println("Failed to unmarshal config override. Fall back to default.")
			return agentConfig, err
		}
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
		ShareCreds:        true,
		KeyAutoRotateDays: defaultProfileKeyAutoRotateDays,
	}
	var s3 S3Cfg
	var mds = MdsCfg{
		CommandWorkersLimit: DefaultCommandWorkersLimit,
		StopTimeoutMillis:   DefaultStopTimeoutMillis,
		CommandRetryLimit:   DefaultCommandRetryLimit,
	}
	var mgs = MgsConfig{
		SessionWorkersLimit: DefaultSessionWorkersLimit,
		StopTimeoutMillis:   DefaultStopTimeoutMillis,
	}
	var ssm = SsmCfg{
		HealthFrequencyMinutes:                DefaultSsmHealthFrequencyMinutes,
		AssociationFrequencyMinutes:           DefaultSsmAssociationFrequencyMinutes,
		AssociationRetryLimit:                 5,
		CustomInventoryDefaultLocation:        DefaultCustomInventoryFolder,
		AssociationLogsRetentionDurationHours: DefaultAssociationLogsRetentionDurationHours,
		RunCommandLogsRetentionDurationHours:  DefaultRunCommandLogsRetentionDurationHours,
		SessionLogsRetentionDurationHours:     DefaultSessionLogsRetentionDurationHours,
		PluginLocalOutputCleanup:              DefaultPluginOutputRetention,
	}
	var agent = AgentInfo{
		Name:                                    "amazon-ssm-agent",
		OrchestrationRootDir:                    defaultOrchestrationRootDirName,
		ContainerMode:                           false,
		SelfUpdate:                              false,
		TelemetryMetricsToCloudWatch:            false,
		TelemetryMetricsToSSM:                   true,
		TelemetryMetricsNamespace:               DefaultTelemetryNamespace,
		AuditExpirationDay:                      DefaultAuditExpirationDay,
		LongRunningWorkerMonitorIntervalSeconds: defaultLongRunningWorkerMonitorIntervalSeconds,
		ForceFileIPC:                            false,
	}
	var os = OsInfo{
		Lang:    "en-US",
		Version: "1",
	}
	var birdwatcher BirdwatcherCfg
	var kms KmsConfig

	var ssmagentCfg = SsmagentConfig{
		Profile:     credsProfile,
		Mds:         mds,
		Ssm:         ssm,
		Mgs:         mgs,
		Agent:       agent,
		Os:          os,
		S3:          s3,
		Birdwatcher: birdwatcher,
		Kms:         kms,
	}

	return ssmagentCfg
}
