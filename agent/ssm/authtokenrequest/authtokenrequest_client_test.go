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

package authtokenrequest

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/ssm/authtokenrequest/mocks"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSSMAuthTokenService_RequestManagedInstanceRoleToken_Success(t *testing.T) {
	sdk := &mocks.ISsmSdk{}
	response := &ssm.RequestManagedInstanceRoleTokenOutput{}
	sdk.On("RequestManagedInstanceRoleToken", mock.Anything).Return(response, nil)
	authTokenService := NewClient(sdk)
	result, err := authTokenService.RequestManagedInstanceRoleToken("SomeFingerprint")
	assert.NoError(t, err)
	assert.Equal(t, response, result)
}

func TestSSMAuthTokenService_UpdateManagedInstancePublicKey_Success(t *testing.T) {
	sdk := &mocks.ISsmSdk{}
	response := &ssm.UpdateManagedInstancePublicKeyOutput{}
	sdk.On("UpdateManagedInstancePublicKey", mock.Anything).Return(response, nil)
	authTokenService := NewClient(sdk)
	result, err := authTokenService.UpdateManagedInstancePublicKey("publicKey", "publicKeyType")
	assert.NoError(t, err)
	assert.Equal(t, response, result)
}
