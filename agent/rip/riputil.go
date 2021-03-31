// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package rip contains AWS services regional endpoints.
package rip

import (
	"net/url"

	"github.com/aws/amazon-ssm-agent/agent/context"
)

const (
	MgsServiceName = "ssmmessages"
)

// TODO: remove rip-gen from s3util and use this shared file.
var awsMessageGatewayServiceEndpointMap = map[string]string{
	//AUTOGEN_START_MessageGatewayService
	"ap-northeast-1": "ssmmessages.ap-northeast-1.amazonaws.com",
	"ap-northeast-2": "ssmmessages.ap-northeast-2.amazonaws.com",
	"ap-south-1":     "ssmmessages.ap-south-1.amazonaws.com",
	"ap-southeast-1": "ssmmessages.ap-southeast-1.amazonaws.com",
	"ap-southeast-2": "ssmmessages.ap-southeast-2.amazonaws.com",
	"ca-central-1":   "ssmmessages.ca-central-1.amazonaws.com",
	"cn-north-1":     "ssmmessages.cn-north-1.amazonaws.com.cn",
	"cn-northwest-1": "ssmmessages.cn-northwest-1.amazonaws.com.cn",
	"eu-central-1":   "ssmmessages.eu-central-1.amazonaws.com",
	"eu-west-1":      "ssmmessages.eu-west-1.amazonaws.com",
	"eu-west-2":      "ssmmessages.eu-west-2.amazonaws.com",
	"eu-west-3":      "ssmmessages.eu-west-3.amazonaws.com",
	"sa-east-1":      "ssmmessages.sa-east-1.amazonaws.com",
	"us-east-1":      "ssmmessages.us-east-1.amazonaws.com",
	"us-east-2":      "ssmmessages.us-east-2.amazonaws.com",
	"us-gov-west-1":  "ssmmessages.us-gov-west-1.amazonaws.com",
	"us-west-1":      "ssmmessages.us-west-1.amazonaws.com",
	"us-west-2":      "ssmmessages.us-west-2.amazonaws.com",
	//AUTOGEN_END_MessageGatewayService
}

/* This function returns the mgs endpoint specified by the user in appconfig.
If the user didn't specify one, it will return the Amazon MGS endpoint in a certain region
*/
func GetMgsEndpoint(context context.T, region string) (mgsEndpoint string) {
	appConfig := context.AppConfig()
	if appConfig.Mgs.Endpoint != "" {
		// use net/url package to parse endpoint, if endpoint doesn't contain protocol,
		// fullUrl.Host is empty, should return fullUrl.Path. For backwards compatible, return the non-empty one.
		fullUrl, err := url.Parse(appConfig.Mgs.Endpoint)
		if err == nil {
			if fullUrl.Host != "" {
				return fullUrl.Host
			}

			return fullUrl.Path
		}

	}

	if mgsEndpoint, ok := awsMessageGatewayServiceEndpointMap[region]; ok {
		return mgsEndpoint
	}

	mgsEndpoint = ruEndpoint.GetDefaultEndpoint(context.Log(), MgsServiceName, region, "")

	return mgsEndpoint
}
