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
	"fmt"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/common/identity/endpoint"
)

const defaultGlobalEndpoint = "s3.amazonaws.com"

/*
	This function returns the s3 endpoint specified by the user in appconfig.

If the user didn't specify one, it will return the Amazon S3 endpoint in a certain region
*/
func GetS3Endpoint(context context.T, region string) (s3Endpoint string, err error) {
	log := context.Log()

	appConfig := context.AppConfig()
	if appConfig.S3.Endpoint != "" {
		return appConfig.S3.Endpoint, nil
	}

	// Get the service endpoint for the region passed in, if it is return it
	endpointHelper := endpoint.NewEndpointHelper(context.Log(), appConfig)
	if serviceEndpoint := endpointHelper.GetServiceEndpoint("s3", region); serviceEndpoint != "" {
		return serviceEndpoint, nil
	}

	if defaultEndpoint := context.Identity().GetServiceEndpoint("s3"); defaultEndpoint != "" {
		return defaultEndpoint, nil
	}

	log.Warnf("Failed to get S3 endpoint for region %v.", region)
	return "", fmt.Errorf("could not fetch s3 endpoint")
}

// Returns an alternate S3 endpoint in the same partition as
// the specified region.
func getFallbackS3Endpoint(context context.T, region string) (s3Endpoint string) {
	if strings.HasPrefix(region, "us-gov-") {
		if region == "us-gov-west-1" {
			s3Endpoint, _ = GetS3Endpoint(context, "us-gov-east-1")
		} else {
			s3Endpoint, _ = GetS3Endpoint(context, "us-gov-west-1")
		}
	} else if strings.HasPrefix(region, "cn-") {
		if region == "cn-north-1" {
			s3Endpoint, _ = GetS3Endpoint(context, "cn-northwest-1")
		} else {
			s3Endpoint, _ = GetS3Endpoint(context, "cn-north-1")
		}
	} else {
		s3Endpoint = defaultGlobalEndpoint
	}
	return s3Endpoint
}
