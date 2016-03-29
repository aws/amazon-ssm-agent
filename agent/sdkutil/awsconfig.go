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

// Package sdkutil provides utilities used to call awssdk.
package sdkutil

import (
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/aws-sdk-go/aws"
)

// GetAwsConfig : Default AWS config populates with default region and credentials.
// Callers should override returned config properties with any values they want for service specific overrides.
func GetAwsConfig() (awsConfig *aws.Config) {
	config := aws.Config{
		Retryer:    retryer(),
		SleepDelay: sleepDelay,
	}
	awsConfig = config.WithLogLevel(aws.LogDebugWithRequestRetries | aws.LogDebugWithRequestErrors)
	region := platform.Region()
	if region != "" {
		awsConfig.Region = &region
	}
	appConfig, err := appconfig.GetConfig(false)
	if err == nil {
		creds, _ := appConfig.ProfileCredentials()
		if creds != nil {
			awsConfig.Credentials = creds
		}
	}
	return
}

var retryer = func() aws.RequestRetryer {
	r := SsmRetryer{}
	r.NumMaxRetries = 3
	return r
}

var sleepDelay = func(d time.Duration) {
	time.Sleep(d)
}
