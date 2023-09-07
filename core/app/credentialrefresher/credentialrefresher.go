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
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/cenkalti/backoff/v4"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/backoffconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/log/logger"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/sharedCredentials"
	"github.com/aws/amazon-ssm-agent/agent/versionutil"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ec2roleprovider"
	"github.com/aws/amazon-ssm-agent/common/identity/endpoint"
	identity2 "github.com/aws/amazon-ssm-agent/common/identity/identity"
	"github.com/aws/amazon-ssm-agent/common/runtimeconfig"
	agentctx "github.com/aws/amazon-ssm-agent/core/app/context"
	"github.com/aws/amazon-ssm-agent/core/executor"
)

const (
	// last version in 3.2 that share cred file between workers for EC2
	defaultSSMEC2SharedFileUsageLastVersion = "3.2.1241.0"
	defaultSSMStartVersion                  = "3.2.0.0"
	credentialSourceEC2                     = ec2roleprovider.CredentialSourceEC2
)

var (
	storeSharedCredentials = sharedCredentials.Store
	purgeSharedCredentials = sharedCredentials.Purge
	backoffRetry           = backoff.Retry
	newSharedCredentials   = credentials.NewSharedCredentials

	fileExists                                         = fileutil.Exists
	getFileNames                                       = fileutil.GetFileNames
	newProcessExecutor                                 = executor.NewProcessExecutor
	osOpen                                             = os.Open
	isCredSaveDefaultSSMAgentVersionPresentUsingReader = isCredSaveDefaultSSMAgentVersionPresentUsingIoReader
)

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

func NewCredentialRefresher(context agentctx.ICoreAgentContext) ICredentialRefresher {
	return &credentialsRefresher{
		log:                          context.Log().WithContext("[CredentialRefresher]"),
		agentIdentity:                context.Identity(),
		provider:                     nil,
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
	credentialProvider, ok := identity2.GetRemoteProvider(c.agentIdentity)
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
	c.log.Info("Credentials ready")
	c.credsReadyOnce.Do(func() {
		c.credentialsReadyChan <- struct{}{}
		c.log.Flush()
	})
}

// retrieveCredsWithRetry will never exit unless it receives a message on stopChan or is able to successfully call Retrieve
func (c *credentialsRefresher) retrieveCredsWithRetry(ctx context.Context) (credentials.Value, bool) {
	retryCount := 0
	identityGetDurationMap, ok := identityGetDurationMaps[c.agentIdentity.IdentityType()]
	if !ok {
		c.log.Warnf("Failed to find AWS error code handler map for identity %s. Using default.", c.agentIdentity.IdentityType())
		identityGetDurationMap = defaultErrorCodeGetDurationMap
	}

	for {
		creds, err := c.provider.RemoteRetrieve(ctx)
		if err == nil {
			return creds, false
		}

		sleepDuration := getDefaultBackoffRetryJitterSleepDuration(retryCount)

		var awsErr awserr.Error
		if isAwsErr := errors.As(err, &awsErr); !isAwsErr {
			c.log.Errorf("Retrieve credentials produced error: %v", err)
		} else if getSleepDurationFunc, ok := identityGetDurationMap[awsErr.Code()]; ok {
			c.log.Errorf("Retrieve credentials produced aws error: %v", err)
			c.log.Tracef("Found %s identity error handler for %s error code", c.agentIdentity.IdentityType(), awsErr.Code())
			sleepDuration = getSleepDurationFunc(retryCount)
		} else if getSleepDurationFunc, ok = defaultErrorCodeGetDurationMap[awsErr.Code()]; ok {
			c.log.Errorf("Retrieve credentials produced aws error: %v", err)
			c.log.Tracef("Found default error handler for %s error code", awsErr.Code())
			sleepDuration = getSleepDurationFunc(retryCount)
		} else if awsRequestFailure, isRequestFailure := err.(awserr.RequestFailure); isRequestFailure {
			// Get error details
			c.log.Errorf("Status code %s returned from AWS API. RequestId: %s Message: %s",
				awsRequestFailure.StatusCode(),
				awsRequestFailure.RequestID(),
				awsRequestFailure.Message())

			statusCode := awsRequestFailure.StatusCode()
			if statusCode == http.StatusNotFound {
				c.log.Debug("This feature is not yet available in current region")
			}

			if getSleepDurationFunc, ok = httpStatusCodeGetDurationMap[statusCode]; ok {
				sleepDuration = getSleepDurationFunc(retryCount)
			} else {
				sleepDuration = getLongSleepDuration(retryCount)
			}
		} else {
			c.log.Errorf("Retrieve credentials produced aws error: %v", err)
			c.log.Tracef("No specific error handler found for %s error code", awsErr.Code())
			// Sleep additional 10 - 20 seconds in case of an aws error
			sleepDuration += time.Second * time.Duration(10+rand.Intn(10))
		}

		c.log.Infof("Sleeping for %v before retrying retrieve credentials", sleepDuration)

		// Max retry count is 16, which will sleep for about 18-22 hours
		if retryCount < 16 {
			retryCount++
		}

		select {
		case <-c.stopCredentialRefresherChan:
			return creds, true
		case <-c.timeAfterFunc(sleepDuration):
		}
	}
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
		if c.identityRuntimeConfig.ShareFile == "" && c.identityRuntimeConfig.CredentialSource == credentialSourceEC2 {
			c.sendCredentialsReadyMessage()
		} else {
			localCredsProvider := newSharedCredentials(c.identityRuntimeConfig.ShareFile, c.identityRuntimeConfig.ShareProfile)
			if _, err := localCredsProvider.Get(); err != nil {
				c.log.Warnf("Credentials are not available when they should be: %v", err)
				// set expiration and retrieved to beginning of time if shared credentials are not available to force credential refresh
				c.identityRuntimeConfig.CredentialsExpiresAt = time.Time{}
			} else {
				c.sendCredentialsReadyMessage()
			}
		}
	}

	c.log.Info("Starting credentials refresher loop")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for {
		select {
		case <-c.stopCredentialRefresherChan:
			c.log.Info("Stopping credentials refresher")
			c.log.Flush()
			return
		case <-c.timeAfterFunc(c.durationUntilRefresh()):
			c.log.Debug("Calling Retrieve on credentials provider")
			creds, stopped := c.retrieveCredsWithRetry(ctx)
			credentialsRetrievedAt := c.getCurrentTimeFunc()
			if stopped {
				c.log.Info("Stopping credentials refresher")
				c.log.Flush()
				return
			}
			credentialSource := c.provider.CredentialSource()
			isEC2CredentialSource := credentialSource == ec2roleprovider.CredentialSourceEC2
			isEc2CredFilePresent := fileExists(appconfig.DefaultEC2SharedCredentialsFilePath)

			c.log.Tracef("Credential source %v", isEC2CredentialSource)
			c.log.Tracef("Cred file present %v", isEc2CredFilePresent)

			isCredFilePurged := false

			if isEC2CredentialSource && isEc2CredFilePresent {
				documentSessionWorkerRunning := c.isDocumentSessionWorkerProcessRunning()
				credSaveDefaultSSMAgentPresent := c.credentialFileConsumerPresent()
				c.log.Tracef("Document/session worker source %v", documentSessionWorkerRunning)
				c.log.Tracef("Cred save default ssm agent %v", credSaveDefaultSSMAgentPresent)
				if !(documentSessionWorkerRunning && credSaveDefaultSSMAgentPresent) {
					c.log.Info("Starting credential purging")
					err = backoffRetry(func() error {
						return purgeSharedCredentials(appconfig.DefaultEC2SharedCredentialsFilePath)
					}, c.backoffConfig)
					if err != nil {
						c.log.Warnf("error while purging cred file: %v", err)
					} else {
						isCredFilePurged = true
					}
				}
			}

			// ShareFile may be updated after retrieveCredsWithRetry()
			newShareFile := c.provider.ShareFile()
			if isCredFilePurged {
				c.log.Info("Credential file purged")
			} else {
				// when ShouldPurgeInstanceProfileRoleCreds config is used,
				// the credential file created in 3.2 for EC2 will be deleted irrespective of whether doc/session worker is running or not
				c.tryPurgeCreds(newShareFile)
			}

			// skip saving when the credential source is EC2
			if !isEC2CredentialSource && newShareFile != "" {
				err = backoffRetry(func() error {
					return storeSharedCredentials(c.log, creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken,
						newShareFile, c.identityRuntimeConfig.ShareProfile, false)
				}, c.backoffConfig)

				// If failed, try once more with force
				if err != nil {
					c.log.Warn("Failed to write credentials to disk, attempting force write")
					err = storeSharedCredentials(c.log, creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken,
						newShareFile, c.identityRuntimeConfig.ShareProfile, true)
				}

				if err != nil {
					// Saving credentials has been retried 6 times at this point.
					c.log.Errorf("Failed to write credentials to disk even with force, retrying: %v", err)
					continue
				}

				c.log.Debug("Successfully stored credentials")
			}

			c.log.Debug("Writing runtime configuration with updated expiration time")
			configCopy := c.identityRuntimeConfig
			configCopy.CredentialsRetrievedAt = credentialsRetrievedAt
			configCopy.CredentialsExpiresAt = c.provider.RemoteExpiresAt()
			configCopy.ShareFile = newShareFile
			configCopy.CredentialSource = credentialSource
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

func (c *credentialsRefresher) tryPurgeCreds(newShareFile string) {
	// Credentials are not purged until agent versions where EC2 agent workers
	// only consume shared credentials are fully deprecated
	shouldPurgeCreds := newShareFile != c.identityRuntimeConfig.ShareFile && c.appConfig.Agent.ShouldPurgeInstanceProfileRoleCreds
	purgeFileLocation := c.identityRuntimeConfig.ShareFile

	if shouldPurgeCreds && purgeFileLocation != "" {
		if defaultSharedCredsFilePath, err := sharedCredentials.GetSharedCredsFilePath(""); err != nil {
			c.log.Warn("Failed to check whether old credential file location is default aws share location." +
				"Skipping purge of old credentials")
		} else if purgeFileLocation == defaultSharedCredsFilePath {
			c.log.Warn("Skipping purge of default aws shared credentials path")
		} else if err = purgeSharedCredentials(purgeFileLocation); err != nil {
			c.log.Warnf("Failed to purge old credentials. Err: %v", err)
		} else {
			c.log.Info("Credential file purged")
		}
	}
}

// isDocumentSessionWorkerProcessRunning checks whether document and session worker is running or not
func (c *credentialsRefresher) isDocumentSessionWorkerProcessRunning() bool {
	var isDocumentSessionWorkerFound = false
	var allProcesses []executor.OsProcess
	var err error
	processExecutor := newProcessExecutor(c.log)
	if allProcesses, err = processExecutor.Processes(); err != nil {
		c.log.Warnf("error while getting process list: %v", err)
		return isDocumentSessionWorkerFound
	}
	for _, process := range allProcesses {
		executableName := strings.ToLower(process.Executable)
		if strings.Contains(executableName, appconfig.SSMDocumentWorkerName) {
			c.log.Infof("document worker with pid <%v> running", process.Pid)
			isDocumentSessionWorkerFound = true
			break
		}
		if strings.Contains(executableName, appconfig.SSMSessionWorkerName) {
			c.log.Infof("session worker with pid <%v> running", process.Pid)
			isDocumentSessionWorkerFound = true
			break
		}
	}
	return isDocumentSessionWorkerFound
}

// credentialFileConsumerPresent checks whether default SSM agent which saves credential file was installed
// during the last 72 hours by looking into audit file
func (c *credentialsRefresher) credentialFileConsumerPresent() bool {
	credSaveDefaultSSMAgentVersionPresent := false
	auditFileDateTimeFormat := "2006-01-02"
	auditFolderPath := filepath.Join(logger.DefaultLogDir, logger.AuditFolderName)
	auditFileNames, err := getFileNames(auditFolderPath)
	if err != nil {
		c.log.Warnf("error while getting file names: %v", err)
		return credSaveDefaultSSMAgentVersionPresent
	}
	currentTimeStamp := time.Now()
	threeDaysBeforeDate := time.Date(currentTimeStamp.Year(), currentTimeStamp.Month(), currentTimeStamp.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -3)

	for _, fileName := range auditFileNames {
		// get date time from audit file name considering datetime format will be in "2006-01-02"
		dateFromAuditFileName := fileName[len(fileName)-len(auditFileDateTimeFormat):]

		// checks whether datetime in file name matches with format "2006-01-02"
		eventFileDateStamp, err := time.Parse(auditFileDateTimeFormat, dateFromAuditFileName)
		if err != nil {
			c.log.Warnf("error while parsing audit file name for date stamp: %v", err)
			credSaveDefaultSSMAgentVersionPresent = false
		}

		eventFileDateStampUTC := eventFileDateStamp.UTC()
		if err == nil && eventFileDateStampUTC.After(threeDaysBeforeDate) {
			func() {
				file, err := osOpen(filepath.Join(auditFolderPath, fileName))
				if err != nil {
					c.log.Warnf("error while reading audit file: %v", err)
					return
				}
				defer file.Close()

				if isCredSaveDefaultSSMAgentVersionPresentUsingReader(file) {
					credSaveDefaultSSMAgentVersionPresent = true
				}
			}()
		}

		if credSaveDefaultSSMAgentVersionPresent == true {
			c.log.Infof("audit file with cred save default SSM Agent present")
			break
		}
	}
	return credSaveDefaultSSMAgentVersionPresent
}

// isCredSaveDefaultSSMAgentVersionPresentUsingIoReader checks whether default SSM agent which saves
// credential file is present in reader or not
func isCredSaveDefaultSSMAgentVersionPresentUsingIoReader(reader io.Reader) bool {
	credSaveDefaultSSMAgentVersionPresent := false
	// Create a new scanner for the file.
	scanner := bufio.NewScanner(reader)
	// Loop over the lines in the file.
	for scanner.Scan() {
		splitVal := strings.Split(scanner.Text(), " ")
		// agent_telemetry event type should have only four fields
		if len(splitVal) != 4 {
			continue
		}
		// not an agent telemetry event type
		if splitVal[0] != logger.AgentTelemetryMessage {
			continue
		}
		versionCompare, err := versionutil.VersionCompare(splitVal[2], defaultSSMEC2SharedFileUsageLastVersion)
		if err != nil || versionCompare > 0 {
			continue
		}
		versionCompare, err = versionutil.VersionCompare(splitVal[2], defaultSSMStartVersion)
		if err != nil || versionCompare < 0 {
			continue
		}
		credSaveDefaultSSMAgentVersionPresent = true
		break
	}
	return credSaveDefaultSSMAgentVersionPresent
}
