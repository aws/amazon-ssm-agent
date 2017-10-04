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

// Package util contains helper function common for ssm service
package util

import (
	"crypto/tls"
	"net/http"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil/retryer"
	"github.com/aws/aws-sdk-go/aws"
)

func AwsConfig() *aws.Config {
	// create default config
	awsConfig := &aws.Config{
		Retryer:    newRetryer(),
		SleepDelay: sleepDelay,
	}

	// parse appConfig overrides
	appConfig, err := appconfig.Config(false)
	if err != nil {
		return awsConfig
	}
	if appConfig.Ssm.Endpoint != "" {
		awsConfig.Endpoint = &appConfig.Ssm.Endpoint
	} else {
		if region, err := platform.Region(); err == nil {
			if defaultEndpoint := appconfig.GetDefaultEndPoint(region, "ssm"); defaultEndpoint != "" {
				awsConfig.Endpoint = &defaultEndpoint
			}
		}
	}
	if appConfig.Agent.Region != "" {
		awsConfig.Region = &appConfig.Agent.Region
	}
	// TODO: test hook, can be removed before release
	// this is to skip ssl verification for the beta self signed certs
	if appConfig.Ssm.InsecureSkipVerify {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		awsConfig.HTTPClient = &http.Client{Transport: tr}
	}

	return awsConfig

}

var newRetryer = func() aws.RequestRetryer {
	r := retryer.SsmRetryer{}
	r.NumMaxRetries = 3
	return r
}

var sleepDelay = func(d time.Duration) {
	time.Sleep(d)
}
