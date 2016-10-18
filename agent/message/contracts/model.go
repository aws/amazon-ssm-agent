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

import "github.com/aws/amazon-ssm-agent/agent/contracts"

// CancelPayload represents the json structure of a cancel command MDS message payload.
type CancelPayload struct {
	CancelMessageID string `json:"CancelMessageId"`
}

// SendCommandPayload parallels the structure of a send command MDS message payload.
type SendCommandPayload struct {
	Parameters         map[string]interface{}    `json:"Parameters"`
	DocumentContent    contracts.DocumentContent `json:"DocumentContent"`
	CommandID          string                    `json:"CommandId"`
	DocumentName       string                    `json:"DocumentName"`
	OutputS3KeyPrefix  string                    `json:"OutputS3KeyPrefix"`
	OutputS3BucketName string                    `json:"OutputS3BucketName"`
}

// SendReplyPayload represents the json structure of a reply sent to MDS.
type SendReplyPayload struct {
	AdditionalInfo      contracts.AdditionalInfo                  `json:"additionalInfo"`
	DocumentStatus      contracts.ResultStatus                    `json:"documentStatus"`
	DocumentTraceOutput string                                    `json:"documentTraceOutput"`
	RuntimeStatus       map[string]*contracts.PluginRuntimeStatus `json:"runtimeStatus"`
}
