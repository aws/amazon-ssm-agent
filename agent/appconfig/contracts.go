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

// CredentialProfile represents configurations for aws credential profile
type CredentialProfile struct {
	ShareCreds        bool
	ShareProfile      string
	ForceUpdateCreds  bool
	KeyAutoRotateDays int
}

// MdsCfg represents configuration for Message delivery service (MDS)
type MdsCfg struct {
	Endpoint                 string
	CommandWorkersLimit      int
	CommandWorkerBufferLimit int
	StopTimeoutMillis        int64
	CommandRetryLimit        int
}

// SsmCfg represents configuration for Simple system manager (SSM)
type SsmCfg struct {
	Endpoint                       string
	HealthFrequencyMinutes         int
	AssociationFrequencyMinutes    int
	AssociationRetryLimit          int
	CustomInventoryDefaultLocation string
	// Hours to retain association logs in the orchestration folder
	AssociationLogsRetentionDurationHours int
	// Hours to retain run command logs in the orchestration folder
	RunCommandLogsRetentionDurationHours int
	// Hours to retain session logs in the orchestration folder
	SessionLogsRetentionDurationHours int
	// Configure when after execution it is safe to delete local plugin output files in orchestration folder
	PluginLocalOutputCleanup string
	// Configure only when it is safe to delete orchestration folder after document execution. This config overrides PluginLocalOutputCleanup when set.
	OrchestrationDirectoryCleanup string
}

// AgentInfo represents metadata for amazon-ssm-agent
type AgentInfo struct {
	Name                                    string
	Version                                 string
	Region                                  string
	ServiceDomain                           string
	OrchestrationRootDir                    string
	DownloadRootDir                         string
	ContainerMode                           bool
	SelfUpdate                              bool
	SelfUpdateScheduleDay                   int
	TelemetryMetricsToCloudWatch            bool
	TelemetryMetricsToSSM                   bool
	TelemetryMetricsNamespace               string
	LongRunningWorkerMonitorIntervalSeconds int
	AuditExpirationDay                      int
	ForceFileIPC                            bool
	// denotes GOMAXPROCS value for legacy agent worker
	GoMaxProcForAgentWorker int
}

// MgsConfig represents configuration for Message Gateway service
type MgsConfig struct {
	Region                        string
	Endpoint                      string
	StopTimeoutMillis             int64
	SessionWorkersLimit           int
	SessionWorkerBufferLimit      int
	DeniedPortForwardingRemoteIPs []string
}

// KmsConfig represents configuration for Key Management Service
type KmsConfig struct {
	Endpoint string
}

// OsInfo represents os related information
type OsInfo struct {
	Lang    string
	Name    string
	Version string
}

// S3Cfg represents configurations related to S3 bucket and key for SSM
type S3Cfg struct {
	Endpoint  string
	Region    string
	LogBucket string
	LogKey    string
}

// BirdwatcherCfg represents configuration related to ConfigurePackage Birdwatcher integration
type BirdwatcherCfg struct {
	ForceEnable bool
}

// SsmagentConfig stores agent configuration values.
type SsmagentConfig struct {
	Profile     CredentialProfile
	Mds         MdsCfg
	Ssm         SsmCfg
	Mgs         MgsConfig
	Agent       AgentInfo
	Os          OsInfo
	S3          S3Cfg
	Birdwatcher BirdwatcherCfg
	Kms         KmsConfig
	Identity    IdentityCfg
}

// AppConstants represents some run time constant variable for various module.
// Currently it only contains HealthCheck module constants for health ping frequency
type AppConstants struct {
	MinHealthFrequencyMinutes int
	MaxHealthFrequencyMinutes int
}

// CustomIdentity defines a single custom identity that the agent can assume
type CustomIdentity struct {
	InstanceID          string
	Region              string
	AvailabilityZone    string
	AvailabilityZoneId  string
	InstanceType        string
	CredentialsProvider string
}

// IdentityCfg stores identity consumption order and custom identities
type IdentityCfg struct {
	Ec2SystemInfoDetectionResponse string
	ConsumptionOrder               []string
	CustomIdentities               []*CustomIdentity
}
