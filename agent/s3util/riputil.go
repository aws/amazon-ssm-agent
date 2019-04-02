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

// Parts of this file are automatically updated and should not be edited.

// Package s3util contains methods for interacting with S3.
package s3util

import "github.com/aws/amazon-ssm-agent/agent/appconfig"
import "github.com/aws/amazon-ssm-agent/agent/platform"

var awsS3EndpointMap = map[string]string{
	//AUTOGEN_START
	"ap-northeast-1": "s3-ap-northeast-1.amazonaws.com",
	"ap-northeast-2": "s3.ap-northeast-2.amazonaws.com",
	"ap-south-1":     "s3.ap-south-1.amazonaws.com",
	"ap-southeast-1": "s3-ap-southeast-1.amazonaws.com",
	"ap-southeast-2": "s3-ap-southeast-2.amazonaws.com",
	"ca-central-1":   "s3.ca-central-1.amazonaws.com",
	"cn-north-1":     "s3.cn-north-1.amazonaws.com.cn",
	"eu-central-1":   "s3.eu-central-1.amazonaws.com",
	"eu-west-1":      "s3-eu-west-1.amazonaws.com",
	"eu-west-2":      "s3.eu-west-2.amazonaws.com",
	"sa-east-1":      "s3-sa-east-1.amazonaws.com",
	"us-east-1":      "s3.amazonaws.com",
	"us-east-2":      "s3.us-east-2.amazonaws.com",
	"us-gov-west-1":  "s3-us-gov-west-1.amazonaws.com",
	"us-west-1":      "s3-us-west-1.amazonaws.com",
	"us-west-2":      "s3-us-west-2.amazonaws.com",
	//AUTOGEN_END
}

/* This function returns the s3 endpoint specified by the user in appconfig.
If the user didn't specify one, it will return the Amazon S3 endpoint in a certain region
*/
func GetS3Endpoint(region string) (s3Endpoint string) {
	if appConfig, err := appconfig.Config(false); err == nil {
		if appConfig.S3.Endpoint != "" {
			return appConfig.S3.Endpoint
		}
	}

	if s3Endpoint, ok := awsS3EndpointMap[region]; ok {
		return s3Endpoint
	}

	if region, err := platform.Region(); err == nil {
		if defaultEndpoint := appconfig.GetDefaultEndPoint(region, "s3"); defaultEndpoint != "" {
			return defaultEndpoint
		}
	}
	return "s3.amazonaws.com" // default global endpoint
}

/*
This function will get the generic S3 endpoint for a certain region.
Most regions will use us-east-1 endpoint except special ones
*/
func GetS3GenericEndPoint(region string) (s3Endpoint string) {
	if region == "us-gov-west-1" {
		return GetS3Endpoint(region) // Restricted regions
	}
	if region == "cn-north-1" {
		return GetS3Endpoint("cn-north-1") // Use cn-north-1 for China
	}
	return GetS3Endpoint("us-east-1") // For all other regions, use us-east-1
}