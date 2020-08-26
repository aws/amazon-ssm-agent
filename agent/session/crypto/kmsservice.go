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
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/aws/aws-sdk-go/service/kms/kmsiface"
)

// KMSKeySizeInBytes is the key size that is fetched from KMS. 64 bytes key is split into two halves.
// First half 32 bytes key is used by agent for encryption and second half 32 bytes by clients like cli/console
const KMSKeySizeInBytes int64 = 64

type IKMSService interface {
	Decrypt(cipherTextBlob []byte, encryptionContext map[string]*string) (plainText []byte, err error)
}

type KMSService struct {
	client kmsiface.KMSAPI
}

// NewKMSService creates a new KMSService instance
func NewKMSService(log log.T) (kmsService *KMSService, err error) {
	var (
		awsConfig        *aws.Config
		appConfig        appconfig.SsmagentConfig
		kmsClientSession *session.Session
		agentName        string
		agentVersion     string
	)
	awsConfig = sdkutil.AwsConfig()
	if appConfig, err = appconfig.Config(false); err != nil {
		log.Warnf("Failed to load appconfig: %s. Using default config.", err)
	} else if appConfig.Kms.Endpoint != "" {
		awsConfig.Endpoint = &appConfig.Kms.Endpoint
	}
	agentName = appConfig.Agent.Name
	agentVersion = appConfig.Agent.Version
	if kmsClientSession, err = session.NewSession(awsConfig); err != nil {
		return nil, fmt.Errorf("Error creating new aws sdk session: %s", err)
	}
	kmsClientSession.Handlers.Build.PushBack(request.MakeAddToUserAgentHandler(agentName, agentVersion))
	kmsService = &KMSService{
		client: kms.New(kmsClientSession),
	}

	return kmsService, nil
}

// Decrypt will get the plaintext key from KMS service
func (kmsService *KMSService) Decrypt(cipherTextBlob []byte, encryptionContext map[string]*string) (plainText []byte, err error) {
	output, err := kmsService.client.Decrypt(&kms.DecryptInput{
		CiphertextBlob:    cipherTextBlob,
		EncryptionContext: encryptionContext})
	if err != nil {
		return nil, fmt.Errorf("Error when decrypting data key %s", err)
	}
	return output.Plaintext, nil
}
