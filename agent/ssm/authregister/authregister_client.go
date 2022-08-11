// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package authregister is an interface to the anonymous methods of the SSM service.
package authregister

import (
	"encoding/json"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/ssm/util"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/iirprovider"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// IClient is an interface to the authenticated registration method of the SSM service.
type IClient interface {
	RegisterManagedInstance(publicKey, publicKeyType, fingerprint, iamRole, tagsJson string) (string, error)
}

// ISsmSdk defines the functions needed from the AWS SSM SDK
type ISsmSdk interface {
	RegisterManagedInstance(input *ssm.RegisterManagedInstanceInput) (*ssm.RegisterManagedInstanceOutput, error)
}

// Client is an service wrapper that delegates to the ssm sdk.
type Client struct {
	sdk ISsmSdk
}

// RegistrationInfo contains information used to register the instance
type RegistrationInfo struct {
	PrivateKey string
	KeyType    string
}

// NewClient creates a new SSM client instance
func NewClient(log logger.T, region string, imdsClient iirprovider.IEC2MdsSdkClient) IClient {
	appConfig, appErr := appconfig.Config(true)
	if appErr != nil {
		log.Warnf("encountered error while loading appconfig - %v", appErr)
	}

	awsConfig := util.AwsConfig(log, appConfig, "ssm", region).WithLogLevel(aws.LogOff)

	if appErr == nil {
		if appConfig.Ssm.Endpoint != "" {
			awsConfig.Endpoint = &appConfig.Ssm.Endpoint
		}

		if appConfig.Agent.Region != "" {
			awsConfig.Region = &appConfig.Agent.Region
		}
	}
	if imdsClient != nil {
		awsConfig.Credentials = credentials.NewCredentials(&iirprovider.IIRRoleProvider{
			ExpiryWindow: iirprovider.EarlyExpiryTimeWindow,
			Config:       &appConfig,
			Log:          log,
			IMDSClient:   imdsClient,
		})
	} else {
		awsConfig.Credentials = credentialproviders.GetRemoteCreds()
	}

	sess := session.New(awsConfig)
	sess.Handlers.Build.PushBack(request.MakeAddToUserAgentHandler(appConfig.Agent.Name, appConfig.Agent.Version))
	ssmService := ssm.New(sess)

	return &Client{sdk: ssmService}
}

// RegisterManagedInstance calls the RegisterManagedInstance SSM API
func (svc *Client) RegisterManagedInstance(publicKey, publicKeyType, fingerprint, iamRole, tagsJson string) (string, error) {
	params := ssm.RegisterManagedInstanceInput{
		PublicKey:     aws.String(publicKey),
		PublicKeyType: aws.String(publicKeyType),
		Fingerprint:   aws.String(fingerprint),
	}

	if iamRole != "" {
		params.IamRole = aws.String(iamRole)
	}

	if tagsJson != "" {
		tags := []struct {
			Key, Value string
		}{}
		err := json.Unmarshal([]byte(tagsJson), &tags)

		if err != nil {
			return "", err
		}

		var ssmTags []*ssm.Tag
		for _, tag := range tags {
			if tag.Key != "" && tag.Value != "" {
				ssmTags = append(ssmTags, &ssm.Tag{
					Key:   aws.String(tag.Key),
					Value: aws.String(tag.Value),
				})
			}
		}

		params.Tags = ssmTags
	}

	var result *ssm.RegisterManagedInstanceOutput
	var err error
	result, err = svc.sdk.RegisterManagedInstance(&params)

	if err != nil {
		return "", err
	}
	return *result.InstanceId, nil
}
