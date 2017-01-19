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

import "os"

const (
	// Agent defaults
	DefaultAgentName = "amazon-ssm-agent"

	DefaultCommandWorkersLimit    = 1
	DefaultCommandWorkersLimitMin = 1
	DefaultCommandWorkersLimitMax = 10

	DefaultCommandRetryLimit    = 15
	DefaultCommandRetryLimitMin = 1
	DefaultCommandRetryLimitMax = 100

	DefaultStopTimeoutMillis    = 20000
	DefaultStopTimeoutMillisMin = 10000
	DefaultStopTimeoutMillisMax = 1000000

	// SSM defaults
	DefaultSsmHealthFrequencyMinutes    = 5
	DefaultSsmHealthFrequencyMinutesMin = 5
	DefaultSsmHealthFrequencyMinutesMax = 60

	DefaultSsmAssociationFrequencyMinutes    = 10
	DefaultSsmAssociationFrequencyMinutesMin = 5
	DefaultSsmAssociationFrequencyMinutesMax = 60

	//aws-ssm-agent bookkeeping constants
	DefaultLocationOfPending     = "pending"
	DefaultLocationOfCurrent     = "current"
	DefaultLocationOfCompleted   = "completed"
	DefaultLocationOfCorrupt     = "corrupt"
	DefaultLocationOfState       = "state"
	DefaultLocationOfAssociation = "association"

	//aws-ssm-agent bookkeeping constants for long running plugins
	LongRunningPluginsLocation         = "longrunningplugins"
	LongRunningPluginsHealthCheck      = "healthcheck"
	LongRunningPluginDataStoreLocation = "datastore"
	LongRunningPluginDataStoreFileName = "store"
	PluginNameLongRunningPluginInvoker = "lrpminvoker"

	//aws-ssm-agent bookkeeping constants for inventory plugin
	InventoryRootDirName         = "inventory"
	CustomInventoryRootDirName   = "custom"
	InventoryContentHashFileName = "contentHash"

	// DefaultDocumentRootDirName is the root directory for storing command states
	DefaultDocumentRootDirName = "document"

	// Orchestration Root Dir
	defaultOrchestrationRootDirName = "orchestration"

	// ConfigurationRootDirName - the configuration folder used in ec2 config
	ConfigurationRootDirName = "Configuration"

	// WorkersRootDirName  - the worker folder used in ec2 config
	WorkersRootDirName = "Workers"

	// Permissions defaults
	//NOTE: Limit READ, WRITE and EXECUTE access to administrators/root.
	ReadWriteAccess        = 0600
	ReadWriteExecuteAccess = 0700

	// Common file flags when opening/creating files
	FileFlagsCreateOrAppend   = os.O_APPEND | os.O_WRONLY | os.O_CREATE
	FileFlagsCreateOrTruncate = os.O_TRUNC | os.O_WRONLY | os.O_CREATE

	// ExitCodes
	SuccessExitCode = 0
	ErrorExitCode   = 1

	// DefaultPluginConfig is a default config with which the plugins are initialized
	DefaultPluginConfig = "aws:defaultPluginConfig"

	// List all plugin names, unfortunately golang doesn't support const arrays of strings

	// PluginNameAwsConfigureDaemon is the name for configure daemon plugin
	PluginNameAwsConfigureDaemon = "aws:configureDaemon"

	// PluginNameAwsConfigurePackage is the name for configure package plugin
	PluginNameAwsConfigurePackage = "aws:configurePackage"

	// PluginNameAwsRunShellScript is the name for run shell script plugin
	PluginNameAwsRunShellScript = "aws:runShellScript"

	// PluginNameAwsRunPowerShellScript is the name of the run powershell script plugin
	PluginNameAwsRunPowerShellScript = "aws:runPowerShellScript"

	// PluginNameAwsAgentUpdate is the name for agent update plugin
	PluginNameAwsAgentUpdate = "aws:updateSsmAgent"

	// PluginEC2ConfigUpdate is the name for ec2 config update plugin
	PluginEC2ConfigUpdate = "aws:updateAgent"

	// PluginNameAwsSoftwareInventory is the name for inventory plugin
	PluginNameAwsSoftwareInventory = "aws:softwareInventory"

	AppConfigFileName    = "amazon-ssm-agent.json"
	SeelogConfigFileName = "seelog.xml"

	// PluginNameDomainJoin is the name of domain join plugin
	PluginNameDomainJoin = "aws:domainJoin"

	// PluginNameCloudWatch is the name of cloud watch plugin
	PluginNameCloudWatch = "aws:cloudWatch"

	// PluginNameRunDockerAction is the name of the docker container plugin
	PluginNameDockerContainer = "aws:runDockerAction"

	// PluginNameConfigureDocker is the name of the configure Docker plugin
	PluginNameConfigureDocker = "aws:configureDocker"

	// PluginNameRefreshAssociation is the name of refresh association plugin
	PluginNameRefreshAssociation = "aws:refreshAssociation"
)
