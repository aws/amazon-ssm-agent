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
	"github.com/aws/amazon-ssm-agent/common/identity/endpoint"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/cenkalti/backoff"
)

// AnonymousService is an interface to the Anonymous methods of the SSM service.
type AnonymousService interface {
	RegisterManagedInstance(activationCode, activationID, publicKey, publicKeyType, fingerprint string) (string, error)
}

// sdkService is an service wrapper that delegates to the ssm sdk.
type sdkService struct {
	sdk *ssm.SSM
}

// NewAnonymousService creates a new SSM service instance.
func NewAnonymousService(logger logger.T, region string) AnonymousService {

	log.SetFlags(0)

	awsConfig := util.AwsConfig(logger).WithLogLevel(aws.LogOff)

	awsConfig.Region = &region
	awsConfig.Credentials = credentials.AnonymousCredentials

	//parse appConfig override to get ssm endpoint if there is any
	appConfig, err := appconfig.Config(true)
	if err == nil {
		if appConfig.Ssm.Endpoint != "" {
			awsConfig.Endpoint = &appConfig.Ssm.Endpoint
		} else {
			// Get the default ssm endpoint for this region
			defaultEndpoint := endpoint.GetDefaultEndpoint(logger, "ssm", region, "")
			awsConfig.Endpoint = &defaultEndpoint

		}
	} else {
		log.Printf("encountered error while loading appconfig - %s", err)
	}

	// Create a session to share service client config and handlers with
	ssmSess := session.New(awsConfig)
	ssmSess.Handlers.Build.PushBack(request.MakeAddToUserAgentHandler(appConfig.Agent.Name, appConfig.Agent.Version))

	ssmService := ssm.New(ssmSess)
	return &sdkService{sdk: ssmService}
}

// RegisterManagedInstance calls the RegisterManagedInstance SSM API.
func (svc *sdkService) RegisterManagedInstance(activationCode, activationID, publicKey, publicKeyType, fingerprint string) (string, error) {
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
	var innerErr error

	err = backoff.Retry(func() error {
		result, innerErr = svc.sdk.RegisterManagedInstance(&params)
		return innerErr
	}, exponentialBackoff)

	if err != nil {
		return "", err
	}
	return *result.InstanceId, nil
}
