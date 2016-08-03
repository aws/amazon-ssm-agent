// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package appconfig manages the configuration of the agent.
package appconfig

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

	//aws-ssm-agent bookkeeping constants
	DefaultLocationOfPending   = "pending"
	DefaultLocationOfCurrent   = "current"
	DefaultLocationOfCompleted = "completed"
	DefaultLocationOfCorrupt   = "corrupt"
	DefaultLocationOfState     = "state"
	// DefaultCommandRootDirName is the root directory for storing command states
	DefaultCommandRootDirName = "command"

	// Orchestration Root Dir
	defaultOrchestrationRootDirName = "orchestration"

	// Permissions defaults
	//NOTE: Limit READ, WRITE and EXECUTE access to administrators/root.
	ReadWriteAccess        = 0600
	ReadWriteExecuteAccess = 0700

	// ExitCodes
	SuccessExitCode = 0
	ErrorExitCode   = 1

	// DefaultPluginConfig is a default config with which the plugins are initialized
	DefaultPluginConfig = "aws:defaultPluginConfig"

	// List all plugin names, unfortunately golang doesn't support const arrays of strings

	// PluginNameAWSConfigureComponent is the name for configure component plugin
	PluginNameAwsConfigureComponent = "aws:configureComponent"

	// PluginNameAwsAgentUpdate is the name for agent update plugin
	PluginNameAwsAgentUpdate = "aws:updateSsmAgent"

	AppConfigFileName    = "amazon-ssm-agent.json"
	SeelogConfigFileName = "seelog.xml"
)
