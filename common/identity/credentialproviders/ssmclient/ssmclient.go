// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License").
// You may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ssmclient

import (
	"context"
	"fmt"

	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/amazon-ssm-agent/agent/ssm/util"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

var (
	loadAppConfig             = appconfig.Config
	utilAWSConfig             = util.AwsConfig
	utilAWSConfigWithEndpoint = util.AwsConfigWithEndpoint
)

// ISSMClient defines the functions needed for role providers send health pings to Systems Manager
type ISSMClient interface {
	UpdateInstanceInformation(ctx context.Context, input *ssm.UpdateInstanceInformationInput, opts ...func(*ssm.Options)) (*ssm.UpdateInstanceInformationOutput, error)
}

// Initializer is a function that initializes and returns an ISSMClient
type Initializer func(log log.T, credentialsProvider aws.CredentialsProvider, region, defaultEndpoint string) ISSMClient

// NewV4ServiceWithCreds creates a ssm.SSM that is configured to sign requests to the SSM API with the given credentials
func NewV4ServiceWithCreds(log log.T, credentialsProvider aws.CredentialsProvider, region, defaultEndpoint string) ISSMClient {
	// read latest from AppConfig file
	appConfig, err := loadAppConfig(true)
	if err != nil {
		log.Warnf("Error while loading app config. Err: %v", err)
	}

	var endpoint *string
	if appConfig.Ssm.Endpoint != "" {
		// Normalize the scheme so scheme-less appconfig overrides (e.g. VPC
		// endpoint hostnames) satisfy SDK v2's URI validation, which requires
		// a scheme on BaseEndpoint values.
		normalized := sdkutil.NormalizeEndpointScheme(appConfig.Ssm.Endpoint)
		endpoint = &normalized
	} else if defaultEndpoint != "" {
		endpoint = &defaultEndpoint
	}

	awsConfig := utilAWSConfigWithEndpoint(log, appConfig, "ssm", region, endpoint)
	awsConfig.Region = region
	awsConfig.Credentials = credentialsProvider
	awsConfig.APIOptions = append(awsConfig.APIOptions, func(stack *middleware.Stack) error {
		return stack.Build.Add(
			middleware.BuildMiddlewareFunc("AddUserAgent", func(ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler) (middleware.BuildOutput, middleware.Metadata, error) {
				req := in.Request.(*smithyhttp.Request)
				userAgent := fmt.Sprintf("%s/%s", appConfig.Agent.Name, appConfig.Agent.Version)
				req.Header.Add("User-Agent", userAgent)
				return next.HandleBuild(ctx, in)
			}),
			middleware.After,
		)
	})

	return ssm.NewFromConfig(awsConfig)
}
