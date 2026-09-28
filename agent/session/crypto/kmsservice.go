// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// crypto package provides methods to encrypt and decrypt data
package crypto

import (
	cont "context"
	"fmt"

	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
)

// KMSKeySizeInBytes is the key size that is fetched from KMS. 64 bytes key is split into two halves.
// First half 32 bytes key is used by agent for encryption and second half 32 bytes by clients like cli/console
const KMSKeySizeInBytes int64 = 64

type IKMSService interface {
	Decrypt(cipherTextBlob []byte, encryptionContext map[string]string, keyId string) (plainText []byte, err error)
}

type KMSService struct {
	client *kms.Client
}

// NewKMSService creates a new KMSService instance
func NewKMSService(context context.T) (kmsService *KMSService, err error) {
	var (
		awsConfig    aws.Config
		appConfig    appconfig.SsmagentConfig
		agentName    string
		agentVersion string
	)

	awsConfig = sdkutil.AwsConfig(context, "kms")

	appConfig = context.AppConfig()
	if appConfig.Kms.Endpoint != "" {
		ep := sdkutil.NormalizeEndpointScheme(appConfig.Kms.Endpoint)
		awsConfig.BaseEndpoint = &ep
	}
	agentName = appConfig.Agent.Name
	agentVersion = appConfig.Agent.Version

	// Add UserAgent middleware BEFORE creating the client
	awsConfig.APIOptions = append(awsConfig.APIOptions, func(stack *middleware.Stack) error {
		return stack.Build.Add(
			middleware.BuildMiddlewareFunc("AddUserAgent", func(ctx cont.Context, in middleware.BuildInput, next middleware.BuildHandler) (middleware.BuildOutput, middleware.Metadata, error) {
				req := in.Request.(*smithyhttp.Request)
				userAgent := fmt.Sprintf("%s/%s", agentName, agentVersion)
				req.Header.Add("User-Agent", userAgent)
				return next.HandleBuild(ctx, in)
			}),
			middleware.After,
		)
	})

	kmsService = &KMSService{
		client: kms.NewFromConfig(awsConfig),
	}

	return kmsService, nil
}

// Decrypt will get the plaintext key from KMS service
func (kmsService *KMSService) Decrypt(cipherTextBlob []byte, encryptionContext map[string]string, keyId string) (plainText []byte, err error) {
	output, err := kmsService.client.Decrypt(cont.TODO(), &kms.DecryptInput{
		CiphertextBlob:    cipherTextBlob,
		EncryptionContext: encryptionContext,
		KeyId:             &keyId,
	})
	if err != nil {
		return nil, fmt.Errorf("Error when decrypting data key %s", err)
	}
	return output.Plaintext, nil
}
