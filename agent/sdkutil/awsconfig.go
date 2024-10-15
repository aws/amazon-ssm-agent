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

// Package sdkutil provides utilities used to call awssdk.
package sdkutil

import (
	"net/http"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/network"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil/retryer"
	"github.com/aws/amazon-ssm-agent/common/identity/endpoint"
	"github.com/aws/aws-sdk-go/aws"
)

// AwsConfig returns the default aws.Config object with the appropriate
// credentials.
func AwsConfig(context context.T, service string) (awsConfig *aws.Config) {
	region, _ := context.Identity().Region()
	endpoint := context.Identity().GetServiceEndpoint(service)
	return AwsConfigForEndpoint(context, endpoint, region)
}

// AwsConfigForRegion returns the default aws.Config object with the appropriate
// credentials and endpoint.
func AwsConfigForRegion(context context.T, service, region string) (awsConfig *aws.Config) {
	endpointHelper := endpoint.NewEndpointHelper(context.Log(), context.AppConfig())
	return AwsConfigForEndpoint(context, endpointHelper.GetServiceEndpoint(service, region), region)
}

// AwsConfigForEndpoint returns the default aws.Config object with the appropriate
// credentials and endpoint.
func AwsConfigForEndpoint(context context.T, endpoint, region string) (awsConfig *aws.Config) {
	// create default config
	return &aws.Config{
		Retryer:    newRetryer(),
		SleepDelay: sleepDelay,
		Region:     aws.String(region),
		Endpoint:   aws.String(endpoint),
		HTTPClient: &http.Client{
			Transport: network.GetDefaultTransport(context.Log(), context.AppConfig()),
		},
		Credentials: context.Identity().Credentials(),
	}
}

var newRetryer = func() aws.RequestRetryer {
	r := retryer.SsmRetryer{}
	r.NumMaxRetries = 3
	return r
}

var sleepDelay = func(d time.Duration) {
	time.Sleep(d)
}
