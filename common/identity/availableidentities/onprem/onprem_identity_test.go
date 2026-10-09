// Copyright 2026 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package onprem

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCredentialsProvider_ReturnsCachedProvider(t *testing.T) {
	mockLog := logmocks.NewMockLog()
	mockProvider := &mocks.IRemoteProvider{}
	mockProvider.On("Retrieve", mock.Anything).Return(aws.Credentials{
		AccessKeyID:     "AKID",
		SecretAccessKey: "SECRET",
		SessionToken:    "TOKEN",
		CanExpire:       true,
		Expires:         time.Now().Add(1 * time.Hour),
	}, nil)

	identity := &Identity{
		Log:                 mockLog,
		Config:              &appconfig.SsmagentConfig{},
		credentialsProvider: mockProvider,
		credentials: aws.Credentials{
			AccessKeyID:     "AKID",
			SecretAccessKey: "SECRET",
		},
	}

	// First call should create the cached provider
	provider1 := identity.CredentialsProvider()
	assert.NotNil(t, provider1)

	// Second call should return the same cached instance
	provider2 := identity.CredentialsProvider()
	assert.NotNil(t, provider2)

	// Both should be the same pointer (cached)
	assert.Same(t, provider1, provider2)
}

func TestCredentialsProvider_CachePreventsRepeatedRetrieve(t *testing.T) {
	mockLog := logmocks.NewMockLog()
	mockProvider := &mocks.IRemoteProvider{}
	mockProvider.On("Retrieve", mock.Anything).Return(aws.Credentials{
		AccessKeyID:     "AKID",
		SecretAccessKey: "SECRET",
		SessionToken:    "TOKEN",
		CanExpire:       true,
		Expires:         time.Now().Add(1 * time.Hour),
	}, nil)

	identity := &Identity{
		Log:                 mockLog,
		Config:              &appconfig.SsmagentConfig{},
		credentialsProvider: mockProvider,
		credentials: aws.Credentials{
			AccessKeyID:     "AKID",
			SecretAccessKey: "SECRET",
		},
	}

	cachedProvider := identity.CredentialsProvider()

	// Simulate multiple SDK calls fetching credentials from the cached provider
	for i := 0; i < 5; i++ {
		creds, err := cachedProvider.Retrieve(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, "AKID", creds.AccessKeyID)
	}

	// The underlying provider's Retrieve should only be called once —
	// subsequent calls are served from cache since credentials haven't expired.
	mockProvider.AssertNumberOfCalls(t, "Retrieve", 1)
}

func TestCredentialsProvider_WrapsExistingProvider(t *testing.T) {
	mockLog := logmocks.NewMockLog()
	mockProvider := &mocks.IRemoteProvider{}
	mockProvider.On("Retrieve", mock.Anything).Return(aws.Credentials{
		AccessKeyID:     "AKID",
		SecretAccessKey: "SECRET",
		CanExpire:       true,
		Expires:         time.Now().Add(1 * time.Hour),
	}, nil)

	identity := &Identity{
		Log:                 mockLog,
		Config:              &appconfig.SsmagentConfig{},
		credentialsProvider: mockProvider,
		credentials: aws.Credentials{
			AccessKeyID:     "AKID",
			SecretAccessKey: "SECRET",
		},
	}

	// Should return a non-nil cached credentials provider
	provider := identity.CredentialsProvider()
	assert.NotNil(t, provider)

	// The underlying credentialsProvider should remain unchanged
	assert.Equal(t, mockProvider, identity.credentialsProvider)
}
