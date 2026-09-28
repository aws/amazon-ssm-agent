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

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"

	"github.com/aws/amazon-ssm-agent/agent/sdkutil"

	"io"
	"math/rand"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/backoffconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/log/logger"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/sharedCredentials"
	"github.com/aws/amazon-ssm-agent/agent/startup/serialport"
	"github.com/aws/amazon-ssm-agent/agent/versionutil"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ec2roleprovider"
	"github.com/aws/amazon-ssm-agent/common/identity/endpoint"
	identity2 "github.com/aws/amazon-ssm-agent/common/identity/identity"
	"github.com/aws/amazon-ssm-agent/common/runtimeconfig"
	agentctx "github.com/aws/amazon-ssm-agent/core/app/context"
	"github.com/aws/amazon-ssm-agent/core/executor"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/cenkalti/backoff/v4"
	"github.com/cihub/seelog"
)

const (
	// last version in 3.2 that share cred file between workers for EC2
	defaultSSMEC2SharedFileUsageLastVersion = "3.2.1241.0"
	defaultSSMStartVersion                  = "3.2.0.0"
	credentialSourceEC2                     = ec2roleprovider.CredentialSourceEC2
)

// configCheckInterval is how often the credential refresher checks if the identity config file still exists
var configCheckInterval = 1 * time.Minute

var (
	storeSharedCredentials = sharedCredentials.Store
	purgeSharedCredentials = sharedCredentials.Purge
	backoffRetry           = backoff.Retry
	newSharedCredentials   = NewSharedCredentials

	fileExists                                         = fileutil.Exists
	getFileNames                                       = fileutil.GetFileNames
	newProcessExecutor                                 = executor.NewProcessExecutor
	osOpen                                             = os.Open
	isCredSaveDefaultSSMAgentVersionPresentUsingReader = isCredSaveDefaultSSMAgentVersionPresentUsingIoReader
	identityGetRemoteProvider                          = identity2.GetRemoteProvider
	newEndPointHelper                                  = endpoint.NewEndpointHelper
	defaultExponentialBackoff                          = backoffconfig.GetDefaultExponentialBackoff
	getSharedCredsFilePath                             = sharedCredentials.GetSharedCredsFilePath
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

	getCurrentTimeFunc       func() time.Time
	newSharedCredentialsFunc func(sharedCredsFile string, sharedProfile string, ctx context.Context) (aws.CredentialsProvider, error)
	timeAfterFunc            func(time.Duration) <-chan time.Time

	// Track if we've already logged credential failure to serial port
	hasLoggedCredentialFailure sync.Once
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
		endpointHelper:               newEndPointHelper(context.Log().WithContext("[EndpointHelper]"), *context.AppConfig()),
		appConfig:                    context.AppConfig(),
		hasLoggedCredentialFailure:   sync.Once{},
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
	credentialProvider, ok := identityGetRemoteProvider(c.agentIdentity)
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

	c.backoffConfig, err = defaultExponentialBackoff()
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
func (c *credentialsRefresher) retrieveCredsWithRetry(ctx context.Context) (aws.Credentials, bool) {
	retryCount := 0
	identityGetDurationMap, ok := identityGetDurationMaps[c.agentIdentity.IdentityType()]
	if !ok {
		c.log.Errorf("Unexpected identity retrieved: %v. Stopping credential refresher.", c.agentIdentity.IdentityType())
		return aws.Credentials{}, true
	}

	// Log credential failure to serial port only once
	logCredentialFailureToSerialPort := func(err error) {
		c.hasLoggedCredentialFailure.Do(func() {
			fullErrorMessage := fmt.Sprintf("SSM Agent unable to acquire credentials: <error>%v</error>", err)
			go serialport.EmitSerialPortMessage(c.log, fullErrorMessage)
		})
	}

	for {
		creds, err := c.provider.RemoteRetrieve(ctx)
		if err == nil {
			return creds, false
		}

		// this will log as debug when the credential refresher retries exceeds 3
		logMessage := fmt.Sprintf("Retrieve credentials produced error: %v", err)
		c.minLog(seelog.ErrorLvl, logMessage, retryCount)

		// Log to serial port on first failure
		logCredentialFailureToSerialPort(err)

		var sleepDuration time.Duration
		// initialize default sleep duration
		if sleepDurationFunc, ok := identityGetDurationMap[ErrAllOtherNonAWSErrors]; ok {
			sleepDuration = sleepDurationFunc(retryCount)
		} else {
			sleepDuration = getDefaultBackoffRetryJitterSleepDuration(retryCount)
		}

		// override sleep duration for aws errors. Route the error-code lookup
		// through sdkutil.GetAwsErrorCode instead of extracting smithy.APIError
		// directly — that way v2 error shapes that don't implement
		// smithy.APIError (e.g. the "no EC2 IMDS role found" fmt.Errorf from
		// ec2rolecreds when the instance profile is detached) still resolve to
		// a v1-compatible error code (EC2RoleRequestError) via GetAwsErrorCode's
		// fallback branches, keeping the backoff selection consistent with
		// pre-v2 behavior.
		if errCode := sdkutil.GetAwsErrorCode(err); errCode != "" {
			// Check if error is a non-retryable error if fingerprint changes or response is access denied exception
			if errCode == ErrCodeMachineFingerprintDoesNotMatch || errCode == ErrCodeAccessDeniedException {
				if sleepDurationFunc, ok := identityGetDurationMap[errCode]; ok {
					sleepDuration = sleepDurationFunc(retryCount)
				}
			} else {
				if sleepDurationFunc, ok := identityGetDurationMap[ErrAllOtherAWSErrors]; ok {
					sleepDuration = sleepDurationFunc(retryCount)
				}
				var awsRequestFailure *awshttp.ResponseError
				if isRequestFailure := errors.As(err, &awsRequestFailure); isRequestFailure {
					logMessage = fmt.Sprintf("Status code %v returned from AWS API. RequestId: %s Message: %s",
						awsRequestFailure.HTTPStatusCode(),
						awsRequestFailure.ServiceRequestID(),
						awsRequestFailure.Error())
					// this will log as debug when the credential refresher retries exceeds 3
					c.minLog(seelog.ErrorLvl, logMessage, retryCount)
				}
			}
		}

		logMessage = fmt.Sprintf("Sleeping for %v before retrying retrieve credentials", sleepDuration)
		// this will log as debug when the credential refresher retries exceeds 3
		c.minLog(seelog.InfoLvl, logMessage, retryCount)

		// Max retry count is 16, which will sleep for about 18-22 hours for long sleep strategy
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

func (c *credentialsRefresher) minLog(defaultLevel int, logMessage string, retryCount int) {
	isEc2 := c.agentIdentity.IdentityType() == ec2.IdentityType
	maxRetryCount := 3
	if retryCount >= maxRetryCount && isEc2 {
		c.log.Debug(logMessage)
		return
	}

	switch defaultLevel {
	case seelog.ErrorLvl:
		c.log.Error(logMessage)
	case seelog.InfoLvl:
		c.log.Info(logMessage)
	case seelog.DebugLvl:
		c.log.Debug(logMessage)
	default:
		c.log.Info(logMessage)
	}
}

func NewSharedCredentials(sharedCredsFile string, sharedProfile string, ctx context.Context) (aws.CredentialsProvider, error) {
	// Normalize an empty profile to "default" to match the write side.
	// agent/managedInstances/sharedCredentials.Store maps "" -> "default"
	// before creating the [default] section in the shared credentials file,
	// but LoadSharedConfigProfile treats "" as a literal section name and
	// would return SharedConfigProfileNotExistError even when [default]
	// exists.
	if sharedProfile == "" {
		sharedProfile = "default"
	}

	// Parse the shared credentials file directly, without going through
	// LoadDefaultConfig. LoadDefaultConfig — even when constrained via
	// WithSharedCredentialsFiles — still consults environment variables and
	// EC2 IMDS in front of / behind the shared file, which would let a
	// caller-supplied AWS_ACCESS_KEY_ID override the share file or a missing
	// share file resolve to IMDS-role credentials. LoadSharedConfigProfile
	// parses only the credentials/config files, so neither happens.
	sc, err := awsconfig.LoadSharedConfigProfile(ctx, sharedProfile, func(o *awsconfig.LoadSharedConfigOptions) {
		o.CredentialsFiles = []string{sharedCredsFile}
		// Also disable ~/.aws/config processing to prevent
		// credential_process/SSO/AssumeRole from executing under the agent's
		// privilege (root/SYSTEM).
		o.ConfigFiles = []string{}
	})
	if err != nil {
		return nil, fmt.Errorf("load shared credentials for profile %q from %q: %w", sharedProfile, sharedCredsFile, err)
	}
	if sc.Credentials.AccessKeyID == "" || sc.Credentials.SecretAccessKey == "" {
		return nil, fmt.Errorf("no credentials found in %q for profile %q", sharedCredsFile, sharedProfile)
	}
	// Return a static provider bound to the on-disk credentials. The refresher
	// re-parses the file on each cycle, so no long-lived staleness.
	return credentials.StaticCredentialsProvider{Value: sc.Credentials}, nil
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
			var localCredsConfigProvider aws.CredentialsProvider
			var credErr error
			if c.newSharedCredentialsFunc != nil {
				localCredsConfigProvider, credErr = c.newSharedCredentialsFunc(c.identityRuntimeConfig.ShareFile, c.identityRuntimeConfig.ShareProfile, context.TODO())
			} else {
				localCredsConfigProvider, credErr = newSharedCredentials(c.identityRuntimeConfig.ShareFile, c.identityRuntimeConfig.ShareProfile, context.TODO())
			}
			if credErr != nil || localCredsConfigProvider == nil {
				c.log.Warnf("Failed to load shared credentials provider: %v", credErr)
				c.identityRuntimeConfig.CredentialsExpiresAt = time.Time{}
			} else if _, err := localCredsConfigProvider.Retrieve(context.TODO()); err != nil {
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

	configCheckTicker := time.NewTicker(configCheckInterval)
	defer configCheckTicker.Stop()

	for {
		select {
		case <-c.stopCredentialRefresherChan:
			c.log.Info("Stopping credentials refresher")
			c.log.Flush()
			return
		case <-configCheckTicker.C:
			// A cheaper minutely check for credential cache existence
			// If cache do not exist or is flushed, it restart the loop and trigger
			// a refresh as durationUntilRefresh would be evaluated to 0
			if exists, err := c.runtimeConfigClient.ConfigExists(); err != nil {
				c.log.Warnf("Error checking credential config existence: %v", err)
			} else if !exists {
				c.log.Info("Credential config file missing, triggering immediate credential refresh")
				c.identityRuntimeConfig.CredentialsExpiresAt = time.Time{}
			}
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
		if defaultSharedCredsFilePath, err := getSharedCredsFilePath(""); err != nil {
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
	isDocumentSessionWorkerFound := false
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
		// In random cases, non audit files are present on customer instances.
		// Ignore non audit files created in this folder
		if !strings.HasPrefix(fileName, logger.EventLogFile) {
			continue
		}

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
