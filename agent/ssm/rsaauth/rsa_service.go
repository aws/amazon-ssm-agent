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

// Package rsaauth is an interface to the RSA signed methods of the SSM service.
package rsaauth

import (
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/ssm/util"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	v4 "github.com/aws/aws-sdk-go/private/signer/v4"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// RsaSignedService is an interface to the RSA signed methods of the SSM service.
type RsaSignedService interface {
	RequestManagedInstanceRoleToken(fingerprint string) (response *ssm.RequestManagedInstanceRoleTokenOutput, err error)
	UpdateManagedInstancePublicKey(publicKey, publicKeyType string) (response *ssm.UpdateManagedInstancePublicKeyOutput, err error)
}

// sdkService is an service wrapper that delegates to the ssm sdk.
type sdkService struct {
	sdk *ssm.SSM
}

// NewRsaService creates a new SSM service instance.
func NewRsaService(serverId string, region string, encodedPrivateKey string) RsaSignedService {
	awsConfig := util.AwsConfig()

	awsConfig.Region = &region
	awsConfig.Credentials = credentials.NewStaticCredentials(serverId, encodedPrivateKey, "")

	appConfig, _ := appconfig.Config(false)
	// Create a session to share service client config and handlers with
	ssmSess, _ := session.NewSession(awsConfig)
	ssmSess.Handlers.Build.PushBack(request.MakeAddToUserAgentHandler(appConfig.Agent.Name, appConfig.Agent.Version))

	ssmService := ssm.New(ssmSess)

	// use Beagle's RSA signer override
	// whenever we update sdk, we need to make sure it's using Beagle's RSA signing protocol
	ssmService.Handlers.Sign.Clear()
	ssmService.Handlers.Sign.PushBack(v4.SignRsa)
	return &sdkService{sdk: ssmService}
}

// RequestManagedInstanceRoleToken calls the RequestManagedInstanceRoleToken SSM API.
func (svc *sdkService) RequestManagedInstanceRoleToken(fingerprint string) (response *ssm.RequestManagedInstanceRoleTokenOutput, err error) {

	params := ssm.RequestManagedInstanceRoleTokenInput{
		Fingerprint: aws.String(fingerprint),
	}

	return svc.sdk.RequestManagedInstanceRoleToken(&params)
}

// UpdateManagedInstancePublicKey calls the UpdateManagedInstancePublicKey SSM API.
func (svc *sdkService) UpdateManagedInstancePublicKey(publicKey, publicKeyType string) (response *ssm.UpdateManagedInstancePublicKeyOutput, err error) {

	params := ssm.UpdateManagedInstancePublicKeyInput{
		NewPublicKey:     aws.String(publicKey),
		NewPublicKeyType: aws.String(publicKeyType),
	}

	return svc.sdk.UpdateManagedInstancePublicKey(&params)
}
