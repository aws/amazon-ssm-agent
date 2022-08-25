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

// OS specific appconfig validator will apply limits and assign default values

package appconfig

import (
	"log"
	"runtime"
)

// func parser(config *T) {
func parser(config *SsmagentConfig) {
	log.Printf("processing appconfig overrides")

	booleanStringOptions := []string{
		"true",
		"false",
	}

	// Agent creds profile
	config.Profile.KeyAutoRotateDays = getNumericValue(
		config.Profile.KeyAutoRotateDays,
		defaultProfileKeyAutoRotateDaysMin,
		defaultProfileKeyAutoRotateDaysMax,
		defaultProfileKeyAutoRotateDays)

	// Agent config
	config.Agent.Name = getStringValue(config.Agent.Name, DefaultAgentName)
	config.Agent.OrchestrationRootDir = getStringValue(config.Agent.OrchestrationRootDir, defaultOrchestrationRootDirName)
	config.Agent.Region = getStringValue(config.Agent.Region, "")
	config.Agent.ServiceDomain = getStringValue(config.Agent.ServiceDomain, "")
	config.Agent.TelemetryMetricsNamespace = getStringValue(config.Agent.TelemetryMetricsNamespace, DefaultTelemetryNamespace)
	config.Agent.LongRunningWorkerMonitorIntervalSeconds = getNumericValue(
		config.Agent.LongRunningWorkerMonitorIntervalSeconds,
		defaultLongRunningWorkerMonitorIntervalSecondsMin,
		defaultLongRunningWorkerMonitorIntervalSecondsMax,
		defaultLongRunningWorkerMonitorIntervalSeconds)
	config.Agent.SelfUpdateScheduleDay = getNumericValue(
		config.Agent.SelfUpdateScheduleDay,
		DefaultSsmSelfUpdateFrequencyDaysMin,
		DefaultSsmSelfUpdateFrequencyDaysMax,
		DefaultSsmSelfUpdateFrequencyDays)
	config.Agent.GoMaxProcForAgentWorker = getNumericValue(config.Agent.GoMaxProcForAgentWorker,
		1,
		runtime.NumCPU(),
		0)

	config.Agent.AuditExpirationDay = getNumericValue(
		config.Agent.AuditExpirationDay,
		DefaultAuditExpirationDayMin,
		DefaultAuditExpirationDayMax,
		DefaultAuditExpirationDay)

	// MDS config
	config.Mds.CommandWorkersLimit = getNumericValue(
		config.Mds.CommandWorkersLimit,
		DefaultCommandWorkersLimitMin,
		config.Mds.CommandWorkersLimit, // we do not restrict max number of worker limit here
		DefaultCommandWorkersLimit)
	config.Mds.CommandWorkerBufferLimit = getNumericValue(
		config.Mds.CommandWorkerBufferLimit,
		DefaultCommandWorkersBufferLimitMin,
		config.Mds.CommandWorkerBufferLimit, // we do not restrict max number of worker buffer limit here
		DefaultCommandWorkerBufferLimit)

	// MGS config
	config.Mgs.SessionWorkerBufferLimit = getNumericValue(
		config.Mgs.SessionWorkerBufferLimit,
		DefaultSessionWorkersBufferLimitMin,
		config.Mgs.SessionWorkerBufferLimit, // we do not restrict max number of worker buffer limit here
		DefaultSessionWorkerBufferLimit)

	config.Mds.CommandRetryLimit = getNumericValue(
		config.Mds.CommandRetryLimit,
		DefaultCommandRetryLimitMin,
		DefaultCommandRetryLimitMax,
		DefaultCommandRetryLimit)
	config.Mds.StopTimeoutMillis = getNumeric64Value(
		config.Mds.StopTimeoutMillis,
		DefaultStopTimeoutMillisMin,
		DefaultStopTimeoutMillisMax,
		DefaultStopTimeoutMillis)
	config.Mds.Endpoint = getStringValue(config.Mds.Endpoint, "")

	// SSM config
	config.Ssm.Endpoint = getStringValue(config.Ssm.Endpoint, "")
	config.Ssm.HealthFrequencyMinutes = getNumericValue(
		config.Ssm.HealthFrequencyMinutes,
		DefaultSsmHealthFrequencyMinutesMin,
		DefaultSsmHealthFrequencyMinutesMax,
		DefaultSsmHealthFrequencyMinutes)
	config.Ssm.AssociationFrequencyMinutes = getNumericValue(
		config.Ssm.AssociationFrequencyMinutes,
		DefaultSsmAssociationFrequencyMinutesMin,
		DefaultSsmAssociationFrequencyMinutesMax,
		DefaultSsmAssociationFrequencyMinutes)
	config.Ssm.AssociationLogsRetentionDurationHours = getNumericValueAboveMin(
		config.Ssm.AssociationLogsRetentionDurationHours,
		DefaultStateOrchestrationLogsRetentionDurationHoursMin,
		DefaultAssociationLogsRetentionDurationHours)
	config.Ssm.RunCommandLogsRetentionDurationHours = getNumericValueAboveMin(
		config.Ssm.RunCommandLogsRetentionDurationHours,
		DefaultStateOrchestrationLogsRetentionDurationHoursMin,
		DefaultRunCommandLogsRetentionDurationHours)
	pluginLocalOutputCleanupOptions := []string{PluginLocalOutputCleanupAfterExecution,
		PluginLocalOutputCleanupAfterUpload,
		DefaultPluginOutputRetention}
	config.Ssm.PluginLocalOutputCleanup = getStringEnum(config.Ssm.PluginLocalOutputCleanup,
		pluginLocalOutputCleanupOptions,
		DefaultPluginOutputRetention)

	OrchestartionDirCleanupOtions := []string{
		DefaultOrchestrationDirCleanup,
		OrchestrationDirCleanupForSuccessFailedCommand,
		OrchestrationDirCleanupForSuccessCommand,
	}
	config.Ssm.OrchestrationDirectoryCleanup = getStringEnum(config.Ssm.OrchestrationDirectoryCleanup,
		OrchestartionDirCleanupOtions,
		DefaultOrchestrationDirCleanup)

	config.Identity.Ec2SystemInfoDetectionResponse = getStringEnum(config.Identity.Ec2SystemInfoDetectionResponse, booleanStringOptions, "")
	IdentityConsumptionOrderOptions := map[string]bool{
		"OnPrem":         true,
		"ECS":            true,
		"EC2":            true,
		"CustomIdentity": true,
	}
	config.Identity.ConsumptionOrder = getStringListEnum(
		config.Identity.ConsumptionOrder,
		IdentityConsumptionOrderOptions,
		DefaultIdentityConsumptionOrder)
	CredentialsProviderOptions := map[string]bool{
		DefaultCustomIdentityCredentialsProvider: true,
	}
	for _, customIdentity := range config.Identity.CustomIdentities {
		customIdentity.CredentialsProvider = getStringEnumMap(customIdentity.CredentialsProvider, CredentialsProviderOptions, DefaultCustomIdentityCredentialsProvider)
	}
}

// getStringValue returns the default value if config is empty, else the config value
func getStringValue(configValue string, defaultValue string) string {
	if configValue == "" {
		return defaultValue
	}
	return configValue
}

// getNumericValueAboveMin returns the default if config is below minimum
func getNumericValueAboveMin(configValue int, minValue int, defaultValue int) int {
	if configValue < minValue {
		return defaultValue
	}
	return configValue
}

// getNumericValue returns the default if config value is below min or above max
func getNumericValue(configValue int, minValue int, maxValue int, defaultValue int) int {
	if configValue < minValue || configValue > maxValue {
		return defaultValue
	}
	return configValue
}

// getNumeric64Value returns the default if config value is below min or above max
func getNumeric64Value(configValue int64, minValue int64, maxValue int64, defaultValue int64) int64 {
	if configValue < minValue || configValue > maxValue {
		return defaultValue
	}
	return configValue
}

func getStringEnum(configValue string, possibleValues []string, defaultValue string) string {
	if stringInList(configValue, possibleValues) {
		return configValue
	} else {
		return defaultValue
	}
}

func stringInList(targetString string, stringList []string) bool {
	for _, candidateString := range stringList {
		if candidateString == targetString {
			return true
		}
	}
	return false
}

// getStringListEnum removes invalid values from a list, if end list is empty, returns default list
func getStringListEnum(configValue []string, possibleValues map[string]bool, defaultValue []string) []string {
	var result []string

	for _, val := range configValue {
		if _, ok := possibleValues[val]; ok {
			result = append(result, val)
		}
	}
	if len(result) == 0 {
		return defaultValue
	}
	return result
}

// getStringEnumMap returns default if config value is not in possible values
func getStringEnumMap(configValue string, possibleValues map[string]bool, defaultValue string) string {
	if _, ok := possibleValues[configValue]; ok {
		return configValue
	}
	return defaultValue
}
