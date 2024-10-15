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

package anonauth

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/ssm/anonauth/mocks"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSdkService_RegisterManagedInstance_Success(t *testing.T) {
	// Arrange
	activationCode := "SomeActivationCode"
	activationId := "SomeActivationId"
	publicKey := "SomePublicKey"
	publicKeyType := "SomePublicKeyType"
	fingerprint := "SomeFingerprint"
	anonServiceSdk := &mocks.ISsmSdk{}
	output := &ssm.RegisterManagedInstanceOutput{
		InstanceId: aws.String("SomeInstanceId"),
	}
	anonServiceSdk.On("RegisterManagedInstance", mock.Anything).Return(output, nil)
	anonService := &Client{
		sdk: anonServiceSdk,
	}

	// Act
	res, err := anonService.RegisterManagedInstance(activationCode, activationId, publicKey, publicKeyType, fingerprint)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, res, *output.InstanceId)
}

func TestSdkService_RegisterManagedInstance_Retries(t *testing.T) {
	testCases := []struct {
		testName       string
		retryableError error
	}{
		{
			testName:       "TestSdkService_RegisterManagedInstance_Retries_WhenTooManyUpdates",
			retryableError: awserr.New(ssm.ErrCodeTooManyUpdates, "too many activation updates", nil),
		},
		{
			testName:       "TestSdkService_RegisterManagedInstance_Retries_WhenNonAwsError",
			retryableError: fmt.Errorf("failed to make call to RegisterManagedInstance API"),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.testName, func(t *testing.T) {
			// Arrange
			activationCode := "SomeActivationCode"
			activationId := "SomeActivationId"
			publicKey := "SomePublicKey"
			publicKeyType := "SomePublicKeyType"
			fingerprint := "SomeFingerprint"
			anonServiceSdk := &mocks.ISsmSdk{}
			anonServiceSdk.On("RegisterManagedInstance", mock.Anything).Return(nil, testCase.retryableError)
			backoffRetry = func(o backoff.Operation, b backoff.BackOff) error {
				err := o()
				// Error triggers retries
				assert.Error(t, err)
				return err
			}

			anonService := &Client{
				sdk: anonServiceSdk,
			}

			// Act
			res, err := anonService.RegisterManagedInstance(activationCode, activationId, publicKey, publicKeyType, fingerprint)

			// Assert
			assert.Error(t, err)
			assert.Equal(t, "", res)
		})
	}

}
