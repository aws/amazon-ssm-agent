// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package rsaauth is an interface to the RSA signed methods of the SSM service.
package rsaauth

import (
	cont "context"
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/amazon-ssm-agent/agent/ssm/authtokenrequest"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/iirprovider"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// NewRsaClient creates a new SSM client instance that signs requests using a private key
func NewRsaClient(log log.T, appConfig *appconfig.SsmagentConfig, serverId, region, encodedPrivateKey string) authtokenrequest.IClient {
	awsConfig := deps.AwsConfig(log, *appConfig, "ssm", region)

	awsConfig.Credentials = aws.NewCredentialsCache(deps.NewStaticCredentialsProvider(serverId, encodedPrivateKey, ""))

	if appConfig.Ssm.Endpoint != "" {
		ep := sdkutil.NormalizeEndpointScheme(appConfig.Ssm.Endpoint)
		awsConfig.BaseEndpoint = &ep
	}

	awsConfig.APIOptions = append(awsConfig.APIOptions, func(stack *middleware.Stack) error {
		return stack.Build.Add(
			middleware.BuildMiddlewareFunc("AddUserAgent", func(ctx cont.Context, in middleware.BuildInput, next middleware.BuildHandler) (middleware.BuildOutput, middleware.Metadata, error) {
				req := in.Request.(*smithyhttp.Request)
				userAgent := fmt.Sprintf("%s/%s", appConfig.Agent.Name, appConfig.Agent.Version)
				req.Header.Add("User-Agent", userAgent)
				return next.HandleBuild(ctx, in)
			}),
			middleware.After,
		)
	})

	awsConfig.APIOptions = append(awsConfig.APIOptions, func(stack *middleware.Stack) error {
		// Remove original signing and add our RSA signer
		_, _ = stack.Finalize.Remove("Signing")
		return stack.Finalize.Add(
			middleware.FinalizeMiddlewareFunc("RSASigner", func(ctx cont.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
				smithyReq := in.Request.(*smithyhttp.Request)

				if err := SignRsa(smithyReq, awsConfig, "ssm"); err != nil {
					return middleware.FinalizeOutput{}, middleware.Metadata{}, err
				}

				return next.HandleFinalize(ctx, in)
			}),
			middleware.After,
		)
	})

	ssmSdk := deps.NewSsmSdk(awsConfig)

	// use Beagle's RSA signer override
	// whenever we update sdk, we need to make sure it's using Beagle's RSA signing protocol
	return deps.NewAuthTokenClient(ssmSdk)
}

// NewIirRsaClient creates a new ssm client that signs requests with both instance identity credentials and private key
func NewIirRsaClient(log log.T, appConfig *appconfig.SsmagentConfig, imdsClient iirprovider.IEC2MdsSdkClient, region, encodedPrivateKey string) authtokenrequest.IClient {
	awsConfig := deps.AwsConfig(log, *appConfig, "ssm", region)
	awsConfig.Credentials = aws.NewCredentialsCache(&iirprovider.IIRRoleProvider{
		Config:     appConfig,
		Log:        log,
		IMDSClient: imdsClient,
	})

	if appConfig.Ssm.Endpoint != "" {
		ep := sdkutil.NormalizeEndpointScheme(appConfig.Ssm.Endpoint)
		awsConfig.BaseEndpoint = &ep
	}

	awsConfig.APIOptions = append(awsConfig.APIOptions, func(stack *middleware.Stack) error {
		return stack.Build.Add(
			middleware.BuildMiddlewareFunc("AddUserAgent", func(ctx cont.Context, in middleware.BuildInput, next middleware.BuildHandler) (middleware.BuildOutput, middleware.Metadata, error) {
				req := in.Request.(*smithyhttp.Request)
				userAgent := fmt.Sprintf("%s/%s", appConfig.Agent.Name, appConfig.Agent.Version)
				req.Header.Add("User-Agent", userAgent)
				return next.HandleBuild(ctx, in)
			}),
			middleware.After,
		)
	})

	awsConfig.APIOptions = append(awsConfig.APIOptions, func(stack *middleware.Stack) error {
		return stack.Finalize.Add(
			middleware.FinalizeMiddlewareFunc("SignIirRsa", func(ctx cont.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
				// Apply RSA signing logic here
				smithyReq := in.Request.(*smithyhttp.Request)
				if err := MakeSignRsaHandler(encodedPrivateKey)(smithyReq); err != nil {
					return middleware.FinalizeOutput{}, middleware.Metadata{}, err
				}

				return next.HandleFinalize(ctx, in)
			}),
			middleware.After,
		)
	})

	ssmSdk := deps.NewSsmSdk(awsConfig)

	return deps.NewAuthTokenClient(ssmSdk)
}
