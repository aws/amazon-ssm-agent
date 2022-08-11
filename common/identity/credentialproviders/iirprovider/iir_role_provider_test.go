// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License").
// You may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package iirprovider

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	iirprovidermocks "github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/iirprovider/mocks"
	"github.com/aws/aws-sdk-go/aws/credentials"

	"github.com/stretchr/testify/assert"
)

const (
	testAccessKeyId     = "SomeAccessKeyId"
	testSecretAccessKey = "SomeSecretAccessKey"
	testSessionToken    = "SomeSessionToken"
)

func TestRetrieve_ReturnsCredentials(t *testing.T) {
	logger := log.NewMockLog()
	ssmConfig, _ := appconfig.Config(true)

	respCreds := Ec2RoleCreds{
		AccessKeyID:     testAccessKeyId,
		SecretAccessKey: testSecretAccessKey,
		Token:           testSessionToken,
		Expiration:      time.Now().Add(time.Hour * 6),
		Code:            "Success",
	}

	expectedResult := credentials.Value{
		AccessKeyID:     respCreds.AccessKeyID,
		SecretAccessKey: respCreds.SecretAccessKey,
		SessionToken:    respCreds.Token,
		ProviderName:    ProviderName,
	}

	respJSONBytes, _ := json.Marshal(respCreds)

	mockIMDSClient := &iirprovidermocks.IEC2MdsSdkClient{}
	mockIMDSClient.On("GetMetadata", iirCredentialsPath).Return(string(respJSONBytes), nil)

	roleProvider := &IIRRoleProvider{
		IMDSClient:   mockIMDSClient,
		ExpiryWindow: EarlyExpiryTimeWindow,
		Config:       &ssmConfig,
		Log:          logger,
	}

	result, err := roleProvider.Retrieve()

	assert.NotNil(t, result)
	assert.Nil(t, err)
	assert.Equal(t, expectedResult, result)
}
