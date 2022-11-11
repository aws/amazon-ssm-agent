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
	"math"
	"math/rand"
	"net/http"
	"runtime/debug"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/backoffconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/sharedCredentials"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders"
	"github.com/aws/amazon-ssm-agent/common/identity/endpoint"
	"github.com/aws/amazon-ssm-agent/common/runtimeconfig"
	"github.com/aws/amazon-ssm-agent/core/app/context"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/ssm"

	"github.com/cenkalti/backoff/v4"
)

var storeSharedCredentials = sharedCredentials.Store
var backoffRetry = backoff.Retry
var newSharedCredentials = credentials.NewSharedCredentials

// Indicates the retrier should retry call for systems manager provided credentials
var shouldRetry = true

// Map of known http responses when requesting credentials from systems manager
var statusCodes = map[int]bool{
	http.StatusInternalServerError: shouldRetry,
	http.StatusTooManyRequests:     shouldRetry,
	http.StatusBadRequest:          !shouldRetry,
	http.StatusUnauthorized:        !shouldRetry,
	http.StatusForbidden:           !shouldRetry,
	http.StatusMethodNotAllowed:    !shouldRetry,
}

type ICredentialRefresher interface {
	Start() error
	Stop()
	GetCredentialsReadyChan() chan struct{}
}

type credentialsRefresher struct {
	log           log.T
	appConfig     *appconfig.SsmagentConfig
	agentIdentity identity.IAgentIdentity
	provider      credentialproviders.IRemoteProvider
	expirer       credentials.Expirer

	runtimeConfigClient   runtimeconfig.IIdentityRuntimeConfigClient
	identityRuntimeConfig runtimeconfig.IdentityRuntimeConfig
	endpointHelper        endpoint.IEndpointHelper

	backoffConfig *backoff.ExponentialBackOff

	credsReadyOnce       sync.Once
	credentialsReadyChan chan struct{}

	stopCredentialRefresherChan  chan struct{}
	isCredentialRefresherRunning bool

	getCurrentTimeFunc func() time.Time
	timeAfterFunc      func(time.Duration) <-chan time.Time
}

func NewCredentialRefresher(context context.ICoreAgentContext) ICredentialRefresher {
	return &credentialsRefresher{
		log:                          context.Log().WithContext("[CredentialRefresher]"),
		agentIdentity:                context.Identity(),
		provider:                     nil,
		expirer:                      nil,
		runtimeConfigClient:          runtimeconfig.NewIdentityRuntimeConfigClient(),
		identityRuntimeConfig:        runtimeconfig.IdentityRuntimeConfig{},
		credsReadyOnce:               sync.Once{},
		credentialsReadyChan:         make(chan struct{}, 1),
		stopCredentialRefresherChan:  make(chan struct{}),
		isCredentialRefresherRunning: false,
		getCurrentTimeFunc:           time.Now,
		timeAfterFunc:                time.After,
		endpointHelper:               endpoint.NewEndpointHelper(context.Log().WithContext("[EndpointHelper]"), *context.AppConfig()),
		appConfig:                    context.AppConfig(),
	}
}

func (c *credentialsRefresher) durationUntilRefresh() time.Duration {
	timeNow := c.getCurrentTimeFunc()

	// Credentials are already expired, should be rotated now
	expiresAt := c.identityRuntimeConfig.CredentialsExpiresAt
	if expiresAt.Before(timeNow) || expiresAt.Equal(timeNow) {
		return time.Duration(0)
	}

	retrievedAt := c.identityRuntimeConfig.CredentialsRetrievedAt
	credentialsDuration := expiresAt.Sub(retrievedAt)

	// Set the expiration window to be half of the token's lifetime. This allows credential refreshes to survive transient
	// network issues more easily. Expiring at half the lifetime also follows the behavior of other protocols such as DHCP
	// https://tools.ietf.org/html/rfc2131#section-4.4.5. Note that not all of the behavior specified in that RFC is
	// implemented, just the suggestion to start renewals at 50% of token validity.
	rotateBeforeExpiryDuration := credentialsDuration / 2

	rotateAtTime := expiresAt.Add(-rotateBeforeExpiryDuration)
	rotateDuration := rotateAtTime.Sub(timeNow)
	c.log.Infof("Next credential rotation will be in %v minutes", rotateDuration.Minutes())
	return rotateDuration
}

func (c *credentialsRefresher) Start() error {
	var err error
	credentialProvider, ok := identity.GetRemoteProvider(c.agentIdentity)
	if !ok {
		c.log.Info("Identity does not require credential refresher")
		c.sendCredentialsReadyMessage()
		return nil
	}

	if !credentialProvider.SharesCredentials() {
		c.log.Info("Identity does not want core agent to rotate credentials")
		c.sendCredentialsReadyMessage()
		return nil
	}

	c.provider = credentialProvider

	// provider should always implement expirer
	if c.expirer, ok = c.provider.(credentials.Expirer); !ok {
		return fmt.Errorf("credentials provider for identity %v does not implement Expirer interface", c.agentIdentity.IdentityType())
	}

	// Initialize the identity runtime config from disk
	if c.identityRuntimeConfig, err = c.runtimeConfigClient.GetConfig(); err != nil {
		return err
	}

	c.backoffConfig, err = backoffconfig.GetDefaultExponentialBackoff()
	if err != nil {
		return fmt.Errorf("error creating backoff config: %v", err)
	}

	// Seed random number generator used to generate jitter during retry
	rand.Seed(time.Now().UnixNano())

	go c.credentialRefresherRoutine()

	c.isCredentialRefresherRunning = true
	c.log.Infof("credentialRefresher has started")

	return nil
}

func (c *credentialsRefresher) Stop() {
	if !c.isCredentialRefresherRunning {
		return
	}

	c.log.Info("Sending credential refresher stop signal")
	c.stopCredentialRefresherChan <- struct{}{}
	c.isCredentialRefresherRunning = false
}

func (c *credentialsRefresher) GetCredentialsReadyChan() chan struct{} {
	return c.credentialsReadyChan
}

func (c *credentialsRefresher) sendCredentialsReadyMessage() {
	c.credsReadyOnce.Do(func() {
		c.credentialsReadyChan <- struct{}{}
		c.log.Flush()
	})
}

func getBackoffRetryJitterSleepDuration(retryCount int) time.Duration {
	expBackoff := math.Pow(2, float64(retryCount))
	return time.Duration(int(expBackoff)+rand.Intn(int(math.Ceil(expBackoff*0.2)))) * time.Second
}

// retrieveCredsWithRetry will never exit unless it receives a message on stopChan or is able to successfully call Retrieve
func (c *credentialsRefresher) retrieveCredsWithRetry() (credentials.Value, bool) {
	retryCount := 0
	for {
		creds, err := c.provider.Retrieve()
		if err == nil {
			return creds, false
		}

		// Default sleep duration for non-aws errors
		sleepDuration := getBackoffRetryJitterSleepDuration(retryCount)
		// Max retry count is 16, which will sleep for about 18-22 hours
		if retryCount < 16 {
			retryCount++
		}

		// Check if error is a non-retryable error if fingerprint changes or response is access denied exception
		if awsErr, isAwsErr := err.(awserr.Error); isAwsErr && (awsErr.Code() == ssm.ErrCodeMachineFingerprintDoesNotMatch || awsErr.Code() == "AccessDeniedException") {
			sleepDuration = getLongSleepDuration(sleepDuration)
			c.log.Errorf("Retrieve credentials produced unrecoverable aws error: %v", awsErr)

		} else if awsRequestFailure, isRequestFailure := err.(awserr.RequestFailure); isRequestFailure {
			// Get error details
			c.log.Warnf("Status code %s returned from AWS API. RequestId: %s Message: %s", awsRequestFailure.StatusCode(), awsRequestFailure.RequestID(), awsRequestFailure.Message())
			statusCode := awsRequestFailure.StatusCode()

			if statusCode == http.StatusNotFound {
				c.log.Debug("This feature is not yet available in current region")
			}

			if shouldRetry, found := statusCodes[statusCode]; !found || !shouldRetry {
				sleepDuration = getLongSleepDuration(sleepDuration)
			}
		} else if isAwsErr {
			// Sleep additional 10 - 20 seconds in case of an aws error
			sleepDuration += time.Second * time.Duration(10+rand.Intn(10))
		} else {
			c.log.Errorf("Retrieve credentials produced error: %v", err)
		}

		c.log.Infof("Sleeping for %v before retrying retrieve credentials", sleepDuration)
		select {
		case <-c.stopCredentialRefresherChan:
			return creds, true
		case <-c.timeAfterFunc(sleepDuration):
		}
	}
}

func getLongSleepDuration(sleepDuration time.Duration) time.Duration {
	// Sleep 24 hours with random jitter of up to 2 hour if error is non-retryable to make sure we spread retries for large de-registered fleets
	jitter := time.Second * time.Duration(rand.Intn(7200))
	sleepDuration = 24*time.Hour + jitter
	return sleepDuration
}

func (c *credentialsRefresher) credentialRefresherRoutine() {
	var err error
	defer func() {
		if err := recover(); err != nil {
			c.log.Errorf("credentials refresher panic: %v", err)
			c.log.Errorf("Stacktrace:\n%s", debug.Stack())
			c.log.Flush()

			// We never want to exit this loop unless explicitly asked to do so, restart loop
			time.Sleep(5 * time.Minute)
			go c.credentialRefresherRoutine()
		}
	}()

	// if credentials are not expired, verify that credentials are available.
	if c.identityRuntimeConfig.CredentialsExpiresAt.After(c.getCurrentTimeFunc()) {
		localCredsProvider := newSharedCredentials(c.identityRuntimeConfig.ShareFile, c.identityRuntimeConfig.ShareProfile)
		if _, err := localCredsProvider.Get(); err != nil {
			c.log.Warnf("Credentials are not available when they should be: %v", err)
			// set expiration and retrieved to beginning of time if shared credentials are not available to force credential refresh
			c.identityRuntimeConfig.CredentialsExpiresAt = time.Time{}
		}
	}

	if c.identityRuntimeConfig.CredentialsExpiresAt.After(c.getCurrentTimeFunc()) {
		c.log.Info("Credentials exist and have not expired, sending ready message")
		c.sendCredentialsReadyMessage()
	}

	c.log.Info("Starting credentials refresher loop")
	for {
		select {
		case <-c.stopCredentialRefresherChan:
			c.log.Info("Stopping credentials refresher")
			c.log.Flush()
			return
		case <-c.timeAfterFunc(c.durationUntilRefresh()):
			c.log.Debug("Calling Retrieve on credentials provider")
			creds, stopped := c.retrieveCredsWithRetry()
			credentialsRetrievedAt := c.getCurrentTimeFunc()
			if stopped {
				c.log.Info("Stopping credentials refresher")
				c.log.Flush()
				return
			}

			err = backoffRetry(func() error {
				return storeSharedCredentials(c.log, creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken, c.identityRuntimeConfig.ShareFile, c.identityRuntimeConfig.ShareProfile, false)
			}, c.backoffConfig)

			// If failed, try once more with force
			if err != nil {
				c.log.Warn("Failed to write credentials to disk, attempting force write")
				err = storeSharedCredentials(c.log, creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken, c.identityRuntimeConfig.ShareFile, c.identityRuntimeConfig.ShareProfile, true)
			}

			if err != nil {
				// Saving credentials has been retried 6 times at this point.
				c.log.Errorf("Failed to write credentials to disk even with force, retrying: %v", err)
				continue
			}

			c.log.Debug("Successfully stored credentials, writing runtime configuration with updated expiration time")
			configCopy := c.identityRuntimeConfig
			configCopy.CredentialsRetrievedAt = credentialsRetrievedAt
			configCopy.CredentialsExpiresAt = c.expirer.ExpiresAt()

			err = backoffRetry(func() error {
				return c.runtimeConfigClient.SaveConfig(configCopy)
			}, c.backoffConfig)
			if err != nil {
				c.log.Warnf("Failed to save new expiration: %v", err)
				continue
			}

			c.identityRuntimeConfig = configCopy
			c.sendCredentialsReadyMessage()
		}
	}
}
