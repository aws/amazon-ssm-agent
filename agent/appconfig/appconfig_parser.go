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
)

//func parser(config *T) {
func parser(config *SsmagentConfig) {
	log.Printf("processing appconfig overrides")

	// Agent config
	config.Agent.Name = getStringValue(config.Agent.Name, DefaultAgentName)
	config.Agent.OrchestrationRootDir = getStringValue(config.Agent.OrchestrationRootDir, defaultOrchestrationRootDirName)
	config.Agent.Region = getStringValue(config.Agent.Region, "")

	// MDS config
	config.Mds.CommandWorkersLimit = getNumericValue(
		config.Mds.CommandWorkersLimit,
		DefaultCommandWorkersLimitMin,
		DefaultCommandWorkersLimitMax,
		DefaultCommandWorkersLimit)
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
}

func getStringValue(configValue string, defaultValue string) string {
	if configValue == "" {
		return defaultValue
	}
	return configValue
}

func getNumericValue(configValue int, minValue int, maxValue int, defaultValue int) int {
	if configValue < minValue || configValue > maxValue {
		return defaultValue
	}
	return configValue
}

func getNumeric64Value(configValue int64, minValue int64, maxValue int64, defaultValue int64) int64 {
	if configValue < minValue || configValue > maxValue {
		return defaultValue
	}
	return configValue
}
