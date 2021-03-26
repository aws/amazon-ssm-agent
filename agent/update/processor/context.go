// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package processor contains the methods for update ssm agent.
// It also provides methods for sendReply and updateInstanceInfo
package processor

import (
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/s3util"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updatemanifest"
	"github.com/aws/amazon-ssm-agent/common/identity"
)

// UpdateState represents the state of update process
type UpdateState string

const (
	// NotStarted represents the state value not started for agent update
	NotStarted UpdateState = "NotStarted"

	// Initialized represents the state value initialized for agent update
	Initialized UpdateState = "Initialized"

	// Staged represents the state value staged for agent update
	Staged UpdateState = "Staged"

	// Installed represents the state value installed for agent update
	Installed UpdateState = "Installed"

	// Rollback represents the state value rollback for agent update
	Rollback UpdateState = "Rollback"

	// RolledBack represents the state value rolledBack for agent update
	RolledBack UpdateState = "RolledBack"

	// Completed represents the state value completed for agent update
	Completed UpdateState = "Completed"

	// TestExecution represents state value denoting test execution on customer's instance
	TestExecution UpdateState = "TestExecution"
)

const (
	// maxAllowedUpdateDuration represents the maximum allowed agent update time in seconds
	maxAllowedUpdateDuration = 180
)

// ContextMgr reprents context management logics
type ContextMgr interface {
	uploadOutput(log log.T, updateDetail *UpdateDetail, orchestrationDir string) error
}

type contextManager struct {
	context context.T
}

// UpdateDetail Book keeping detail for Agent Update
type UpdateDetail struct {
	State              UpdateState
	Result             contracts.ResultStatus
	StandardOut        string
	StandardError      string
	OutputS3KeyPrefix  string
	OutputS3BucketName string
	StdoutFileName     string
	StderrFileName     string
	SourceVersion      string
	SourceLocation     string
	SourceHash         string
	TargetVersion      string
	TargetResolver     updateconstants.TargetVersionResolver
	TargetLocation     string
	TargetHash         string
	PackageName        string
	StartDateTime      time.Time
	EndDateTime        time.Time
	MessageID          string
	UpdateRoot         string
	RequiresUninstall  bool
	ManifestURL        string
	Manifest           updatemanifest.T
	SelfUpdate         bool
	AllowDowngrade     bool
}

// HasMessageID represents if update is triggered by run command
func (update *UpdateDetail) HasMessageID() bool {
	return len(update.MessageID) > 0
}

// AppendInfo appends messages to UpdateDetail StandardOut
func (update *UpdateDetail) AppendInfo(log log.T, format string, params ...interface{}) {
	message := fmt.Sprintf(format, params...)
	log.Infof(message)
	if update.StandardOut != "" {
		update.StandardOut = fmt.Sprintf("%v\n%v", update.StandardOut, message)
	} else {
		update.StandardOut = message
	}
}

// AppendError appends messages to UpdateDetail StandardError and StandardOut
func (update *UpdateDetail) AppendError(log log.T, format string, params ...interface{}) {
	message := fmt.Sprintf(format, params...)
	log.Errorf(message)
	if update.StandardOut != "" {
		update.StandardOut = fmt.Sprintf("%v\n%v", update.StandardOut, message)
	} else {
		update.StandardOut = message
	}
	if update.StandardError != "" {
		update.StandardError = fmt.Sprintf("%v\n%v", update.StandardError, message)
	} else {
		update.StandardError = message
	}
}

// processMessageID splits the messageID and returns the commandID part of it.
func processMessageID(messageID string) string {
	// MdsMessageID is in the format of : aws.ssm.CommandId.InstanceId
	// E.g (aws.ssm.2b196342-d7d4-436e-8f09-3883a1116ac3.i-57c0a7be)
	mdsMessageIDSplit := strings.Split(messageID, ".")
	return mdsMessageIDSplit[len(mdsMessageIDSplit)-2]
}

// GetCommandID verifies the regex of messageID and returns the commandID by calling processMessageID
func getCommandID(messageID string) (string, error) {
	//messageID format: E.g (aws.ssm.2b196342-d7d4-436e-8f09-3883a1116ac3.i-57c0a7be)
	if match, err := regexp.MatchString("aws\\.ssm\\..+\\.+", messageID); !match {
		return messageID, fmt.Errorf("invalid messageID format: %v | %v", messageID, err)
	}

	return processMessageID(messageID), nil
}

// getV12DocOrchDir returns the orchestration path for v1.2 document plugins
func getV12DocOrchDir(identity identity.IAgentIdentity, log log.T, update *UpdateDetail) string {
	shortInstanceId, err := identity.ShortInstanceID()

	if err != nil {
		log.Errorf("Cannot get instance id: %v", err)
	}

	var commandID string
	if update.HasMessageID() {
		commandID, _ = getCommandID(update.MessageID)
	}

	return fileutil.BuildPath(
		appconfig.DefaultDataStorePath,
		shortInstanceId,
		appconfig.DefaultDocumentRootDirName,
		"orchestration",
		commandID,
		updateconstants.DefaultOutputFolder)
}

// getV22DocOrchDir returns the orchestration path for v2.2 document plugins
func getV22DocOrchDir(identity identity.IAgentIdentity, log log.T, update *UpdateDetail) string {
	return fileutil.BuildPath(getV12DocOrchDir(identity, log, update), updateconstants.DefaultOutputFolder)
}

// isV22DocUpdate returns true if the v2.2 document plugin folder exists
func isV22DocUpdate(identity identity.IAgentIdentity, log log.T, update *UpdateDetail) bool {
	return fileutil.Exists(getV22DocOrchDir(identity, log, update))
}

// getOrchestrationDir returns the orchestration directory
func getOrchestrationDir(identity identity.IAgentIdentity, log log.T, update *UpdateDetail) string {
	if isV22DocUpdate(identity, log, update) {
		log.Debugf("Assuming v2.2 document is being executed")
		return getV22DocOrchDir(identity, log, update)
	}

	log.Debugf("Assuming v1.2 document is being executed")
	return getV12DocOrchDir(identity, log, update)
}

// uploadOutput uploads the stdout and stderr file to S3
func (c *contextManager) uploadOutput(log log.T, updateDetail *UpdateDetail, orchestrationDirectory string) (err error) {

	// upload outputs (if any) to s3
	uploadOutputsToS3 := func() {
		// delete temp outputDir once we're done
		defer func() {
			if err := fileutil.DeleteDirectory(updateutil.UpdateOutputDirectory(updateDetail.UpdateRoot)); err != nil {
				log.Error("error deleting directory", err)
			}
		}()

		// get stdout file path
		stdoutPath := updateutil.UpdateStdOutPath(orchestrationDirectory, updateDetail.StdoutFileName)
		s3Key := path.Join(updateDetail.OutputS3KeyPrefix, updateDetail.StdoutFileName)
		log.Debugf("Uploading %v to s3://%v/%v", stdoutPath, updateDetail.OutputS3BucketName, s3Key)
		if s3, err := s3util.NewAmazonS3Util(c.context, updateDetail.OutputS3BucketName); err == nil {
			if err := s3.S3Upload(log, updateDetail.OutputS3BucketName, s3Key, stdoutPath); err != nil {
				log.Errorf("failed uploading %v to s3://%v/%v \n err:%v",
					stdoutPath,
					updateDetail.OutputS3BucketName,
					s3Key,
					err)
			}
		} else {
			log.Errorf("s3 client initialization failed, not uploading %v to s3. err: %v", stdoutPath, err)
		}

		// get stderr file path
		stderrPath := updateutil.UpdateStdErrPath(orchestrationDirectory, updateDetail.StderrFileName)
		s3Key = path.Join(updateDetail.OutputS3KeyPrefix, updateDetail.StderrFileName)
		log.Debugf("Uploading %v to s3://%v/%v", stderrPath, updateDetail.OutputS3BucketName, s3Key)
		if s3, err := s3util.NewAmazonS3Util(c.context, updateDetail.OutputS3BucketName); err == nil {
			if err := s3.S3Upload(log, updateDetail.OutputS3BucketName, s3Key, stderrPath); err != nil {
				log.Errorf("failed uploading %v to s3://%v/%v \n err:%v", stderrPath, updateDetail.StderrFileName, s3Key, err)
			}
		} else {
			log.Errorf("s3 client initialization failed, not uploading %v to s3. err: %v", stderrPath, err)
		}
	}

	uploadOutputsToS3()

	return nil
}
