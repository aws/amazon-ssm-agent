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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	"regexp"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/s3util"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
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
)

const (
	// maxAllowedUpdateDuration represents the maximum allowed agent update time in seconds
	maxAllowedUpdateDuration = 180
)

// ContextMgr reprents context management logics
type ContextMgr interface {
	uploadOutput(log log.T, context *UpdateContext, orchestrationDir string) error
	saveUpdateContext(log log.T, context *UpdateContext, contextLocation string) error
}

type contextManager struct{}

// UpdateDetail Book keeping detail for Agent Update
type UpdateDetail struct {
	State              UpdateState            `json:"State"`
	Result             contracts.ResultStatus `json:"Result"`
	StandardOut        string                 `json:"StandardOut"`
	StandardError      string                 `json:"StandardError"`
	OutputS3KeyPrefix  string                 `json:"OutputS3KeyPrefix"`
	OutputS3BucketName string                 `json:"OutputS3BucketName"`
	StdoutFileName     string                 `json:"StdoutFileName"`
	StderrFileName     string                 `json:"StderrFileName"`
	SourceVersion      string                 `json:"SourceVersion"`
	SourceLocation     string                 `json:"SourceLocation"`
	SourceHash         string                 `json:"SourceHash"`
	TargetVersion      string                 `json:"TargetVersion"`
	TargetLocation     string                 `json:"TargetLocation"`
	TargetHash         string                 `json:"TargetHash"`
	PackageName        string                 `json:"PackageName"`
	StartDateTime      time.Time              `json:"StartDateTime"`
	EndDateTime        time.Time              `json:"EndDateTime"`
	MessageID          string                 `json:"MessageId"`
	UpdateRoot         string                 `json:"UpdateRoot"`
	RequiresUninstall  bool                   `json:"RequiresUninstall"`
}

// UpdateContext holds the book keeping details for Update context
// It contains current update detail and all the update histories
type UpdateContext struct {
	Current   *UpdateDetail   `json:"Current"`
	Histories []*UpdateDetail `json:"Histories"`
}

// HasMessageID represents if update is triggered by run command
func (update *UpdateDetail) HasMessageID() bool {
	return len(update.MessageID) > 0
}

// IsUpdateInProgress represents if the another update is running
func (context *UpdateContext) IsUpdateInProgress(log log.T) bool {
	//System will check the start time of the last update
	//If current system time minus start time is bigger than the MaxAllowedUpdateTime, which means update has been interrupted.
	//Allow system to resume update
	if context.Current == nil {
		return false
	} else if string(context.Current.State) == "" {
		return false
	} else {
		duration := time.Since(context.Current.StartDateTime)
		log.Infof("Attemping to retry update after %v seconds", duration.Seconds())
		if duration.Seconds() > maxAllowedUpdateDuration {
			return false
		}
	}

	return true
}

// AppendInfo appends messages to UpdateContext StandardOut
func (update *UpdateDetail) AppendInfo(log log.T, format string, params ...interface{}) {
	message := fmt.Sprintf(format, params...)
	log.Infof(message)
	if update.StandardOut != "" {
		update.StandardOut = fmt.Sprintf("%v\n%v", update.StandardOut, message)
	} else {
		update.StandardOut = message
	}
}

// AppendError appends messages to UpdateContext StandardError and StandardOut
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

// LoadUpdateContext loads update context info from local storage, set current update with new update detail
func LoadUpdateContext(log log.T, source string) (context *UpdateContext, err error) {
	log.Debugf("file %v", source)
	if _, err := os.Stat(source); os.IsNotExist(err) {
		log.Debugf("UpdateContext file doesn't exist, creating new one")
		context = &UpdateContext{}
	} else {
		log.Debugf("UpdateContext file exists")
		if context, err = parseContext(log, source); err != nil {
			return context, err
		}
	}
	return context, nil
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

// getOrchestrationDir returns the orchestration directory
func getOrchestrationDir(log log.T, update *UpdateDetail) string {
	var err error
	var instanceId string
	if instanceId, err = platform.InstanceID(); err != nil {
		log.Errorf("Cannot get instance id.")
	}
	orchestrationDir := fileutil.BuildPath(
		appconfig.DefaultDataStorePath,
		instanceId,
		appconfig.DefaultDocumentRootDirName,
		"orchestration")
	var commandID string
	if update.HasMessageID() {
		commandID, _ = getCommandID(update.MessageID)
	}

	orchestrationDirectory := fileutil.BuildPath(orchestrationDir, commandID, updateutil.DefaultOutputFolder)
	return orchestrationDirectory
}

func (context *UpdateContext) cleanUpdate() {
	context.Histories = append(context.Histories, context.Current)
	context.Current = &UpdateDetail{}
}

// saveUpdateContext save update context to local storage
func (c *contextManager) saveUpdateContext(log log.T, context *UpdateContext, contextLocation string) (err error) {
	var jsonData = []byte{}
	if jsonData, err = json.Marshal(context); err != nil {
		return err
	}

	if err = ioutil.WriteFile(
		contextLocation,
		jsonData,
		appconfig.ReadWriteAccess); err != nil {
		return err
	}
	return nil
}

// parseContext loads and parses update context from local storage
func parseContext(log log.T, fileName string) (context *UpdateContext, err error) {
	// Load specified file from file system
	result, err := ioutil.ReadFile(fileName)
	if err != nil {
		return
	}
	// parse context file
	if err = json.Unmarshal([]byte(result), &context); err != nil {
		return
	}

	return context, err
}

// uploadOutput uploads the stdout and stderr file to S3
func (c *contextManager) uploadOutput(log log.T, context *UpdateContext, orchestrationDirectory string) (err error) {

	// upload outputs (if any) to s3
	uploadOutputsToS3 := func() {
		// delete temp outputDir once we're done
		defer func() {
			if err := fileutil.DeleteDirectory(updateutil.UpdateOutputDirectory(context.Current.UpdateRoot)); err != nil {
				log.Error("error deleting directory", err)
			}
		}()

		// get stdout file path
		stdoutPath := updateutil.UpdateStdOutPath(orchestrationDirectory, context.Current.StdoutFileName)
		s3Key := path.Join(context.Current.OutputS3KeyPrefix, context.Current.StdoutFileName)
		log.Debugf("Uploading %v to s3://%v/%v", stdoutPath, context.Current.OutputS3BucketName, s3Key)
		err = s3util.NewAmazonS3Util(log, context.Current.OutputS3BucketName).S3Upload(log, context.Current.OutputS3BucketName, s3Key, stdoutPath)
		if err != nil {
			log.Errorf("failed uploading %v to s3://%v/%v \n err:%v",
				stdoutPath,
				context.Current.OutputS3BucketName,
				s3Key,
				err)
		}

		// get stderr file path
		stderrPath := updateutil.UpdateStdErrPath(orchestrationDirectory, context.Current.StderrFileName)
		s3Key = path.Join(context.Current.OutputS3KeyPrefix, context.Current.StderrFileName)
		log.Debugf("Uploading %v to s3://%v/%v", stderrPath, context.Current.OutputS3BucketName, s3Key)
		err = s3util.NewAmazonS3Util(log, context.Current.OutputS3BucketName).S3Upload(log, context.Current.OutputS3BucketName, s3Key, stderrPath)
		if err != nil {
			log.Errorf("failed uploading %v to s3://%v/%v \n err:%v", stderrPath, context.Current.StderrFileName, s3Key, err)
		}
	}

	uploadOutputsToS3()

	return nil
}
