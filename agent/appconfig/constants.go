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
	"os"
	"syscall"
)

const (
	// Agent defaults
	DefaultAgentName   = "amazon-ssm-agent"
	SSMAgentWorkerName = "ssm-agent-worker"

	DefaultTelemetryNamespace = "amazon-ssm-agent-telemetry"

	DefaultCommandWorkersLimit    = 5
	DefaultCommandWorkersLimitMin = 1

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

	DefaultSsmSelfUpdateFrequencyDays    = 7
	DefaultSsmSelfUpdateFrequencyDaysMin = 1 //Minimum frequency is 1 day
	DefaultSsmSelfUpdateFrequencyDaysMax = 7 //Maximum frequency is 7 day

	//aws-ssm-agent bookkeeping constants
	DefaultLocationOfPending     = "pending"
	DefaultLocationOfCurrent     = "current"
	DefaultLocationOfCompleted   = "completed"
	DefaultLocationOfCorrupt     = "corrupt"
	DefaultLocationOfState       = "state"
	DefaultLocationOfAssociation = "association"

	// PluginLocalOutputCleanup
	// Delete plugin output file locally after plugin execution
	PluginLocalOutputCleanupAfterExecution = "after-execution"
	// Delete plugin output locally after successful s3 or cloudWatch upload
	PluginLocalOutputCleanupAfterUpload = "after-upload"
	// Don't delete logs immediately after execution. Fall back to AssociationLogsRetentionDurationHours,
	// RunCommandLogsRetentionDurationHours, and SessionLogsRetentionDurationHours
	DefaultPluginOutputRetention = "default"

	//aws-ssm-agent state and orchestration logs duration for Run Command and Association
	DefaultAssociationLogsRetentionDurationHours           = 24  // 1 day default retention
	DefaultRunCommandLogsRetentionDurationHours            = 336 // 14 days default retention
	DefaultSessionLogsRetentionDurationHours               = 336 // 14 days default retention
	DefaultStateOrchestrationLogsRetentionDurationHoursMin = 8   // Min retention of 8hrs as some processes may not timeout before this and don't want logs to be deleted before the process completes

	DefaultAuditExpirationDay    = 7  // 7 days default audit files count
	DefaultAuditExpirationDayMax = 30 // 30 days max audit files count
	DefaultAuditExpirationDayMin = 3  // 3 days min audit files count

	//aws-ssm-agent bookkeeping constants for long running plugins
	LongRunningPluginsLocation         = "longrunningplugins"
	LongRunningPluginsHealthCheck      = "healthcheck"
	LongRunningPluginDataStoreLocation = "datastore"
	LongRunningPluginDataStoreFileName = "store"
	PluginNameLongRunningPluginInvoker = "lrpminvoker"

	//aws-ssm-agent bookkeeping constants for inventory plugin
	InventoryRootDirName         = "inventory"
	CustomInventoryRootDirName   = "custom"
	FileInventoryRootDirName     = "file"
	RoleInventoryRootDirName     = "role"
	InventoryContentHashFileName = "contentHash"

	//aws-ssm-agent bookkeeping constants for failed sent replies
	RepliesRootDirName = "replies"

	//aws-ssm-agent bookkeeping constants for compliance
	ComplianceRootDirName         = "compliance"
	ComplianceContentHashFileName = "contentHash"

	// DefaultDocumentRootDirName is the root directory for storing command states
	DefaultDocumentRootDirName = "document"

	// DefaultSessionRootDirName is the root directory for storing session manager data
	DefaultSessionRootDirName = "session"

	// Orchestration Root Dir
	defaultOrchestrationRootDirName = "orchestration"

	// ConfigurationRootDirName - the configuration folder used in ec2 config
	ConfigurationRootDirName = "Configuration"

	// WorkersRootDirName  - the worker folder used in ec2 config
	WorkersRootDirName = "Workers"

	defaultLongRunningWorkerMonitorIntervalSeconds    = 60
	defaultLongRunningWorkerMonitorIntervalSecondsMin = 30
	defaultLongRunningWorkerMonitorIntervalSecondsMax = 1800

	defaultProfileKeyAutoRotateDays    = 0
	defaultProfileKeyAutoRotateDaysMin = 0
	defaultProfileKeyAutoRotateDaysMax = 365

	// Permissions defaults
	//NOTE: Limit READ, WRITE and EXECUTE access to administrators/root.
	ReadWriteAccess        = 0600
	ReadWriteExecuteAccess = 0700

	// Common file flags when opening/creating files
	FileFlagsCreateOrAppend          = os.O_APPEND | os.O_WRONLY | os.O_CREATE
	FileFlagsCreateOrTruncate        = os.O_TRUNC | os.O_WRONLY | os.O_CREATE
	FileFlagsCreateOrAppendReadWrite = os.O_APPEND | os.O_RDWR | os.O_CREATE

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

	// PluginDownloadContent is the name for downloadContent plugin
	PluginDownloadContent = "aws:downloadContent"

	// PluginRunDocument is the name of the run document plugin
	PluginRunDocument = "aws:runDocument"

	// PluginNameAwsSoftwareInventory is the name for inventory plugin
	PluginNameAwsSoftwareInventory = "aws:softwareInventory"

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

	// PluginNameAwsPowerShellModule is the name of the PowerShell Module
	PluginNameAwsPowerShellModule = "aws:psModule"

	// PluginNameAwsApplications is the name of the Applications plugin
	PluginNameAwsApplications = "aws:applications"

	AppConfigFileName    = "amazon-ssm-agent.json"
	SeelogConfigFileName = "seelog.xml"

	// Output truncation limits
	MaxStdoutLength = 24000
	MaxStderrLength = 8000

	// Session worker defaults
	DefaultSessionWorkersLimit    = 1000
	DefaultSessionWorkersLimitMin = 1

	// PluginNameStandardStream is the name for session manager standard stream plugin aka shell.
	PluginNameStandardStream = "Standard_Stream"

	// PluginNameInteractiveCommands is the name for session manager interactive commands plugin.
	PluginNameInteractiveCommands = "InteractiveCommands"

	// PluginNameNonInteractiveCommands is the name for session manager non-interactive commands plugin.
	PluginNameNonInteractiveCommands = "NonInteractiveCommands"

	// PluginNamePort is the name for session manager port plugin.
	PluginNamePort = "Port"

	// Session default RunAs user name
	DefaultRunAsUserName = "ssm-user"
)

// Document versions that are supported by this Agent version.
// Note that 1.1 and 2.1 are deprecated schemas and hence are not added here.
// Version 2.0.1, 2.0.2, and 2.0.3 are added to support install documents for configurePackage
// that require capabilities that did not exist before the build where support for these versions was added
var SupportedDocumentVersions = map[string]struct{}{
	"1.0":   {},
	"1.2":   {},
	"2.0":   {},
	"2.0.1": {},
	"2.0.2": {},
	"2.0.3": {},
	"2.2":   {},
}

// Session Manager Document versions that are supported by this Agent version.
var SupportedSessionDocumentVersions = map[string]struct{}{
	"1.0": {},
}

// All the control signals to handles interrupt input from SSM CLI
// SIGINT captures Ctrl+C
// SIGQUIT captures Ctrl+\
var ByteControlSignalsLinux = map[byte]os.Signal{
	'\003': syscall.SIGINT,
	'\x1c': syscall.SIGQUIT,
}

// All the input control messages that can be transformed to SIGKILL signal on Windows platforms
// Windows platforms do not support SIGINT or SIGQUIT signals.
// It only processes SIGKILL signal, which is translated to taskkill command on the process.
var ByteControlSignalsWindows = map[byte]os.Signal{
	'\003': syscall.SIGKILL,
	'\x1c': syscall.SIGKILL,
}
