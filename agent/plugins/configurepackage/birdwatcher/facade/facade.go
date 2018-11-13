// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

// This package returns the means of creating an object of type facade
package facade

import (
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	retry "github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/facade/retryer"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/amazon-ssm-agent/agent/version"

	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
)

const (
	maxRetries = 3
)

func NewBirdwatcherFacade() BirdwatcherFacade {
	awsConfig := sdkutil.AwsConfig()
	// overriding the retry strategy
	retryer := retry.BirdwatcherRetryer{
		DefaultRetryer: client.DefaultRetryer{
			NumMaxRetries: maxRetries,
		},
	}

	cfg := request.WithRetryer(awsConfig, retryer)

	// overrides ssm client config from appconfig if applicable
	if appCfg, err := appconfig.Config(false); err == nil {
		if appCfg.Ssm.Endpoint != "" {
			cfg.Endpoint = &appCfg.Ssm.Endpoint
		} else {
			if region, err := platform.Region(); err == nil {
				if defaultEndpoint := appconfig.GetDefaultEndPoint(region, "ssm"); defaultEndpoint != "" {
					cfg.Endpoint = &defaultEndpoint
				}
			}
		}
		if appCfg.Agent.Region != "" {
			cfg.Region = &appCfg.Agent.Region
		}
	}
	facadeClientSession := session.New(cfg)

	// Define a request handler with current agentName and version
	SSMAgentVersionUserAgentHandler := request.NamedHandler{
		Name: "ssm.SSMAgentVersionUserAgentHandler",
		Fn:   request.MakeAddToUserAgentHandler(appconfig.DefaultConfig().Agent.Name, version.Version),
	}

	// Add the handler to each request to the BirdwatcherStationService
	facadeClientSession.Handlers.Build.PushBackNamed(SSMAgentVersionUserAgentHandler)

	return ssm.New(facadeClientSession)
}
