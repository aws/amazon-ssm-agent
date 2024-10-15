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
	"log"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/backoffconfig"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/ssm/util"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"

	"github.com/cenkalti/backoff/v4"
)

var backoffRetry = backoff.Retry

// IClient is an interface to the Anonymous methods of the SSM service.
type IClient interface {
	RegisterManagedInstance(activationCode, activationID, publicKey, publicKeyType, fingerprint string) (string, error)
}

// ISsmSdk defines the functions needed from the AWS SSM SDK
type ISsmSdk interface {
	RegisterManagedInstance(input *ssm.RegisterManagedInstanceInput) (*ssm.RegisterManagedInstanceOutput, error)
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

	if awsErr, ok := err.(awserr.Error); ok {
		if awsErr.Code() == ssm.ErrCodeTooManyUpdates {
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

	appConfig, appErr := appconfig.Config(true)
	if appErr != nil {
		log.Printf("encountered error while loading appconfig - %v", appErr)
	}
	awsConfig := util.AwsConfig(logger, appConfig, "ssm", region).WithLogLevel(aws.LogOff)
	awsConfig.Credentials = credentials.AnonymousCredentials

	if appErr == nil && appConfig.Ssm.Endpoint != "" {
		awsConfig.Endpoint = &appConfig.Ssm.Endpoint
	}

	// Create a session to share service client config and handlers with
	ssmSess := session.New(awsConfig)
	ssmSess.Handlers.Build.PushBack(request.MakeAddToUserAgentHandler(appConfig.Agent.Name, appConfig.Agent.Version))

	ssmService := ssm.New(ssmSess)
	return &Client{sdk: ssmService}
}

// RegisterManagedInstance calls the RegisterManagedInstance SSM API.
func (svc *Client) RegisterManagedInstance(activationCode, activationID, publicKey, publicKeyType, fingerprint string) (string, error) {
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

	var result *ssm.RegisterManagedInstanceOutput
	_ = backoffRetry(func() error {
		result, err = svc.sdk.RegisterManagedInstance(&params)
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
