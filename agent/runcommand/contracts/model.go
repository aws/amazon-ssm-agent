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

// Package model contains message struct for MDS/SSM messages.
package model

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
)

// CancelPayload represents the json structure of a cancel command MDS message payload.
type CancelPayload struct {
	CancelMessageID string `json:"CancelMessageId"`
}

// SendCommandPayload parallels the structure of a send command MDS message payload.
type SendCommandPayload struct {
	Parameters              map[string]interface{}    `json:"Parameters"`
	DocumentContent         contracts.DocumentContent `json:"DocumentContent"`
	CommandID               string                    `json:"CommandId"`
	DocumentName            string                    `json:"DocumentName"`
	OutputS3KeyPrefix       string                    `json:"OutputS3KeyPrefix"`
	OutputS3BucketName      string                    `json:"OutputS3BucketName"`
	CloudWatchLogGroupName  string                    `json:"CloudWatchLogGroupName"`
	CloudWatchOutputEnabled string                    `json:"CloudWatchOutputEnabled"`
}

// SendReplyPayload represents the json structure of a reply sent to MDS.
type SendReplyPayload struct {
	AdditionalInfo      contracts.AdditionalInfo                  `json:"additionalInfo"`
	DocumentStatus      contracts.ResultStatus                    `json:"documentStatus"`
	DocumentTraceOutput string                                    `json:"documentTraceOutput"`
	RuntimeStatus       map[string]*contracts.PluginRuntimeStatus `json:"runtimeStatus"`
}

//getCommandID gets CommandID from given MessageID
func getCommandID(messageID string) string {
	// MdsMessageID is in the format of : aws.ssm.CommandId.InstanceId
	// E.g (aws.ssm.2b196342-d7d4-436e-8f09-3883a1116ac3.i-57c0a7be)
	mdsMessageIDSplit := strings.Split(messageID, ".")
	return mdsMessageIDSplit[len(mdsMessageIDSplit)-2]
}

func GetCommandID(messageID string) (string, error) {
	//messageID format: E.g (aws.ssm.2b196342-d7d4-436e-8f09-3883a1116ac3.i-57c0a7be)
	if match, err := regexp.MatchString("aws\\.ssm\\..+\\.+", messageID); !match {
		return messageID, fmt.Errorf("invalid messageID format: %v | %v", messageID, err)
	}

	return getCommandID(messageID), nil
}
