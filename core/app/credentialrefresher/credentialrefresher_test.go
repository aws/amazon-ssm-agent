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

package credentialrefresher

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity"
	mocks3 "github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/mocks"
	"github.com/aws/amazon-ssm-agent/common/runtimeconfig"
	mocks2 "github.com/aws/amazon-ssm-agent/common/runtimeconfig/mocks"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var (
	fiveMinBeforeTime = time.Date(2021, time.January, 1, 12, 10, 30, 0, time.UTC).Round(0)
	currentTime       = time.Date(2021, time.January, 1, 12, 15, 30, 0, time.UTC).Round(0)
	fiveMinAfterTime  = time.Date(2021, time.January, 1, 12, 20, 30, 0, time.UTC).Round(0)
	tenMinAfterTime   = time.Date(2021, time.January, 1, 12, 25, 30, 0, time.UTC).Round(0)
)

func init() {
	newSharedCredentials = func(_, _ string) *credentials.Credentials {
		provider := &mocks3.Provider{}
		provider.On("Retrieve").Return(credentials.Value{}, nil).Once()
		return credentials.NewCredentials(provider)
	}
}

func Test_credentialsRefresher_durationUntilRefresh(t *testing.T) {
	type fields struct {
		log                         log.T
		agentIdentity               identity.IAgentIdentity
		provider                    credentials.Provider
		expirer                     credentials.Expirer
		runtimeConfigClient         runtimeconfig.IIdentityRuntimeConfigClient
		identityRuntimeConfig       runtimeconfig.IdentityRuntimeConfig
		backoffConfig               *backoff.ExponentialBackOff
		credsReadyOnce              sync.Once
		credentialsReadyChan        chan struct{}
		stopCredentialRefresherChan chan struct{}
		getCurrentTimeFunc          func() time.Time
	}
	tests := []struct {
		name   string
		fields fields
		want   time.Duration
	}{
		{
			"TestCredentialsAlreadyExpired",
			fields{
				identityRuntimeConfig: runtimeconfig.IdentityRuntimeConfig{
					CredentialsExpiresAt: fiveMinBeforeTime,
				},
				getCurrentTimeFunc: func() time.Time {
					return currentTime
				},
			},
			time.Duration(0),
		},
		{
			"TestCredentialsExpireCurrentTime",
			fields{
				identityRuntimeConfig: runtimeconfig.IdentityRuntimeConfig{
					CredentialsExpiresAt: currentTime,
				},
				getCurrentTimeFunc: func() time.Time {
					return currentTime
				},
			},
			time.Duration(0),
		},
		{
			"TestCredentialsExpireInFiveMinutes_CredentialsLifetimeTenMinutes",
			fields{
				identityRuntimeConfig: runtimeconfig.IdentityRuntimeConfig{
					CredentialsExpiresAt:   fiveMinAfterTime,
					CredentialsRetrievedAt: fiveMinBeforeTime,
				},
				getCurrentTimeFunc: func() time.Time {
					return currentTime
				},
			},
			time.Duration(0),
		},
		{
			"TestCredentialsExpireInFiveMinutes_CredentialsLifetimeFifteenMinutes",
			fields{
				identityRuntimeConfig: runtimeconfig.IdentityRuntimeConfig{
					CredentialsExpiresAt:   tenMinAfterTime,
					CredentialsRetrievedAt: fiveMinBeforeTime,
				},
				getCurrentTimeFunc: func() time.Time {
					return currentTime
				},
			},
			time.Minute*2 + time.Second*30,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &credentialsRefresher{
				log:                         tt.fields.log,
				agentIdentity:               tt.fields.agentIdentity,
				provider:                    tt.fields.provider,
				expirer:                     tt.fields.expirer,
				runtimeConfigClient:         tt.fields.runtimeConfigClient,
				identityRuntimeConfig:       tt.fields.identityRuntimeConfig,
				backoffConfig:               tt.fields.backoffConfig,
				credsReadyOnce:              tt.fields.credsReadyOnce,
				credentialsReadyChan:        tt.fields.credentialsReadyChan,
				stopCredentialRefresherChan: tt.fields.stopCredentialRefresherChan,
				getCurrentTimeFunc:          tt.fields.getCurrentTimeFunc,
			}
			if got := c.durationUntilRefresh(); got != tt.want {
				t.Errorf("durationUntilRefresh() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_credentialsRefresher_credentialRefresherRoutine_CredentialsNotExpired_NotCallRefresh(t *testing.T) {
	storeSharedCredentials = func(log.T, string, string, string, string, string, bool) error {
		return nil
	}

	runtimeConfig := runtimeconfig.IdentityRuntimeConfig{
		CredentialsExpiresAt:   tenMinAfterTime,
		CredentialsRetrievedAt: currentTime,
	}

	c := &credentialsRefresher{
		log:                          log.NewMockLog(),
		identityRuntimeConfig:        runtimeConfig,
		credsReadyOnce:               sync.Once{},
		credentialsReadyChan:         make(chan struct{}, 1),
		stopCredentialRefresherChan:  make(chan struct{}),
		isCredentialRefresherRunning: true,
		getCurrentTimeFunc:           func() time.Time { return currentTime },
	}

	go c.credentialRefresherRoutine()

	// verify credentials ready message is sent
	select {
	case <-c.credentialsReadyChan:
	case <-time.After(time.Second):
		assert.Fail(t, "CredentialsReadyChan never got a message")
	}

	// Stop goroutine and verify it stops within a second
	select {
	case c.stopCredentialRefresherChan <- struct{}{}:
	case <-time.After(time.Second):
		assert.Fail(t, "Took more than a second to stop credential refresher")
	}
}

func Test_credentialsRefresher_credentialRefresherRoutine_CredentialsNotExpired_CredentialsFileFailure(t *testing.T) {
	storeSharedCredentials = func(log.T, string, string, string, string, string, bool) error {
		return nil
	}

	oldFun := newSharedCredentials
	defer func() { newSharedCredentials = oldFun }()
	newSharedCredentials = func(_, _ string) *credentials.Credentials {
		provider := &mocks3.Provider{}
		provider.On("Retrieve").Return(credentials.Value{}, fmt.Errorf("SomeShareCredsErr")).Once()
		return credentials.NewCredentials(provider)
	}

	runtimeConfig := runtimeconfig.IdentityRuntimeConfig{
		CredentialsExpiresAt:   tenMinAfterTime,
		CredentialsRetrievedAt: fiveMinBeforeTime,
	}

	provider := &mocks3.Provider{}
	provider.On("Retrieve").Return(func() credentials.Value { return credentials.Value{} }, func() error {
		// Sleep here because we know that if we reach this point and have not got message in credentialsReadyChan, the time is set correctly
		time.Sleep(time.Second)
		return fmt.Errorf("SomeRetrieveErr")
	})

	expirer := &mocks3.Expirer{}
	expirer.On("ExpiresAt").Return(tenMinAfterTime)

	c := &credentialsRefresher{
		log:                          log.NewMockLog(),
		provider:                     provider,
		expirer:                      expirer,
		identityRuntimeConfig:        runtimeConfig,
		credsReadyOnce:               sync.Once{},
		credentialsReadyChan:         make(chan struct{}, 1),
		stopCredentialRefresherChan:  make(chan struct{}),
		isCredentialRefresherRunning: true,
		getCurrentTimeFunc:           func() time.Time { return currentTime },
	}

	go c.credentialRefresherRoutine()

	// verify credentials ready message is sent
	select {
	case <-c.credentialsReadyChan:
		assert.Fail(t, "CredentialsReadyChan got a message when credentials were not available")
	case <-time.After(time.Second):
	}

	// Stop goroutine and verify it stops within a second
	select {
	case c.stopCredentialRefresherChan <- struct{}{}:
	case <-time.After(time.Second):
		assert.Fail(t, "Took more than a second to stop credential refresher")
	}

	assert.True(t, c.identityRuntimeConfig.CredentialsExpiresAt.Equal(time.Time{}))
	assert.True(t, c.identityRuntimeConfig.CredentialsRetrievedAt.Equal(time.Time{}))
}

func Test_credentialsRefresher_credentialRefresherRoutine_CredentialsExist_CallStopMultipleTimes(t *testing.T) {
	storeSharedCredentials = func(log.T, string, string, string, string, string, bool) error {
		return nil
	}

	runtimeConfig := runtimeconfig.IdentityRuntimeConfig{
		CredentialsExpiresAt:   tenMinAfterTime,
		CredentialsRetrievedAt: currentTime,
	}

	c := &credentialsRefresher{
		log:                          log.NewMockLog(),
		identityRuntimeConfig:        runtimeConfig,
		credsReadyOnce:               sync.Once{},
		credentialsReadyChan:         make(chan struct{}, 1),
		stopCredentialRefresherChan:  make(chan struct{}),
		isCredentialRefresherRunning: true,
		getCurrentTimeFunc:           func() time.Time { return currentTime },
	}

	go c.credentialRefresherRoutine()

	// verify credentials ready message is sent
	select {
	case <-c.credentialsReadyChan:
	case <-time.After(time.Second):
		assert.Fail(t, "CredentialsReadyChan never got a message")
	}

	// Stop goroutine
	c.Stop()
	assert.False(t, c.isCredentialRefresherRunning)

	// verify stop does not block
	c.Stop()
}

func Test_credentialsRefresher_credentialRefresherRoutine_CredentialsDontExist(t *testing.T) {
	storeSharedCredentials = func(_ log.T, _ string, _ string, _ string, _ string, _ string, force bool) error {
		if !force {
			return fmt.Errorf("someErrorMustForce")
		}

		return nil
	}

	// Return error once and success second time
	backoffRetry = func(o backoff.Operation, _ backoff.BackOff) error {
		return o()
	}

	// Should rotate right away
	runtimeConfig := runtimeconfig.IdentityRuntimeConfig{
		CredentialsExpiresAt:   fiveMinAfterTime,
		CredentialsRetrievedAt: fiveMinBeforeTime,
	}

	runtimeConfigClient := &mocks2.IIdentityRuntimeConfigClient{}
	runtimeConfigClient.On("SaveConfig", mock.Anything).Return(nil).Once()

	provider := &mocks3.Provider{}
	provider.On("Retrieve").Return(credentials.Value{}, nil).Once()

	expirer := &mocks3.Expirer{}
	expirer.On("ExpiresAt").Return(tenMinAfterTime).Once()

	c := &credentialsRefresher{
		log:                          log.NewMockLog(),
		provider:                     provider,
		expirer:                      expirer,
		runtimeConfigClient:          runtimeConfigClient,
		identityRuntimeConfig:        runtimeConfig,
		credsReadyOnce:               sync.Once{},
		credentialsReadyChan:         make(chan struct{}, 1),
		stopCredentialRefresherChan:  make(chan struct{}),
		isCredentialRefresherRunning: true,
		getCurrentTimeFunc:           func() time.Time { return currentTime },
	}

	go c.credentialRefresherRoutine()

	// verify credentials ready message is sent because there are still 5 minutes left of credential
	select {
	case <-c.credentialsReadyChan:
	case <-time.After(time.Second):
		assert.Fail(t, "CredentialsReadyChan never got a message")
	}

	// Give goroutine 1 second to go through retrieval
	time.Sleep(time.Second)

	// Stop goroutine
	c.Stop()
	assert.False(t, c.isCredentialRefresherRunning)

	runtimeConfigClient.AssertExpectations(t)
	provider.AssertExpectations(t)
	expirer.AssertExpectations(t)

	c.identityRuntimeConfig.CredentialsRetrievedAt.Equal(currentTime)
	c.identityRuntimeConfig.CredentialsExpiresAt.Equal(tenMinAfterTime)

}
