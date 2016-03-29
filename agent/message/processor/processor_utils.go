// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package processor implements MDS plugin processor
// processor_utils contains utility functions
package processor

import (
	"errors"
	"path"
	"path/filepath"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/message/contracts"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/aws/aws-sdk-go/service/ssmmds"
)

// empty returns true if string is empty
func empty(s *string) bool {
	return s == nil || *s == ""
}

//getCommandID gets CommandID from given MessageID
func getCommandID(messageID string) string {
	// MdsMessageID is in the format of : aws.ssm.CommandId.InstanceId
	// E.g (aws.ssm.2b196342-d7d4-436e-8f09-3883a1116ac3.i-57c0a7be)
	mdsMessageIDSplit := strings.Split(messageID, ".")
	return mdsMessageIDSplit[len(mdsMessageIDSplit)-2]
}

// validate returns error if the message is invalid
func validate(msg *ssmmds.Message) error {
	if msg == nil {
		return errors.New("Message is nil")
	}
	if empty(msg.Topic) {
		return errors.New("Topic is missing")
	}
	if empty(msg.Destination) {
		return errors.New("Destination is missing")
	}
	if empty(msg.MessageId) {
		return errors.New("MessageId is missing")
	}
	if empty(msg.CreatedDate) {
		return errors.New("CreatedDate is missing")
	}
	return nil
}

// newDocumentInfo initializes new DocumentInfo object
func newDocumentInfo(msg ssmmds.Message, parsedMsg messageContracts.SendCommandPayload) messageContracts.DocumentInfo {

	documentInfo := new(messageContracts.DocumentInfo)

	documentInfo.CommandID = getCommandID(*msg.MessageId)
	documentInfo.Destination = *msg.Destination
	documentInfo.MessageID = *msg.MessageId
	documentInfo.RunID = times.ToIsoDashUTC(times.DefaultClock.Now())
	documentInfo.CreatedDate = *msg.CreatedDate
	documentInfo.DocumentName = parsedMsg.DocumentName
	documentInfo.IsCommand = true
	documentInfo.DocumentStatus = contracts.ResultStatusInProgress
	documentInfo.DocumentTraceOutput = ""

	return *documentInfo
}

// getPluginConfigurations converts from PluginConfig (structure from the MDS message) to plugin.Configuration (structure expected by the plugin)
func getPluginConfigurations(runtimeConfig map[string]*contracts.PluginConfig, orchestrationDir, s3BucketName, s3KeyPrefix, messageID string) (res map[string]*contracts.Configuration) {
	res = make(map[string]*contracts.Configuration)
	for pluginName, pluginConfig := range runtimeConfig {
		res[pluginName] = &contracts.Configuration{
			Properties:             pluginConfig.Properties,
			OutputS3BucketName:     s3BucketName,
			OutputS3KeyPrefix:      path.Join(s3KeyPrefix, fileutil.RemoveInvalidChars(pluginName)),
			OrchestrationDirectory: fileutil.RemoveInvalidChars(filepath.Join(orchestrationDir, pluginName)),
			MessageId:              messageID,
			BookKeepingFileName:    getCommandID(messageID),
		}
	}
	return
}
