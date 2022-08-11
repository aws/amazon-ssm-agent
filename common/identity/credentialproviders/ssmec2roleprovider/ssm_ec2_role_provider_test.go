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

package ssmec2roleprovider

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/ssm/authregister"
	"github.com/aws/amazon-ssm-agent/agent/ssm/authtokenrequest"
	authtokenrequestmocks "github.com/aws/amazon-ssm-agent/agent/ssm/authtokenrequest/mocks"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/iirprovider"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSSMEC2RoleProvider_IsEC2InstanceRegistered_ReturnsFalse_WhenNoRegistrationInfoInChannel(t *testing.T) {
	// Arrange
	roleProvider := &SSMEC2RoleProvider{
		Log:                   log.NewMockLog(),
		RegistrationReadyChan: make(chan *authregister.RegistrationInfo, 1),
	}

	assert.False(t, roleProvider.isEC2InstanceRegistered())
}

func TestSSMEC2RoleProvider_IsEC2InstanceRegistered_ReturnsTrue_WhenRegistrationInfoExists(t *testing.T) {
	// Arrange
	registrationInfo := &authregister.RegistrationInfo{
		PrivateKey: "SomePrivateKey",
		KeyType:    "SomeKeyType",
	}
	roleProvider := &SSMEC2RoleProvider{
		Log:                   log.NewMockLog(),
		RegistrationReadyChan: make(chan *authregister.RegistrationInfo, 1),
	}

	roleProvider.RegistrationReadyChan <- registrationInfo

	assert.True(t, roleProvider.isEC2InstanceRegistered())
	assert.True(t, roleProvider.isEC2InstanceRegistered())
}

func TestSSMEC2RoleProvider_Retrieve_ReturnsCredentials(t *testing.T) {
	// Arrange
	registrationInfo := &authregister.RegistrationInfo{
		PrivateKey: "SomePrivateKey",
		KeyType:    "SomeKeyType",
	}

	tokenExpiration := time.Now().Add(time.Hour)
	roleCreds := &ssm.RequestManagedInstanceRoleTokenOutput{
		AccessKeyId:         aws.String("SomeAccessKeyId"),
		SecretAccessKey:     aws.String("SomeSecretAccessKey"),
		SessionToken:        aws.String("SomeSessionToken"),
		TokenExpirationDate: &tokenExpiration,
	}

	roleProvider := &SSMEC2RoleProvider{
		Log:                   log.NewMockLog(),
		RegistrationReadyChan: make(chan *authregister.RegistrationInfo, 1),
		InstanceInfo:          &InstanceInfo{Region: "SomeRegion"},
	}

	roleProvider.RegistrationReadyChan <- registrationInfo

	tokenRequestService := &authtokenrequestmocks.IClient{}
	tokenRequestService.On("RequestManagedInstanceRoleToken", mock.Anything).Return(roleCreds, nil)
	newIirRsaAuth = func(log log.T, appConfig *appconfig.SsmagentConfig, imdsClient iirprovider.IEC2MdsSdkClient, region, encodedPrivateKey string) authtokenrequest.IClient {
		return tokenRequestService
	}

	// Act
	creds, err := roleProvider.Retrieve()

	//Assert
	assert.NoError(t, err)
	assert.Equal(t, ProviderName, creds.ProviderName)
	assert.Equal(t, *roleCreds.AccessKeyId, creds.AccessKeyID)
	assert.Equal(t, *roleCreds.SecretAccessKey, creds.SecretAccessKey)
	assert.Equal(t, *roleCreds.SessionToken, creds.SessionToken)
	assert.Equal(t, time.Duration(0), roleProvider.ExpiryWindow)
	assert.False(t, roleProvider.IsExpired())
}

func TestSSMEC2RoleProvider_Retrieve_ReturnsEmptyCredentials_WhenInstanceNotRegistered(t *testing.T) {
	// Arrange
	roleProvider := &SSMEC2RoleProvider{
		Log:                   log.NewMockLog(),
		RegistrationReadyChan: make(chan *authregister.RegistrationInfo, 1),
	}

	creds, err := roleProvider.Retrieve()
	assert.Error(t, err)
	assert.Equal(t, EmptyCredentials(), creds)
}

func TestSSMEC2RoleProvider_Retrieve_ReturnsEmptyCredentials_NoRetry(t *testing.T) {
	// Arrange
	statusCode := 400
	statusMessage := "Unauthorized"
	unauthorizedErr := awserr.New(fmt.Sprint(statusCode), statusMessage, nil)
	unauthorizedRequestFailure := awserr.NewRequestFailure(unauthorizedErr, statusCode, "testRequestId")

	registrationInfo := &authregister.RegistrationInfo{
		PrivateKey: "SomePrivateKey",
		KeyType:    "SomeKeyType",
	}

	roleProvider := &SSMEC2RoleProvider{
		Log:                   log.NewMockLog(),
		RegistrationReadyChan: make(chan *authregister.RegistrationInfo, 1),
		InstanceInfo:          &InstanceInfo{Region: "SomeRegion"},
	}

	roleProvider.RegistrationReadyChan <- registrationInfo

	tokenRequestService := &authtokenrequestmocks.IClient{}

	tokenRequestService.On("RequestManagedInstanceRoleToken", mock.Anything).Return(nil, unauthorizedRequestFailure).Repeatability = 1
	newIirRsaAuth = func(log log.T, appConfig *appconfig.SsmagentConfig, imdsClient iirprovider.IEC2MdsSdkClient, region, encodedPrivateKey string) authtokenrequest.IClient {
		return tokenRequestService
	}

	creds, err := roleProvider.Retrieve()
	assert.Error(t, err)
	assert.Equal(t, EmptyCredentials(), creds)
}

func TestSSMEC2RoleProvider_Retrieve_ReturnsEmptyCredentials_Retries(t *testing.T) {
	// Arrange
	statusCode := 500
	statusMessage := "InternalServerError"
	unauthorizedErr := awserr.New(fmt.Sprint(statusCode), statusMessage, nil)
	unauthorizedRequestFailure := awserr.NewRequestFailure(unauthorizedErr, statusCode, "testRequestId")

	registrationInfo := &authregister.RegistrationInfo{
		PrivateKey: "SomePrivateKey",
		KeyType:    "SomeKeyType",
	}

	roleProvider := &SSMEC2RoleProvider{
		Log:                   log.NewMockLog(),
		RegistrationReadyChan: make(chan *authregister.RegistrationInfo, 1),
		InstanceInfo:          &InstanceInfo{Region: "SomeRegion"},
	}

	roleProvider.RegistrationReadyChan <- registrationInfo

	tokenRequestService := &authtokenrequestmocks.IClient{}

	tokenRequestService.On("RequestManagedInstanceRoleToken", mock.Anything).Return(nil, unauthorizedRequestFailure)
	newIirRsaAuth = func(log log.T, appConfig *appconfig.SsmagentConfig, imdsClient iirprovider.IEC2MdsSdkClient, region, encodedPrivateKey string) authtokenrequest.IClient {
		return tokenRequestService
	}

	creds, err := roleProvider.Retrieve()
	assert.Error(t, err)
	assert.Equal(t, EmptyCredentials(), creds)
}
