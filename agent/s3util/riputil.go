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

import (
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/platform"
)

var awsS3EndpointMap = map[string]string{
	//AUTOGEN_START
	"af-south-1":     "s3.af-south-1.amazonaws.com",
	"ap-east-1":      "s3.ap-east-1.amazonaws.com",
	"ap-northeast-1": "s3.ap-northeast-1.amazonaws.com",
	"ap-northeast-2": "s3.ap-northeast-2.amazonaws.com",
	"ap-northeast-3": "s3.ap-northeast-3.amazonaws.com",
	"ap-south-1":     "s3.ap-south-1.amazonaws.com",
	"ap-southeast-1": "s3.ap-southeast-1.amazonaws.com",
	"ap-southeast-2": "s3.ap-southeast-2.amazonaws.com",
	"ca-central-1":   "s3.ca-central-1.amazonaws.com",
	"cn-north-1":     "s3.cn-north-1.amazonaws.com.cn",
	"cn-northwest-1": "s3.cn-northwest-1.amazonaws.com.cn",
	"eu-central-1":   "s3.eu-central-1.amazonaws.com",
	"eu-north-1":     "s3.eu-north-1.amazonaws.com",
	"eu-south-1":     "s3.eu-south-1.amazonaws.com",
	"eu-west-1":      "s3.eu-west-1.amazonaws.com",
	"eu-west-2":      "s3.eu-west-2.amazonaws.com",
	"eu-west-3":      "s3.eu-west-3.amazonaws.com",
	"me-south-1":     "s3.me-south-1.amazonaws.com",
	"sa-east-1":      "s3.sa-east-1.amazonaws.com",
	"us-east-1":      "s3.us-east-1.amazonaws.com",
	"us-east-2":      "s3.us-east-2.amazonaws.com",
	"us-gov-east-1":  "s3.us-gov-east-1.amazonaws.com",
	"us-gov-west-1":  "s3.us-gov-west-1.amazonaws.com",
	"us-west-1":      "s3.us-west-1.amazonaws.com",
	"us-west-2":      "s3.us-west-2.amazonaws.com",
	//AUTOGEN_END
}

const defaultGlobalEndpoint = "s3.amazonaws.com"

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
		if defaultEndpoint := platform.GetDefaultEndPoint(region, "s3"); defaultEndpoint != "" {
			return defaultEndpoint
		}
	}
	return defaultGlobalEndpoint
}

// Returns an alternate S3 endpoint in the same partition as
// the specified region.
func getFallbackS3Endpoint(region string) (s3Endpoint string) {
	if strings.HasPrefix(region, "us-gov-") {
		if region == "us-gov-west-1" {
			s3Endpoint = GetS3Endpoint("us-gov-east-1")
		} else {
			s3Endpoint = GetS3Endpoint("us-gov-west-1")
		}
	} else if strings.HasPrefix(region, "cn-") {
		if region == "cn-north-1" {
			s3Endpoint = GetS3Endpoint("cn-northwest-1")
		} else {
			s3Endpoint = GetS3Endpoint("cn-north-1")
		}
	} else {
		s3Endpoint = defaultGlobalEndpoint
	}
	return
}

// Tests whether the given string is a known region name (e.g. us-east-1)
func isKnownRegion(val string) bool {
	_, found := awsS3EndpointMap[val]
	return found
}
