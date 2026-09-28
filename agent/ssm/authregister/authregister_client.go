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
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/amazon-ssm-agent/agent/ssm/util"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/iirprovider"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// IClient is an interface to the authenticated registration method of the SSM service.
type IClient interface {
	RegisterManagedInstance(ctx context.Context, publicKey string, publicKeyType string, fingerprint string, iamRole string, tagsJson string, provider string) (string, error)
}

// ISsmSdk defines the functions needed from the AWS SSM SDK
type ISsmSdk interface {
	RegisterManagedInstance(ctx context.Context, input *ssm.RegisterManagedInstanceInput, optFns ...func(*ssm.Options)) (*ssm.RegisterManagedInstanceOutput, error)
}

// Client is a service wrapper that delegates to the ssm sdk.
type Client struct {
	sdk ISsmSdk
}

// RegistrationInfo contains information used to register the instance
type RegistrationInfo struct {
	PrivateKey string
	PublicKey  string
	KeyType    string
	InstanceId string
}

func NewClientWithConfig(log logger.T, appConfig appconfig.SsmagentConfig, imdsClient iirprovider.IEC2MdsSdkClient, awsConfig aws.Config) IClient {
	if imdsClient != nil {
		// Wrap IIRRoleProvider in aws.NewCredentialsCache so an IMDS
		// GetMetadata call isn't issued on every SDK attempt during
		// registration (v2 resolves credentials inside the retry loop).
		// IIRRoleProvider does not implement HandleFailToRefresh, so the
		// cache will surface a failed Retrieve immediately instead of
		// extending stale credentials.
		awsConfig.Credentials = aws.NewCredentialsCache(&iirprovider.IIRRoleProvider{
			Config:     &appConfig,
			Log:        log,
			IMDSClient: imdsClient,
		})
	} else {
		awsConfig.Credentials = credentialproviders.GetRemoteCredsProvider()
	}

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
	ssmService := ssm.NewFromConfig(awsConfig)

	return &Client{sdk: ssmService}
}

// NewClient creates a new SSM client instance
func NewClient(log logger.T, region string, imdsClient iirprovider.IEC2MdsSdkClient) IClient {
	appConfig, appErr := appconfig.Config(true)
	if appErr != nil {
		log.Warnf("encountered error while loading appconfig - %v", appErr)
	}

	awsConfig := util.AwsConfig(log, appConfig, "ssm", region)

	if appErr == nil {
		if appConfig.Ssm.Endpoint != "" {
			ep := sdkutil.NormalizeEndpointScheme(appConfig.Ssm.Endpoint)
			awsConfig.BaseEndpoint = &ep
		}

		if appConfig.Agent.Region != "" {
			awsConfig.Region = appConfig.Agent.Region
		}
	}

	return NewClientWithConfig(log, appConfig, imdsClient, awsConfig)
}

// RegisterManagedInstanceWithContext calls the RegisterManagedInstance SSM API
func (svc *Client) RegisterManagedInstance(ctx context.Context, publicKey, publicKeyType, fingerprint, iamRole, tagsJson, provider string) (string, error) {
	params := ssm.RegisterManagedInstanceInput{
		PublicKey:     aws.String(publicKey),
		PublicKeyType: aws.String(publicKeyType),
		Fingerprint:   aws.String(fingerprint),
	}

	if provider != "" {
		params.Provider = aws.String(provider)
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

		var ssmTags []ssmtypes.Tag
		for _, tag := range tags {
			if tag.Key != "" && tag.Value != "" {
				ssmTags = append(ssmTags, ssmtypes.Tag{
					Key:   aws.String(tag.Key),
					Value: aws.String(tag.Value),
				})
			}
		}

		params.Tags = ssmTags
	}

	var result *ssm.RegisterManagedInstanceOutput
	var err error
	result, err = svc.sdk.RegisterManagedInstance(ctx, &params)

	if err != nil {
		return "", err
	}
	return *result.InstanceId, nil
}
