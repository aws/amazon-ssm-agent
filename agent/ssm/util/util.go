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
	"fmt"
	"net/http"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/network"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil/retryer"
	"github.com/aws/amazon-ssm-agent/common/identity/endpoint"
	"github.com/aws/aws-sdk-go/aws"
)

func AwsConfig(logger log.T, appConfig appconfig.SsmagentConfig, service, region string) *aws.Config {
	endpointHelper := endpoint.NewEndpointHelper(logger, appConfig)

	return &aws.Config{
		Retryer:    newRetryer(),
		SleepDelay: sleepDelay,
		HTTPClient: &http.Client{
			Transport:     network.GetDefaultTransport(logger, appConfig),
			CheckRedirect: disableHTTPDowngrade,
		},
		Region:   aws.String(region),
		Endpoint: aws.String(endpointHelper.GetServiceEndpoint(service, region)),
		Logger:   logger,
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

func disableHTTPDowngrade(req *http.Request, via []*http.Request) error {
	//Go's http.DefaultClient allows 10 redirects before returning an error.
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}

	//Send an error on HTTP redirect attempt
	if len(via) > 0 && via[0].URL.Scheme == "https" && req.URL.Scheme != "https" {
		lastHop := via[len(via)-1].URL
		return fmt.Errorf("redirected from secure URL %s to insecure URL %s", lastHop, req.URL)
	}
	return nil
}
