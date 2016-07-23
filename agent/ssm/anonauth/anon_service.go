// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the AWS Customer Agreement (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/agreement/

// Package anonauth is an interface to the anonymous methods of the SSM service.
package anonauth

import (
	"log"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/ssm/util"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
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
func NewAnonymousService(region string) AnonymousService {
	log.SetFlags(0)
	awsConfig := util.AwsConfig().WithLogLevel(aws.LogOff)

	awsConfig.Region = &region
	awsConfig.Credentials = credentials.AnonymousCredentials

	//parse appConfig override to get ssm endpoint if there is any
	appConfig, err := appconfig.Config(true)
	if err == nil {
		if appConfig.Ssm.Endpoint != "" {
			awsConfig.Endpoint = &appConfig.Ssm.Endpoint
		}
	} else {
		log.Printf("encountered error while loading appconfig - %s", err)
	}

	// Create a session to share service client config and handlers with
	ssmSess := session.New(awsConfig)

	ssmService := ssm.New(ssmSess)
	return &sdkService{sdk: ssmService}
}

// RegisterManagedInstance calls the RegisterManagedInstance SSM API.
func (svc *sdkService) RegisterManagedInstance(activationCode, activationID, publicKey, publicKeyType, fingerprint string) (string, error) {

	params := ssm.RegisterManagedInstanceInput{
		ActivationCode: aws.String(activationCode),
		ActivationId:   aws.String(activationID),
		PublicKey:      aws.String(publicKey),
		PublicKeyType:  aws.String(publicKeyType),
		Fingerprint:    aws.String(fingerprint),
	}

	result, err := svc.sdk.RegisterManagedInstance(&params)
	if err != nil {
		return "", err
	}
	return *result.InstanceId, nil
}
