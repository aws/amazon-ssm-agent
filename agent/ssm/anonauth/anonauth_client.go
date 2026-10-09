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

// Package anonauth is an interface to the anonymous methods of the SSM service.
package anonauth

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	logger "github.com/aws/amazon-ssm-agent/agent/log"

	"github.com/aws/amazon-ssm-agent/agent/backoffconfig"
	"github.com/aws/amazon-ssm-agent/agent/network"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/amazon-ssm-agent/common/identity/endpoint"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/smithy-go"

	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/cenkalti/backoff/v4"
)

var backoffRetry = backoff.Retry

// IClient is an interface to the Anonymous methods of the SSM service.
type IClient interface {
	RegisterManagedInstance(activationCode string, activationID string, publicKey string, publicKeyType string, fingerprint string, provider string) (string, error)
}

// ISsmSdk defines the functions needed from the AWS SSM SDK
type ISsmSdk interface {
	RegisterManagedInstance(ctx context.Context, input *ssm.RegisterManagedInstanceInput, optFns ...func(*ssm.Options)) (*ssm.RegisterManagedInstanceOutput, error)
}

// Client is a service wrapper that delegates to the ssm sdk
type Client struct {
	sdk ISsmSdk
}

// shouldRetryAwsRequest determines if request should be retried
func shouldRetryAwsRequest(err error) bool {
	// Don't retry if no error
	if err == nil {
		return false
	}

	// In v2, check for specific error types
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		if apiErr.ErrorCode() == "TooManyUpdates" {
			return true
		}
		return false
	}

	// Retry for any non-aws errors
	return true
}

// NewClient creates a new SSM client instance
func NewClient(logger logger.T, region string) IClient {

	log.SetFlags(0)

	var endpointString string
	appConfig, appErr := appconfig.Config(true)
	if appErr != nil {
		log.Printf("encountered error while loading appconfig - %v", appErr)
	}
	endpointHelper := endpoint.NewEndpointHelper(logger, appConfig)
	if appErr == nil && appConfig.Ssm.Endpoint != "" {
		endpointString = sdkutil.NormalizeEndpointScheme(appConfig.Ssm.Endpoint)
	} else {
		endpointString = endpointHelper.GetServiceEndpoint("ssm", region)
	}
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithCredentialsProvider(aws.AnonymousCredentials{}),
		config.WithBaseEndpoint(endpointString),
		config.WithHTTPClient(&http.Client{
			Transport:     network.GetDefaultTransport(logger, appConfig),
			CheckRedirect: network.DisableHTTPDowngrade,
			Timeout:       60 * time.Second,
		}),
		// Disable SDK's env-based dualstack resolution. The agent manages dualstack
		// endpoints itself via appConfig.Agent.UseDualStackEndpoint. Allowing the SDK
		// to also read AWS_USE_DUALSTACK_ENDPOINT from the environment conflicts with
		// BaseEndpoint and causes "Invalid Configuration: Dualstack and custom endpoint
		// are not supported".
		config.WithUseDualStackEndpoint(aws.DualStackEndpointStateDisabled),
		// Same for FIPS — agent manages endpoints itself, AWS_USE_FIPS_ENDPOINT from
		// env would conflict with BaseEndpoint.
		config.WithUseFIPSEndpoint(aws.FIPSEndpointStateDisabled),
	)
	if err != nil {
		log.Printf("failed to load AWS config: %v", err)
		return nil
	}

	cfg.APIOptions = append(cfg.APIOptions, awsmiddleware.AddUserAgentKeyValue(appConfig.Agent.Name, appConfig.Agent.Version))

	ssmService := ssm.NewFromConfig(cfg)
	return &Client{sdk: ssmService}
}

// RegisterManagedInstance calls the RegisterManagedInstance SSM API.
func (svc *Client) RegisterManagedInstance(activationCode, activationID, publicKey, publicKeyType, fingerprint, provider string) (string, error) {
	exponentialBackoff, err := backoffconfig.GetDefaultExponentialBackoff()
	if err != nil {
		return "", err
	}

	params := ssm.RegisterManagedInstanceInput{
		ActivationCode: aws.String(activationCode),
		ActivationId:   aws.String(activationID),
		PublicKey:      aws.String(publicKey),
		PublicKeyType:  aws.String(publicKeyType),
		Fingerprint:    aws.String(fingerprint),
	}

	if provider != "" {
		params.Provider = aws.String(provider)
	}

	var result *ssm.RegisterManagedInstanceOutput
	_ = backoffRetry(func() error {
		result, err = svc.sdk.RegisterManagedInstance(context.TODO(), &params)
		if shouldRetryAwsRequest(err) {
			return err
		}
		return nil
	}, exponentialBackoff)

	if err != nil {
		return "", err
	}
	return *result.InstanceId, nil
}
