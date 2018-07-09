// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package testutils represents the common logic needed for agent tests
package testutils

import (
	"crypto/sha256"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/runcommand/contracts"
	mdsmock "github.com/aws/amazon-ssm-agent/agent/runcommand/mock"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssmmds"
	"github.com/stretchr/testify/mock"
	"github.com/twinj/uuid"
)

func NewMDSMock() *mdsmock.MockedMDS {
	mdsService := new(mdsmock.MockedMDS)

	replies := []string{}
	mdsService.On("LoadFailedReplies", mock.AnythingOfType("*log.Wrapper")).Return(replies)
	mdsService.On("AcknowledgeMessage", mock.Anything, mock.AnythingOfType("string")).Return(nil)
	mdsService.On("Stop").Return()
	return mdsService
}

func GenerateEmptyMessage() (*ssmmds.GetMessagesOutput, error) {
	instanceID, _ := platform.InstanceID()
	uuid.SwitchFormat(uuid.CleanHyphen)
	var testMessageId = uuid.NewV4().String()
	msgs := make([]*ssmmds.Message, 0)
	messagesOutput := ssmmds.GetMessagesOutput{
		Destination:       &instanceID,
		Messages:          msgs,
		MessagesRequestId: &testMessageId,
	}

	return &messagesOutput, nil
}

func GenerateMessages(messageContent string) (*ssmmds.GetMessagesOutput, error) {
	uuid.SwitchFormat(uuid.CleanHyphen)
	instanceID, _ := platform.InstanceID()
	// mock GetMessagesOutput to return one message
	var testMessageId = uuid.NewV4().String()
	msgs := make([]*ssmmds.Message, 1)
	mdsMessage, err := createMDSMessage(messageContent, instanceID)
	msgs[0] = mdsMessage
	messagesOutput := ssmmds.GetMessagesOutput{
		Destination:       &instanceID,
		Messages:          msgs,
		MessagesRequestId: &testMessageId,
	}

	return &messagesOutput, err
}

func createMDSMessage(messageContent string, instanceID string) (*ssmmds.Message, error) {
	// load message payload and create MDS message from it
	var err error

	var payload messageContracts.SendCommandPayload
	err = jsonutil.Unmarshal(messageContent, &payload)
	if err != nil {
		return nil, err
	}
	uuid.SwitchFormat(uuid.CleanHyphen)
	payload.CommandID = uuid.NewV4().String()
	msgContent, err := jsonutil.Marshal(payload)
	if err != nil {
		return nil, err
	}

	messageCreatedDate := time.Date(2015, 7, 9, 23, 22, 39, 19000000, time.UTC)

	c := sha256.New()
	c.Write([]byte(msgContent))
	payloadDigest := string(c.Sum(nil))

	msg := ssmmds.Message{
		CreatedDate:   aws.String(times.ToIso8601UTC(messageCreatedDate)),
		Destination:   aws.String(instanceID),
		MessageId:     aws.String("aws.ssm." + payload.CommandID + "." + instanceID),
		Payload:       aws.String(msgContent),
		PayloadDigest: aws.String(payloadDigest),
		Topic:         aws.String("aws.ssm.sendCommand.us.east.1.1"),
	}
	return &msg, err
}
