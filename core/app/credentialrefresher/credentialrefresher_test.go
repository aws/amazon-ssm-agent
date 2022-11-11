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
	identityMock "github.com/aws/amazon-ssm-agent/common/identity/mocks"
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
	mockAgentIdentity = &identityMock.IAgentIdentity{}
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
				log: log.NewMockLog(),
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
				log: log.NewMockLog(),
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
				log: log.NewMockLog(),
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
				log: log.NewMockLog(),
			},
			time.Minute*2 + time.Second*30,
		},
	}
	for _, tt := range tests {
		provider := &mocks3.IRemoteProvider{}
		provider.On("Retrieve").Return(credentials.Value{}, nil).Repeatability = 0
		t.Run(tt.name, func(t *testing.T) {
			c := &credentialsRefresher{
				log:                         tt.fields.log,
				agentIdentity:               mockAgentIdentity,
				provider:                    provider,
				expirer:                     tt.fields.expirer,
				runtimeConfigClient:         tt.fields.runtimeConfigClient,
				identityRuntimeConfig:       tt.fields.identityRuntimeConfig,
				backoffConfig:               tt.fields.backoffConfig,
				credsReadyOnce:              tt.fields.credsReadyOnce,
				credentialsReadyChan:        tt.fields.credentialsReadyChan,
				stopCredentialRefresherChan: tt.fields.stopCredentialRefresherChan,
				getCurrentTimeFunc:          tt.fields.getCurrentTimeFunc,
				timeAfterFunc:               time.After,
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

	provider := &mocks3.IRemoteProvider{}
	provider.On("Retrieve").Return(credentials.Value{}, nil).Repeatability = 0

	c := &credentialsRefresher{
		log:                          log.NewMockLog(),
		agentIdentity:                mockAgentIdentity,
		provider:                     provider,
		identityRuntimeConfig:        runtimeConfig,
		credsReadyOnce:               sync.Once{},
		credentialsReadyChan:         make(chan struct{}, 1),
		stopCredentialRefresherChan:  make(chan struct{}),
		isCredentialRefresherRunning: true,
		getCurrentTimeFunc:           func() time.Time { return currentTime },
		timeAfterFunc:                time.After,
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

	provider := &mocks3.IRemoteProvider{}
	provider.On("Retrieve").Return(func() credentials.Value { return credentials.Value{} }, func() error {
		// Sleep here because we know that if we reach this point and have not got message in credentialsReadyChan, the time is set correctly
		time.Sleep(time.Second)
		return fmt.Errorf("SomeRetrieveErr")
	})

	expirer := &mocks3.Expirer{}
	expirer.On("ExpiresAt").Return(tenMinAfterTime)

	c := &credentialsRefresher{
		log:                          log.NewMockLog(),
		agentIdentity:                mockAgentIdentity,
		provider:                     provider,
		expirer:                      expirer,
		identityRuntimeConfig:        runtimeConfig,
		credsReadyOnce:               sync.Once{},
		credentialsReadyChan:         make(chan struct{}, 1),
		stopCredentialRefresherChan:  make(chan struct{}),
		isCredentialRefresherRunning: true,
		getCurrentTimeFunc:           func() time.Time { return currentTime },
		timeAfterFunc:                time.After,
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
	assert.True(t, c.identityRuntimeConfig.CredentialsRetrievedAt.Equal(fiveMinBeforeTime))
}

func Test_credentialsRefresher_credentialRefresherRoutine_CredentialsExist_CallStopMultipleTimes(t *testing.T) {
	storeSharedCredentials = func(log.T, string, string, string, string, string, bool) error {
		return nil
	}

	runtimeConfig := runtimeconfig.IdentityRuntimeConfig{
		CredentialsExpiresAt:   tenMinAfterTime,
		CredentialsRetrievedAt: currentTime,
	}

	provider := &mocks3.IRemoteProvider{}
	provider.On("Retrieve").Return(credentials.Value{}, nil).Repeatability = 0

	c := &credentialsRefresher{
		log:                          log.NewMockLog(),
		agentIdentity:                mockAgentIdentity,
		provider:                     provider,
		identityRuntimeConfig:        runtimeConfig,
		credsReadyOnce:               sync.Once{},
		credentialsReadyChan:         make(chan struct{}, 1),
		stopCredentialRefresherChan:  make(chan struct{}),
		isCredentialRefresherRunning: true,
		getCurrentTimeFunc:           func() time.Time { return currentTime },
		timeAfterFunc:                time.After,
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

	provider := &mocks3.IRemoteProvider{}
	provider.On("Retrieve").Return(credentials.Value{}, nil).Once()

	expirer := &mocks3.Expirer{}
	expirer.On("ExpiresAt").Return(tenMinAfterTime).Once()

	c := &credentialsRefresher{
		log:                          log.NewMockLog(),
		agentIdentity:                mockAgentIdentity,
		provider:                     provider,
		expirer:                      expirer,
		runtimeConfigClient:          runtimeConfigClient,
		identityRuntimeConfig:        runtimeConfig,
		credsReadyOnce:               sync.Once{},
		credentialsReadyChan:         make(chan struct{}, 1),
		stopCredentialRefresherChan:  make(chan struct{}),
		isCredentialRefresherRunning: true,
		getCurrentTimeFunc:           func() time.Time { return currentTime },
		timeAfterFunc:                time.After,
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

// Mock aws error struct
type awsTestError struct {
	errCode string
}

func (a awsTestError) Error() string   { return "" }
func (a awsTestError) Message() string { return "" }
func (a awsTestError) OrigErr() error  { return fmt.Errorf("SomeErr") }
func (a awsTestError) Code() string    { return a.errCode }

func Test_credentialsRefresher_retrieveCredsWithRetry_NonActionableErr(t *testing.T) {
	for _, awsErr := range []error{awsTestError{"AccessDeniedException"}, awsTestError{"MachineFingerprintDoesNotMatch"}} {
		provider := &mocks3.IRemoteProvider{}
		provider.On("Retrieve").Return(credentials.Value{}, awsErr).Once()

		var timeAfterParamVal time.Duration
		c := &credentialsRefresher{
			log:                         log.NewMockLog(),
			agentIdentity:               mockAgentIdentity,
			provider:                    provider,
			stopCredentialRefresherChan: make(chan struct{}),
			timeAfterFunc: func(duration time.Duration) <-chan time.Time {
				timeAfterParamVal = duration
				c := make(chan time.Time)
				return c
			},
		}

		waitGrp := sync.WaitGroup{}
		waitGrp.Add(1)
		stopped := false
		go func() {
			defer waitGrp.Done()
			_, stopped = c.retrieveCredsWithRetry()
		}()

		// Allow retrieve to finish one round
		time.Sleep(time.Millisecond * 100)

		// Verify sleep was at least 24 hours
		assert.True(t, timeAfterParamVal >= time.Hour*24)
		provider.AssertExpectations(t)

		// stop
		c.stopCredentialRefresherChan <- struct{}{}

		waitGrp.Wait()
		assert.True(t, stopped, "expected retrieve to have been stopped by channel message")
	}
}

func Test_credentialsRefresher_retrieveCredsWithRetry_Retry2000TimesNoExitUntilSuccess(t *testing.T) {
	provider := &mocks3.IRemoteProvider{}
	provider.On("Retrieve").Return(credentials.Value{}, awsTestError{"PotentiallyRecoverableAWSError"}).Times(1000)
	provider.On("Retrieve").Return(credentials.Value{}, fmt.Errorf("SomeRandomNonAwsErr")).Times(1000)
	provider.On("Retrieve").Return(credentials.Value{}, nil).Once()

	numSleeps := 0
	c := &credentialsRefresher{
		log:                         log.NewMockLog(),
		agentIdentity:               mockAgentIdentity,
		provider:                    provider,
		stopCredentialRefresherChan: make(chan struct{}),
		timeAfterFunc: func(duration time.Duration) <-chan time.Time {
			numSleeps++
			// assumes random aws error first 3 retries which would never produce a retry below 6 seconds
			assert.True(t, duration > time.Second*5, "AWS Error produced retry below 6 seconds")

			// Retry for errors that are not invalid instance id nor machine fingerprint should never produce sleep longer than 22 hours
			assert.True(t, duration < time.Hour*22, "sleep for longer than 22 hours")
			c := make(chan time.Time, 1)
			c <- time.Now()
			return c
		},
	}

	_, stopped := c.retrieveCredsWithRetry()
	provider.AssertExpectations(t)
	assert.Equal(t, 2000, numSleeps, "Number of retries was not correct")
	assert.False(t, stopped, "expected retrieve to not have been stopped by channel message")
}
