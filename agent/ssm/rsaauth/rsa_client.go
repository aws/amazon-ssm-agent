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
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/ssm/authtokenrequest"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/iirprovider"
	"github.com/aws/aws-sdk-go/aws/request"
)

// NewRsaClient creates a new SSM client instance that signs requests using a private key
func NewRsaClient(log log.T, appConfig *appconfig.SsmagentConfig, serverId, region, encodedPrivateKey string) authtokenrequest.IClient {
	awsConfig := deps.AwsConfig(log, *appConfig, "ssm", region)

	awsConfig.Credentials = deps.NewStaticCredentials(serverId, encodedPrivateKey, "")

	if appConfig.Ssm.Endpoint != "" {
		awsConfig.Endpoint = &appConfig.Ssm.Endpoint
	}

	// Create a session to share service client config and handlers with
	ssmSess, _ := deps.NewSession(awsConfig)
	ssmSess.Handlers.Build.PushBack(deps.MakeAddToUserAgentHandler(appConfig.Agent.Name, appConfig.Agent.Version))

	ssmSdk := deps.NewSsmSdk(ssmSess)

	// use Beagle's RSA signer override
	// whenever we update sdk, we need to make sure it's using Beagle's RSA signing protocol
	ssmSdk.Handlers.Sign.Clear()
	ssmSdk.Handlers.Sign.PushBack(SignRsa)
	return deps.NewAuthTokenClient(ssmSdk)
}

// NewIirRsaClient creates a new ssm client that signs requests with both instance identity credentials and private key
func NewIirRsaClient(log log.T, appConfig *appconfig.SsmagentConfig, imdsClient iirprovider.IEC2MdsSdkClient, region, encodedPrivateKey string) authtokenrequest.IClient {
	awsConfig := deps.AwsConfig(log, *appConfig, "ssm", region)
	awsConfig.Credentials = deps.NewCredentials(&iirprovider.IIRRoleProvider{
		ExpiryWindow: iirprovider.EarlyExpiryTimeWindow, // Triggers credential refresh, updated on Retrieve()
		Config:       appConfig,
		Log:          log,
		IMDSClient:   imdsClient,
	})

	if appConfig.Ssm.Endpoint != "" {
		awsConfig.Endpoint = &appConfig.Ssm.Endpoint
	}

	// Create a session to share service client config and handlers with
	ssmSess, _ := deps.NewSession(awsConfig)
	ssmSess.Handlers.Build.PushBackNamed(request.NamedHandler{
		Name: "AddUserAgent",
		Fn:   deps.MakeAddToUserAgentHandler(appConfig.Agent.Name, appConfig.Agent.Version),
	})

	ssmSdk := deps.NewSsmSdk(ssmSess)
	ssmSdk.Handlers.Sign.PushBackNamed(request.NamedHandler{
		Name: "SignIirRsa",
		Fn:   MakeSignRsaHandler(encodedPrivateKey),
	})

	return deps.NewAuthTokenClient(ssmSdk)
}
