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
	cont "context"
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/facade/retryer"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

const (
	maxRetries = 3
)

func NewBirdwatcherFacade(context context.T) BirdwatcherFacade {
	awsConfig := sdkutil.AwsConfig(context, "ssm")
	awsConfig.Retryer = func() aws.Retryer {
		return &retryer.BirdwatcherRetryer{
			Standard: retry.NewStandard(func(so *retry.StandardOptions) {
				so.MaxAttempts = maxRetries + 1
			}),
		}
	}

	// Override endpoint if configured
	appCfg := context.AppConfig()
	if appCfg.Ssm.Endpoint != "" {
		awsConfig.BaseEndpoint = aws.String(sdkutil.NormalizeEndpointScheme(appCfg.Ssm.Endpoint))
	}

	if appCfg.Agent.Region != "" {
		awsConfig.Region = appCfg.Agent.Region
	}

	// Create SSM client with user agent handler in APIOptions
	return ssm.NewFromConfig(awsConfig, func(o *ssm.Options) {
		o.APIOptions = append(o.APIOptions, func(stack *middleware.Stack) error {
			return stack.Build.Add(
				middleware.BuildMiddlewareFunc("SSMAgentVersionUserAgent",
					func(ctx cont.Context, in middleware.BuildInput, next middleware.BuildHandler) (middleware.BuildOutput, middleware.Metadata, error) {
						req := in.Request.(*smithyhttp.Request)
						userAgent := fmt.Sprintf("%s/%s", context.AppConfig().Agent.Name, version.Version)
						req.Header.Add("User-Agent", userAgent)
						return next.HandleBuild(ctx, in)
					}),
				middleware.After,
			)
		})
	})
}
