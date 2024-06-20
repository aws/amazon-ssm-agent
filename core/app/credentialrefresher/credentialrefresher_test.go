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
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/sharedCredentials"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/onprem"
	credentialmocks "github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/mocks"
	identityMock "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/aws/amazon-ssm-agent/common/runtimeconfig"
	runtimeconfigmocks "github.com/aws/amazon-ssm-agent/common/runtimeconfig/mocks"
	"github.com/aws/amazon-ssm-agent/core/executor"
	"github.com/aws/amazon-ssm-agent/core/executor/mocks"
	"github.com/aws/aws-sdk-go/aws/awserr"
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
		provider := &credentialmocks.Provider{}
		provider.On("RemoteRetrieve", mock.Anything).Return(credentials.Value{}, nil).Once()
		provider.On("RemoteExpiresAt").Return(time.Now().Add(1 * time.Hour)).Once()
		provider.On("ShareFile").Return("", nil).Times(2)
		provider.On("CredentialSource").Return("SSM").Times(3)
		return credentials.NewCredentials(provider)
	}
}

func Test_credentialsRefresher_durationUntilRefresh(t *testing.T) {
	type fields struct {
		log                   log.T
		runtimeConfigClient   runtimeconfig.IIdentityRuntimeConfigClient
		identityRuntimeConfig runtimeconfig.IdentityRuntimeConfig
		getCurrentTimeFunc    func() time.Time
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
				log: logmocks.NewMockLog(),
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
				log: logmocks.NewMockLog(),
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
				log: logmocks.NewMockLog(),
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
				log: logmocks.NewMockLog(),
			},
			time.Minute*2 + time.Second*30,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &credentialsRefresher{
				log:                   tt.fields.log,
				runtimeConfigClient:   tt.fields.runtimeConfigClient,
				identityRuntimeConfig: tt.fields.identityRuntimeConfig,
				getCurrentTimeFunc:    tt.fields.getCurrentTimeFunc,
				appConfig:             &appconfig.SsmagentConfig{Agent: appconfig.AgentInfo{}},
			}
			if got := c.durationUntilRefresh(); got != tt.want {
				t.Errorf("durationUntilRefresh() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_credentialsRefresher_credentialRefresherRoutine_CredentialsNotExpired_SharedCreds_NotCallRefresh(t *testing.T) {
	storeSharedCredentials = func(log.T, string, string, string, string, string, bool) error {
		return nil
	}

	oldFun := newSharedCredentials
	defer func() { newSharedCredentials = oldFun }()
	newSharedCredentials = func(_, _ string) *credentials.Credentials {
		provider := &credentialmocks.Provider{}
		provider.On("Retrieve").Return(credentials.Value{}, nil).Once()
		return credentials.NewCredentials(provider)
	}

	runtimeConfig := runtimeconfig.IdentityRuntimeConfig{
		CredentialsExpiresAt:   tenMinAfterTime,
		CredentialsRetrievedAt: currentTime,
	}

	provider := &credentialmocks.IRemoteProvider{}
	provider.On("ShareFile").Return("SomeSharedCredentialsFile").Once()
	c := &credentialsRefresher{
		log:                          logmocks.NewMockLog(),
		agentIdentity:                &identityMock.IAgentIdentity{},
		provider:                     provider,
		identityRuntimeConfig:        runtimeConfig,
		credsReadyOnce:               sync.Once{},
		credentialsReadyChan:         make(chan struct{}, 1),
		stopCredentialRefresherChan:  make(chan struct{}),
		isCredentialRefresherRunning: true,
		getCurrentTimeFunc:           func() time.Time { return currentTime },
		timeAfterFunc:                time.After,
		appConfig:                    &appconfig.SsmagentConfig{Agent: appconfig.AgentInfo{}},
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

func Test_credentialsRefresher_credentialRefresherRoutine_CredentialsNotExpired_NoSharedCreds_NotCallRefresh(t *testing.T) {
	storeSharedCredentials = func(log.T, string, string, string, string, string, bool) error {
		return nil
	}

	oldFun := newSharedCredentials
	defer func() { newSharedCredentials = oldFun }()
	newSharedCredentials = func(_, _ string) *credentials.Credentials {
		provider := &credentialmocks.Provider{}
		provider.On("Retrieve").Return(credentials.Value{}, nil).Once()
		return credentials.NewCredentials(provider)
	}

	runtimeConfig := runtimeconfig.IdentityRuntimeConfig{
		CredentialsExpiresAt:   tenMinAfterTime,
		CredentialsRetrievedAt: currentTime,
	}

	provider := &credentialmocks.IRemoteProvider{}
	provider.On("ShareFile").Return("").Once()
	c := &credentialsRefresher{
		log:                          logmocks.NewMockLog(),
		provider:                     provider,
		identityRuntimeConfig:        runtimeConfig,
		credentialsReadyChan:         make(chan struct{}, 1),
		stopCredentialRefresherChan:  make(chan struct{}),
		isCredentialRefresherRunning: true,
		getCurrentTimeFunc:           func() time.Time { return currentTime },
		timeAfterFunc:                time.After,
		appConfig:                    &appconfig.SsmagentConfig{Agent: appconfig.AgentInfo{}},
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
		provider := &credentialmocks.Provider{}
		provider.On("Retrieve").Return(credentials.Value{}, fmt.Errorf("SomeShareCredsErr")).Once()
		return credentials.NewCredentials(provider)
	}

	runtimeConfig := runtimeconfig.IdentityRuntimeConfig{
		CredentialsExpiresAt:   tenMinAfterTime,
		CredentialsRetrievedAt: fiveMinBeforeTime,
	}

	provider := &credentialmocks.IRemoteProvider{}
	provider.On("RemoteRetrieve", mock.Anything).Return(func(context.Context) credentials.Value { return credentials.Value{} }, func(context.Context) error {
		// Sleep here because we know that if we reach this point and have not got message in credentialsReadyChan, the time is set correctly
		time.Sleep(time.Second)
		return fmt.Errorf("SomeRetrieveErr")
	})
	provider.On("ShareFile").Return("SomeShareFile").Repeatability = 0
	provider.On("CredentialSource").Return("SSM").Repeatability = 0
	provider.On("RemoteExpiresAt").Return(tenMinAfterTime).Once()
	mockAgentIdentity := &identityMock.IAgentIdentity{}
	mockAgentIdentity.On("IdentityType").Return(onprem.IdentityType)

	c := &credentialsRefresher{
		log:                          logmocks.NewMockLog(),
		agentIdentity:                mockAgentIdentity,
		provider:                     provider,
		identityRuntimeConfig:        runtimeConfig,
		credsReadyOnce:               sync.Once{},
		credentialsReadyChan:         make(chan struct{}, 1),
		stopCredentialRefresherChan:  make(chan struct{}),
		isCredentialRefresherRunning: true,
		getCurrentTimeFunc:           func() time.Time { return currentTime },
		timeAfterFunc:                time.After,
		appConfig:                    &appconfig.SsmagentConfig{Agent: appconfig.AgentInfo{}},
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

	assert.True(t, c.identityRuntimeConfig.CredentialsExpiresAt.Equal(time.Time{}), fmt.Sprintf("CredentialExpiresAt is %v but should be %v", c.identityRuntimeConfig.CredentialsExpiresAt, time.Time{}))
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

	provider := &credentialmocks.IRemoteProvider{}
	provider.On("Retrieve").Return(credentials.Value{}, nil).Repeatability = 0
	provider.On("RemoteRetrieve", mock.Anything).Return(credentials.Value{}, nil).Repeatability = 0
	provider.On("RemoteExpiresAt").Return(time.Now().Add(1 * time.Hour)).Repeatability = 0
	provider.On("ShareFile").Return("SomeShareFile", nil).Repeatability = 0
	provider.On("CredentialSource").Return("SSM").Repeatability = 0
	mockAgentIdentity := &identityMock.IAgentIdentity{}
	mockAgentIdentity.On("IdentityType").Return(onprem.IdentityType)
	newSharedCredentials = func(filename, profile string) *credentials.Credentials {
		return credentials.NewCredentials(provider)
	}

	c := &credentialsRefresher{
		log:                          logmocks.NewMockLog(),
		agentIdentity:                mockAgentIdentity,
		provider:                     provider,
		identityRuntimeConfig:        runtimeConfig,
		credsReadyOnce:               sync.Once{},
		credentialsReadyChan:         make(chan struct{}, 1),
		stopCredentialRefresherChan:  make(chan struct{}),
		isCredentialRefresherRunning: true,
		getCurrentTimeFunc:           func() time.Time { return currentTime },
		timeAfterFunc:                time.After,
		appConfig:                    &appconfig.SsmagentConfig{Agent: appconfig.AgentInfo{}},
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

func Test_credentialsRefresher_credentialRefresherRoutine_Purge(t *testing.T) {
	defaultShareLocation, _ := sharedCredentials.GetSharedCredsFilePath("")
	testCases := []struct {
		testName             string
		oldShareFileLocation string
		newShareFileLocation string
		shouldPurge          bool
		purgeConfigVal       bool
	}{
		{
			testName:             "DoesNotPurgeWhenOldShareFileEmpty",
			oldShareFileLocation: "",
			newShareFileLocation: "SomeShareFile",
			shouldPurge:          false,
		},
		{
			testName:             "PurgesWhenOldShareFileNotEmpty",
			oldShareFileLocation: "SomeShareFile",
			newShareFileLocation: "",
			shouldPurge:          true,
			purgeConfigVal:       true,
		},
		{
			testName:             "PurgesWhenOldShareFileNotEmpty",
			oldShareFileLocation: "SomeShareFile",
			newShareFileLocation: "",
			shouldPurge:          false,
			purgeConfigVal:       false,
		},
		{
			testName:             "DoesNotPurgeWhenShareFileSameAndNotEmpty",
			oldShareFileLocation: "SomeShareFile",
			newShareFileLocation: "SomeShareFile",
			shouldPurge:          false,
		},
		{
			testName:             "DoesNotPurgeWhenShareFileSameAndEmpty",
			oldShareFileLocation: "",
			newShareFileLocation: "",
			shouldPurge:          false,
		},
		{
			testName:             "DoesNotPurgeWhenOldShareFileIsDefaultAndNewIsNotEmpty",
			oldShareFileLocation: defaultShareLocation,
			newShareFileLocation: "SomeShareFile",
			shouldPurge:          false,
		},
		{
			testName:             "DoesNotPurgeWhenOldShareFileIsDefaultAndNewIsEmpty",
			oldShareFileLocation: defaultShareLocation,
			newShareFileLocation: "",
			shouldPurge:          false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
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
				CredentialsExpiresAt: fiveMinBeforeTime,
				ShareFile:            tc.oldShareFileLocation,
			}

			runtimeConfigClient := &runtimeconfigmocks.IIdentityRuntimeConfigClient{}
			runtimeConfigClient.On("SaveConfig", mock.Anything).Return(nil).Once()
			provider := &credentialmocks.IRemoteProvider{}
			provider.On("ShareFile").Return(tc.newShareFileLocation, nil).Once()
			provider.On("RemoteRetrieve", mock.Anything).Return(credentials.Value{}, nil).Once()
			provider.On("RemoteExpiresAt").Return(time.Now().Add(1 * time.Hour)).Once()
			provider.On("CredentialSource").Return("").Once()
			mockAgentIdentity := &identityMock.IAgentIdentity{}
			mockAgentIdentity.On("IdentityType").Return(onprem.IdentityType)

			purgeCalled := atomic.Value{}
			purgeCalled.Store(false)

			purgeSharedCredentials = func(shareFilePath string) error {
				purgeCalled.Store(true)
				if !tc.shouldPurge || !tc.purgeConfigVal {
					assert.Fail(t, fmt.Sprintf("Purging credentials at path %q when credentials should not be purged", shareFilePath))
				}

				assert.Equal(t, tc.oldShareFileLocation, shareFilePath)
				return nil
			}

			newSharedCredentials = func(filename, profile string) *credentials.Credentials {
				return credentials.NewCredentials(provider)
			}

			c := &credentialsRefresher{
				log:                          logmocks.NewMockLog(),
				agentIdentity:                mockAgentIdentity,
				provider:                     provider,
				runtimeConfigClient:          runtimeConfigClient,
				identityRuntimeConfig:        runtimeConfig,
				credsReadyOnce:               sync.Once{},
				credentialsReadyChan:         make(chan struct{}, 1),
				stopCredentialRefresherChan:  make(chan struct{}),
				isCredentialRefresherRunning: true,
				getCurrentTimeFunc:           func() time.Time { return currentTime },
				timeAfterFunc:                time.After,
				appConfig: &appconfig.SsmagentConfig{Agent: appconfig.AgentInfo{
					ShouldPurgeInstanceProfileRoleCreds: tc.purgeConfigVal,
				}},
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
			assert.Equal(t, tc.shouldPurge, purgeCalled.Load())

			c.identityRuntimeConfig.CredentialsRetrievedAt.Equal(currentTime)
			c.identityRuntimeConfig.CredentialsExpiresAt.Equal(tenMinAfterTime)
		})
	}
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

	runtimeConfigClient := &runtimeconfigmocks.IIdentityRuntimeConfigClient{}
	runtimeConfigClient.On("SaveConfig", mock.Anything).Return(nil).Once()

	provider := &credentialmocks.IRemoteProvider{}
	provider.On("ShareFile").Return("SomeShareFile", nil).Times(1)
	provider.On("Retrieve").Return(credentials.Value{}, fmt.Errorf("share file doesn't exist")).Once()
	provider.On("RemoteRetrieve", mock.Anything).Return(credentials.Value{}, nil).Once()
	provider.On("RemoteExpiresAt").Return(time.Now().Add(1 * time.Hour)).Once()
	provider.On("CredentialSource").Return("SSM").Once()
	mockAgentIdentity := &identityMock.IAgentIdentity{}
	mockAgentIdentity.On("IdentityType").Return(onprem.IdentityType)

	newSharedCredentials = func(filename, profile string) *credentials.Credentials {
		return credentials.NewCredentials(provider)
	}

	c := &credentialsRefresher{
		log:                          logmocks.NewMockLog(),
		agentIdentity:                mockAgentIdentity,
		provider:                     provider,
		runtimeConfigClient:          runtimeConfigClient,
		identityRuntimeConfig:        runtimeConfig,
		credsReadyOnce:               sync.Once{},
		credentialsReadyChan:         make(chan struct{}, 1),
		stopCredentialRefresherChan:  make(chan struct{}),
		isCredentialRefresherRunning: true,
		getCurrentTimeFunc:           func() time.Time { return currentTime },
		timeAfterFunc:                time.After,
		appConfig:                    &appconfig.SsmagentConfig{Agent: appconfig.AgentInfo{}},
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

func Test_credentialsRefresher_retrieveCredsWithRetry_ValidateSleepDuration(t *testing.T) {
	const maxLongSleepDuration = 30 * time.Minute
	const minLongSleepDuration = 25 * time.Minute

	const minSleepDuration = 1 * time.Second
	const maxSleepDuration = 10 * time.Second

	const minUnknownAwsErrorSleepDuration = minSleepDuration + 10*time.Second
	const maxUnknownAwsErrorSleepDuration = maxSleepDuration + 20*time.Second
	const ec2PreEndueSleepMinDuration = 5 * time.Minute
	const ec2PreEndueSleepMaxDuration = 10 * time.Minute
	const unrecognizedIdentity = "UnrecognizedIdentity"

	testCases := []struct {
		TestName         string
		IdentityType     string
		Error            error
		MaxSleepDuration time.Duration
		MinSleepDuration time.Duration
	}{
		{
			TestName:         "EC2IdentityEC2LongSleepOnAccessDenied",
			IdentityType:     ec2.IdentityType,
			Error:            fmt.Errorf("ADE: %w", awsTestError{ErrCodeAccessDeniedException}),
			MinSleepDuration: minLongSleepDuration,
			MaxSleepDuration: maxLongSleepDuration,
		},
		{
			TestName:         "EC2IdentityEC2DefaultSleepOnInvalidInstanceId",
			IdentityType:     ec2.IdentityType,
			Error:            fmt.Errorf("invalid instance id: %w", awsTestError{ErrCodeInvalidInstanceId}),
			MinSleepDuration: ec2PreEndueSleepMinDuration,
			MaxSleepDuration: ec2PreEndueSleepMaxDuration,
		},
		{
			TestName:         "EC2IdentityEC2DefaultSleepOnUnknownError",
			IdentityType:     ec2.IdentityType,
			Error:            fmt.Errorf("unknown error: %w", awsTestError{"UnknownError"}),
			MinSleepDuration: ec2PreEndueSleepMinDuration,
			MaxSleepDuration: ec2PreEndueSleepMaxDuration,
		},
		{
			TestName:         "EC2IdentityEC2DefaultSleepOnHttpStatusNotFound",
			IdentityType:     ec2.IdentityType,
			Error:            fmt.Errorf("http status error: %w", awserr.NewRequestFailure(awsTestError{"NotKnownErrorCode"}, http.StatusNotFound, "")),
			MinSleepDuration: ec2PreEndueSleepMinDuration,
			MaxSleepDuration: ec2PreEndueSleepMaxDuration,
		},
		{
			TestName:         "EC2IdentityEC2DefaultOnHttpStatusTooManyRequests",
			IdentityType:     ec2.IdentityType,
			Error:            awserr.NewRequestFailure(awsTestError{"NotKnownErrorCode"}, http.StatusTooManyRequests, ""),
			MinSleepDuration: ec2PreEndueSleepMinDuration,
			MaxSleepDuration: ec2PreEndueSleepMaxDuration,
		},
		{
			TestName:         "EC2IdentityEC2DefaultOnUnrecognizedHttpStatusCode",
			IdentityType:     ec2.IdentityType,
			Error:            awserr.NewRequestFailure(awsTestError{"NotKnownErrorCode"}, http.StatusUpgradeRequired, ""),
			MinSleepDuration: ec2PreEndueSleepMinDuration,
			MaxSleepDuration: ec2PreEndueSleepMaxDuration,
		},
		{
			TestName:         "EC2IdentityEC2DefaultSleepOnGenericError",
			IdentityType:     ec2.IdentityType,
			Error:            fmt.Errorf("generic non-aws error"),
			MinSleepDuration: ec2PreEndueSleepMinDuration,
			MaxSleepDuration: ec2PreEndueSleepMaxDuration,
		},
		{
			TestName:         "OnPremIdentityLongSleepOnAccessDeniedException",
			IdentityType:     onprem.IdentityType,
			Error:            awsTestError{ErrCodeAccessDeniedException},
			MinSleepDuration: minLongSleepDuration,
			MaxSleepDuration: maxLongSleepDuration,
		},
		{
			TestName:         "OnPremIdentityLongSleepOnMachineFingerprintDoesNotMatch",
			IdentityType:     onprem.IdentityType,
			Error:            awsTestError{"MachineFingerprintDoesNotMatch"},
			MinSleepDuration: minLongSleepDuration,
			MaxSleepDuration: maxLongSleepDuration,
		},
		{
			TestName:         "OnPremIdentityMediumSleepOnInvalidInstanceId",
			IdentityType:     onprem.IdentityType,
			Error:            awsTestError{ErrCodeInvalidInstanceId},
			MinSleepDuration: minUnknownAwsErrorSleepDuration,
			MaxSleepDuration: maxUnknownAwsErrorSleepDuration,
		},
		{
			TestName:         "OnPremIdentityMediumSleepOnHttpNotFound",
			IdentityType:     onprem.IdentityType,
			Error:            awserr.NewRequestFailure(awsTestError{"NotKnownErrorCode"}, http.StatusNotFound, ""),
			MinSleepDuration: minUnknownAwsErrorSleepDuration,
			MaxSleepDuration: maxUnknownAwsErrorSleepDuration,
		},
		{
			TestName:         "OnPremIdentityMediumSleepOnUnrecognizedHttpStatusCode",
			IdentityType:     onprem.IdentityType,
			Error:            awserr.NewRequestFailure(awsTestError{"NotKnownErrorCode"}, http.StatusUpgradeRequired, ""),
			MinSleepDuration: minUnknownAwsErrorSleepDuration,
			MaxSleepDuration: maxUnknownAwsErrorSleepDuration,
		},
		{
			TestName:         "OnPremIdentityShortSleepOnGenericError",
			IdentityType:     onprem.IdentityType,
			Error:            fmt.Errorf("generic non-aws error"),
			MinSleepDuration: minSleepDuration,
			MaxSleepDuration: maxSleepDuration,
		},
		{
			TestName:         "UnrecognizedIdentityMediumSleepOnKnownAwsError",
			IdentityType:     unrecognizedIdentity,
			Error:            awsTestError{"UnknownError"},
			MinSleepDuration: minUnknownAwsErrorSleepDuration,
			MaxSleepDuration: maxUnknownAwsErrorSleepDuration,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.TestName, func(t *testing.T) {
			provider := &credentialmocks.IRemoteProvider{}
			provider.On("RemoteRetrieve", mock.Anything).Return(credentials.Value{}, tc.Error).Once()
			mockAgentIdentity := &identityMock.IAgentIdentity{}
			mockAgentIdentity.On("IdentityType").Return(tc.IdentityType)
			var timeAfterParamVal time.Duration
			c := &credentialsRefresher{
				log:                         logmocks.NewMockLog(),
				agentIdentity:               mockAgentIdentity,
				provider:                    provider,
				stopCredentialRefresherChan: make(chan struct{}),
				timeAfterFunc: func(duration time.Duration) <-chan time.Time {
					timeAfterParamVal = duration
					c := make(chan time.Time)
					return c
				},
				appConfig: &appconfig.SsmagentConfig{Agent: appconfig.AgentInfo{}},
			}

			waitGrp := sync.WaitGroup{}
			waitGrp.Add(1)
			stopped := false
			go func() {
				defer waitGrp.Done()
				_, stopped = c.retrieveCredsWithRetry(nil)
			}()

			// Allow retrieve to finish one round
			time.Sleep(time.Millisecond * 100)

			// When unrecognized identity present, return from cred refresher
			if tc.IdentityType == unrecognizedIdentity {
				assert.True(t, stopped, "cred refresher is not stopped")
				return
			}

			// Verify sleep duration
			assert.True(t, timeAfterParamVal >= tc.MinSleepDuration,
				fmt.Sprintf("duration until retry should be greater than or equal to %vs but is %vs", tc.MinSleepDuration.Seconds(), timeAfterParamVal.Seconds()))
			assert.True(t, timeAfterParamVal < tc.MaxSleepDuration,
				fmt.Sprintf("duration until retry should be less than %vs but is %vs", tc.MaxSleepDuration.Seconds(), timeAfterParamVal.Seconds()))
			provider.AssertExpectations(t)

			// stop
			c.stopCredentialRefresherChan <- struct{}{}

			waitGrp.Wait()
			assert.True(t, stopped, "expected retrieve to have been stopped by channel message")
		})
	}
}

func TestCredUtilityFunctions_sleepRetry_minMaxTesting(t *testing.T) {
	minSeconds := getDefaultBackoffRetryJitterSleepDuration(0).Seconds()
	for i := 0; i < 17; i++ {
		seconds := getDefaultBackoffRetryJitterSleepDuration(i)
		assert.True(t, seconds >= 0, "non negative value not allowed")
		assert.NotNil(t, seconds, "No Panic in backoff jitter")
	}
	maxSeconds := getDefaultBackoffRetryJitterSleepDuration(16).Seconds()
	assert.True(t, 1 <= minSeconds && minSeconds <= 3, "wrong min value for default backoff jitter")
	assert.True(t, 25*60 <= maxSeconds && maxSeconds <= 30*60, "wrong max value for default backoff jitter")

	minSeconds = getEC2DefaultSSMSleepDuration(0).Seconds()
	for i := 0; i < 17; i++ {
		seconds := getEC2DefaultSSMSleepDuration(i)
		assert.True(t, seconds >= 0, "non negative value not allowed")
		assert.NotNil(t, seconds, "No Panic in ec2 pre default jitter")
	}
	maxSeconds = getEC2DefaultSSMSleepDuration(16).Seconds()
	assert.True(t, 300 <= minSeconds && minSeconds <= 300, "wrong min value for ec2 pre default jitter")
	assert.True(t, 3200 <= maxSeconds && maxSeconds <= 3600, "wrong max value for ec2 pre default jitter")

	minSeconds = getMediumBackoffRetryJitterSleepDuration(0).Seconds()
	for i := 0; i < 17; i++ {
		seconds := getMediumBackoffRetryJitterSleepDuration(i)
		assert.True(t, seconds >= 0, "non negative value not allowed")
		assert.NotNil(t, seconds, "No Panic in ec2 medium sleep jitter")
	}
	maxSeconds = getMediumBackoffRetryJitterSleepDuration(16).Seconds()
	assert.True(t, 10 <= minSeconds && minSeconds <= 30, "wrong min value for medium sleep jitter")
	assert.True(t, 25*60 <= maxSeconds && maxSeconds <= 30*60, "wrong max value for medium sleep jitter")

	minSeconds = getLongSleepDuration(0).Seconds()
	for i := 0; i < 17; i++ {
		seconds := getLongSleepDuration(i)
		assert.True(t, seconds >= 0, "non negative value not allowed")
		assert.NotNil(t, seconds, "No Panic in long sleep jitter")
	}
	maxSeconds = getLongSleepDuration(16).Seconds()
	assert.True(t, 25*60 <= minSeconds && minSeconds <= 30*60, "wrong min value for long sleep jitter")
	assert.True(t, 25*60 <= maxSeconds && maxSeconds <= 30*60, "wrong max value for long sleep jitter")
}

func Test_credentialsRefresher_retrieveCredsWithRetry_OnpremRetry2000TimesNoExitUntilSuccess(t *testing.T) {
	mockLog := logmocks.NewEmptyLogMock()
	provider := &credentialmocks.IRemoteProvider{}
	provider.On("RemoteRetrieve", mock.Anything).Return(credentials.Value{}, awsTestError{"PotentiallyRecoverableAWSError"}).Times(1000)
	provider.On("RemoteRetrieve", mock.Anything).Return(credentials.Value{}, fmt.Errorf("SomeRandomNonAwsErr1")).Times(1000)
	mockLog.On("Info", mock.Anything).Times(2000)
	mockLog.On("Error", mock.Anything).Times(2000)
	provider.On("RemoteRetrieve", mock.Anything).Return(credentials.Value{}, nil).Once()
	mockAgentIdentity := &identityMock.IAgentIdentity{}
	mockAgentIdentity.On("IdentityType").Return(onprem.IdentityType)

	numSleeps := 0
	c := &credentialsRefresher{
		log:                         mockLog,
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
		appConfig: &appconfig.SsmagentConfig{Agent: appconfig.AgentInfo{}},
	}

	_, stopped := c.retrieveCredsWithRetry(nil)

	provider.AssertExpectations(t)
	mockLog.AssertExpectations(t)
	assert.Equal(t, 2000, numSleeps, "Number of retries was not correct")
	assert.False(t, stopped, "expected retrieve to not have been stopped by channel message")
}

func Test_credentialsRefresher_retrieveCredsWithRetry_EC2Retry2000TimesNoExitUntilSuccess(t *testing.T) {
	mockLog := logmocks.NewEmptyLogMock()
	provider := &credentialmocks.IRemoteProvider{}
	provider.On("RemoteRetrieve", mock.Anything).Return(credentials.Value{}, awsTestError{"PotentiallyRecoverableAWSError"}).Times(1000)
	provider.On("RemoteRetrieve", mock.Anything).Return(credentials.Value{}, fmt.Errorf("SomeRandomNonAwsErr1")).Times(1000)

	mockLog.On("Info", mock.Anything).Times(3)
	mockLog.On("Error", mock.Anything).Times(3)
	mockLog.On("Debug", mock.Anything).Times(1997 * 2)

	provider.On("RemoteRetrieve", mock.Anything).Return(credentials.Value{}, nil).Once()
	mockAgentIdentity := &identityMock.IAgentIdentity{}
	mockAgentIdentity.On("IdentityType").Return(ec2.IdentityType)

	numSleeps := 0
	c := &credentialsRefresher{
		log:                         mockLog,
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
		appConfig: &appconfig.SsmagentConfig{Agent: appconfig.AgentInfo{}},
	}

	_, stopped := c.retrieveCredsWithRetry(nil)

	provider.AssertExpectations(t)
	mockLog.AssertExpectations(t)
	assert.Equal(t, 2000, numSleeps, "Number of retries was not correct")
	assert.False(t, stopped, "expected retrieve to not have been stopped by channel message")
}

func Test_credentialsRefresher_isDocumentSessionWorkerProcessRunning_Success(t *testing.T) {
	executorMock := mocks.IExecutor{}
	newProcessExecutor = func(log log.T) executor.IExecutor {
		return &executorMock
	}
	c := &credentialsRefresher{
		log: logmocks.NewMockLog(),
	}

	// 2 workers present
	documentWorkerProcess := executor.OsProcess{Executable: "/usr/bin/ssm-document-worker"}
	sessionWorkerProcess := executor.OsProcess{Executable: "/usr/bin/ssm-session-worker"}
	processList := []executor.OsProcess{
		documentWorkerProcess,
		sessionWorkerProcess,
	}
	executorMock.On("Processes").Return(processList, nil)

	isPresent := c.isDocumentSessionWorkerProcessRunning()
	assert.True(t, isPresent, "document and session worker not present")

	// session worker alone present
	processList = []executor.OsProcess{
		sessionWorkerProcess,
	}
	executorMock = mocks.IExecutor{}
	executorMock.On("Processes").Return(processList, nil)
	isPresent = c.isDocumentSessionWorkerProcessRunning()
	assert.True(t, isPresent, "document and session worker not present")

	// document worker alone present
	processList = []executor.OsProcess{
		documentWorkerProcess,
	}
	executorMock = mocks.IExecutor{}
	executorMock.On("Processes").Return(processList, nil)
	isPresent = c.isDocumentSessionWorkerProcessRunning()
	assert.True(t, isPresent, "document and session worker not present")
}

func Test_credentialsRefresher_isDocumentSessionWorkerProcessRunning_Failed(t *testing.T) {
	executorMock := mocks.IExecutor{}
	newProcessExecutor = func(log log.T) executor.IExecutor {
		return &executorMock
	}
	c := &credentialsRefresher{
		log: logmocks.NewMockLog(),
	}

	// workers not present
	noWorkerProcess := executor.OsProcess{Executable: "/usr/bin/no-worker"}
	processList := []executor.OsProcess{
		noWorkerProcess,
	}
	executorMock.On("Processes").Return(processList, nil)

	isPresent := c.isDocumentSessionWorkerProcessRunning()
	assert.False(t, isPresent, "document and session worker present")

	// process throws error
	documentWorkerProcess := executor.OsProcess{Executable: "/usr/bin/ssm-document-worker"}
	processList = []executor.OsProcess{
		documentWorkerProcess,
	}
	executorMock.On("Processes").Return(processList, fmt.Errorf("error"))

	isPresent = c.isDocumentSessionWorkerProcessRunning()
	assert.False(t, isPresent, "document and session worker present")
}

func Test_credentialsRefresher_checkCredSaveDefaultSSMAgentPresent_Success(t *testing.T) {
	dateTimeNow := time.Now().Format("2006-01-02")
	getFileNames = func(srcPath string) (files []string, err error) {
		return []string{"amazon-ssm-agent-audit-" + dateTimeNow}, nil
	}
	visitedCount := 0
	isCredSaveDefaultSSMAgentVersionPresentUsingReader = func(reader io.Reader) bool {
		visitedCount++
		return true
	}
	osOpen = func(name string) (*os.File, error) {
		return &os.File{}, nil
	}
	c := &credentialsRefresher{
		log: logmocks.NewMockLog(),
	}
	isPresent := c.credentialFileConsumerPresent()
	assert.True(t, isPresent, "version not present")
	assert.Equal(t, 1, visitedCount)

	getFileNames = func(srcPath string) (files []string, err error) {
		return []string{"testFileName-" + dateTimeNow}, nil
	}
	visitedCount = 0
	isCredSaveDefaultSSMAgentVersionPresentUsingReader = func(reader io.Reader) bool {
		visitedCount++
		return true
	}
	isPresent = c.credentialFileConsumerPresent()
	assert.False(t, isPresent, "version present")
	assert.Equal(t, 0, visitedCount)

	getFileNames = func(srcPath string) (files []string, err error) {
		return []string{dateTimeNow}, nil
	}
	visitedCount = 0
	isCredSaveDefaultSSMAgentVersionPresentUsingReader = func(reader io.Reader) bool {
		visitedCount++
		return true
	}
	isPresent = c.credentialFileConsumerPresent()
	assert.False(t, isPresent, "version present")
	assert.Equal(t, 0, visitedCount)
}

func Test_credentialsRefresher_checkCredSaveDefaultSSMAgentPresent_Failed(t *testing.T) {
	osOpen = func(name string) (*os.File, error) {
		return &os.File{}, nil
	}
	c := &credentialsRefresher{
		log: logmocks.NewMockLog(),
	}
	dateTimeNow := time.Now().Format("2006-01-02")
	dateTimePrev := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	dateTimePrev2 := time.Now().AddDate(0, 0, -2).Format("2006-01-02")
	visitedCount := 0
	isCredSaveDefaultSSMAgentVersionPresentUsingReader = func(reader io.Reader) bool {
		visitedCount++
		return false
	}
	getFileNames = func(srcPath string) (files []string, err error) {
		return []string{
			"amazon-ssm-agent-audit-" + dateTimeNow,
			"amazon-ssm-agent-audit-" + dateTimePrev,
			"amazon-ssm-agent-audit-" + dateTimePrev2,
		}, nil
	}
	isPresent := c.credentialFileConsumerPresent()
	assert.False(t, isPresent, "version not present")
	assert.Equal(t, 3, visitedCount)
}

func Test_credentialsRefresher_isCredSaveDefaultSSMAgentVersionPresentUsingIoReader_Success(t *testing.T) {
	timeStamp := "22:59:59"
	nextTimeStamp := "23:00:00"

	file1Input := "SchemaVersion=1\n" +
		"agent_telemetry amazon-ssm-agent.start 2.2.1.0 " + timeStamp +
		"\nagent_telemetry ssm-agent-worker.start 2.2.1.0 " + timeStamp +
		"\nagent_telemetry amazon-ssm-agent.start 3.2.1241.0 " + nextTimeStamp +
		"\nagent_telemetry ssm-agent-worker.start 3.2.1241.0 " + nextTimeStamp +
		"\n"
	reader := bytes.NewBuffer([]byte(file1Input))
	isPresent := isCredSaveDefaultSSMAgentVersionPresentUsingIoReader(reader)
	assert.True(t, isPresent)
}

func Test_credentialsRefresher_isCredSaveDefaultSSMAgentVersionPresentUsingIoReader_Failed(t *testing.T) {
	timeStamp := "22:59:59"
	nextTimeStamp := "23:00:00"

	file1Input := "SchemaVersion=1\n" +
		"agent_telemetry amazon-ssm-agent.start 2.2.1.0 " + timeStamp +
		"\nagent_telemetry ssm-agent-worker.start 2.2.1.0 " + timeStamp +
		"\nagent_telemetry amazon-ssm-agent.start 3.2.1251.0 " + nextTimeStamp +
		"\nagent_telemetry ssm-agent-worker.start 3.2.1251.0 " + nextTimeStamp +
		"\n"
	reader := bytes.NewBuffer([]byte(file1Input))
	isPresent := isCredSaveDefaultSSMAgentVersionPresentUsingIoReader(reader)
	assert.False(t, isPresent)
}

func Test_credentialsRefresher_credentialRefresherRoutine_CredFilePurge(t *testing.T) {
	testCases := []struct {
		testName         string
		credentialSource string
		inits            func()
		shouldPurge      bool
	}{
		{
			testName:         "PurgeCredsSuccessForEC2",
			credentialSource: "EC2",
			inits: func() {
				// update newProcessExecutor
				executorMock := mocks.IExecutor{}
				sessionWorkerProcess := executor.OsProcess{Executable: "/usr/bin/ssm-session-worker"}
				processList := []executor.OsProcess{
					sessionWorkerProcess,
				}
				executorMock.On("Processes").Return(processList, nil)
				newProcessExecutor = func(log log.T) executor.IExecutor {
					return &executorMock
				}

				// update isCredSaveDefaultSSMAgentVersionPresentUsingReader
				dateTimeNow := time.Now().Format("2006-01-02")
				getFileNames = func(srcPath string) (files []string, err error) {
					return []string{"amazon-ssm-agent-audit-" + dateTimeNow}, nil
				}
				visitedCount := 0
				isCredSaveDefaultSSMAgentVersionPresentUsingReader = func(reader io.Reader) bool {
					visitedCount++
					return true
				}
				osOpen = func(name string) (*os.File, error) {
					return &os.File{}, nil
				}
				fileExists = func(filePath string) bool {
					return true
				}
			},
			shouldPurge: false,
		},
		{
			testName:         "PurgeCredsFailureForEC2WithoutWorkers",
			credentialSource: "EC2",
			inits: func() {
				// update newProcessExecutor
				executorMock := mocks.IExecutor{}
				sessionWorkerProcess := executor.OsProcess{Executable: "/usr/bin/no-worker"}
				processList := []executor.OsProcess{
					sessionWorkerProcess,
				}
				executorMock.On("Processes").Return(processList, nil)
				newProcessExecutor = func(log log.T) executor.IExecutor {
					return &executorMock
				}

				// update isCredSaveDefaultSSMAgentVersionPresentUsingReader
				dateTimeNow := time.Now().Format("2006-01-02")
				getFileNames = func(srcPath string) (files []string, err error) {
					return []string{"amazon-ssm-agent-audit-" + dateTimeNow}, nil
				}
				visitedCount := 0
				isCredSaveDefaultSSMAgentVersionPresentUsingReader = func(reader io.Reader) bool {
					visitedCount++
					return true
				}
				osOpen = func(name string) (*os.File, error) {
					return &os.File{}, nil
				}
				fileExists = func(filePath string) bool {
					return true
				}
			},
			shouldPurge: true,
		},
		{
			testName:         "PurgeCredsFailureForEC2WithWorkersWithoutFile",
			credentialSource: "EC2",
			inits: func() {
				// update newProcessExecutor
				executorMock := mocks.IExecutor{}
				sessionWorkerProcess := executor.OsProcess{Executable: "/usr/bin/no-worker"}
				processList := []executor.OsProcess{
					sessionWorkerProcess,
				}
				executorMock.On("Processes").Return(processList, nil)
				newProcessExecutor = func(log log.T) executor.IExecutor {
					return &executorMock
				}

				// update isCredSaveDefaultSSMAgentVersionPresentUsingReader
				dateTimeNow := time.Now().Format("2006-01-02")
				getFileNames = func(srcPath string) (files []string, err error) {
					return []string{"amazon-ssm-agent-audit-" + dateTimeNow}, nil
				}
				visitedCount := 0
				isCredSaveDefaultSSMAgentVersionPresentUsingReader = func(reader io.Reader) bool {
					visitedCount++
					return true
				}
				osOpen = func(name string) (*os.File, error) {
					return &os.File{}, nil
				}
				fileExists = func(filePath string) bool {
					return false
				}
			},
			shouldPurge: false,
		},
	}

	for _, tc := range testCases {
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
		tc.inits()
		// Should rotate right away
		runtimeConfig := runtimeconfig.IdentityRuntimeConfig{
			CredentialsExpiresAt: fiveMinBeforeTime,
		}

		runtimeConfigClient := &runtimeconfigmocks.IIdentityRuntimeConfigClient{}
		runtimeConfigClient.On("SaveConfig", mock.Anything).Return(nil).Once()
		provider := &credentialmocks.IRemoteProvider{}
		provider.On("ShareFile").Return("sample", nil).Once()
		provider.On("RemoteRetrieve", mock.Anything).Return(credentials.Value{}, nil).Once()
		provider.On("RemoteExpiresAt").Return(time.Now().Add(1 * time.Hour)).Once()
		provider.On("CredentialSource").Return(tc.credentialSource).Once()
		mockAgentIdentity := &identityMock.IAgentIdentity{}
		mockAgentIdentity.On("IdentityType").Return(onprem.IdentityType)

		purgeCalled := atomic.Value{}
		purgeCalled.Store(false)

		purgeSharedCredentials = func(shareFilePath string) error {
			purgeCalled.Store(true)
			return nil
		}

		newSharedCredentials = func(filename, profile string) *credentials.Credentials {
			return credentials.NewCredentials(provider)
		}

		c := &credentialsRefresher{
			log:                          logmocks.NewMockLog(),
			agentIdentity:                mockAgentIdentity,
			provider:                     provider,
			runtimeConfigClient:          runtimeConfigClient,
			identityRuntimeConfig:        runtimeConfig,
			credsReadyOnce:               sync.Once{},
			credentialsReadyChan:         make(chan struct{}, 1),
			stopCredentialRefresherChan:  make(chan struct{}),
			isCredentialRefresherRunning: true,
			getCurrentTimeFunc:           func() time.Time { return currentTime },
			timeAfterFunc:                time.After,
			appConfig:                    &appconfig.SsmagentConfig{Agent: appconfig.AgentInfo{}},
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

		c.log.Infof(tc.testName)
		runtimeConfigClient.AssertExpectations(t)
		provider.AssertExpectations(t)
		assert.Equal(t, tc.shouldPurge, purgeCalled.Load())
	}
}
