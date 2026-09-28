// Copyright 2025 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package config

import (
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity"
)

func IsTelemetryEnabled(log log.T, identity identity.IAgentIdentity, appConfig appconfig.SsmagentConfig) bool {
	region, err := identity.Region()
	if err != nil {
		log.Warnf("Could not determine region")
		return false
	}

	isAwsPartition, err := isRegionInAwsPartition(region)
	if err != nil {
		log.Warnf("Could not determine partition for region %s: %v", region, err)
		return false
	}

	if !isAwsPartition {
		log.Debugf("Region %s is not in AWS standard partition", region)
		return false
	}

	if !appConfig.Agent.GlobalEnhancedTelemetryEnabled {
		log.Info("Agent GlobalEnhancedTelemetry is disabled")
		return false
	}

	return true
}

func isRegionInAwsPartition(region string) (bool, error) {
	if region == "" {
		return false, nil
	}

	// Check if the region belongs to a non-commercial partition.
	// Unknown regions default to false (disabled) for safety.
	nonCommercialPrefixes := []string{
		"cn-",      // aws-cn (China)
		"us-gov-",  // aws-us-gov (GovCloud)
		"us-iso-",  // aws-iso (ISO)
		"us-isob-", // aws-iso-b (ISO-B)
		"us-isof-", // aws-iso-f (ISO-F)
		"eu-isoe-", // aws-iso-e (ISO-E)
		"eusc-de-", // aws-eusc (European Sovereign Cloud)
	}
	for _, prefix := range nonCommercialPrefixes {
		if strings.HasPrefix(region, prefix) {
			return false, nil
		}
	}

	// Only treat known commercial region prefixes as aws partition
	commercialPrefixes := []string{
		"us-east-", "us-west-", "eu-west-", "eu-central-", "eu-north-", "eu-south-",
		"ap-southeast-", "ap-northeast-", "ap-south-", "ap-east-",
		"sa-east-", "ca-central-", "ca-west-", "me-south-", "me-central-",
		"af-south-", "il-central-", "ap-southeast-",
	}
	for _, prefix := range commercialPrefixes {
		if strings.HasPrefix(region, prefix) {
			return true, nil
		}
	}

	// Unknown region — default to disabled (safe)
	return false, nil
}
