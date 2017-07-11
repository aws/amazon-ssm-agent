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

// Package processor implements MDS plugin processor
// processor_state contains utilities that interact with the state manager
package processor

import (
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	docModel "github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/message/contracts"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/aws/aws-sdk-go/service/ssmmds"

	"fmt"
	"strings"
)

// initializes CancelCommandState - an interim state that is used during a command cancelling
func initializeCancelCommandState(msg ssmmds.Message, parsedMsg messageContracts.CancelPayload) docModel.DocumentState {
	documentInfo := docModel.DocumentInfo{}
	documentInfo.InstanceID = *msg.Destination
	documentInfo.CreatedDate = *msg.CreatedDate
	documentInfo.MessageID = *msg.MessageId
	documentInfo.CommandID = getCommandID(*msg.MessageId)
	documentInfo.DocumentID = documentInfo.CommandID
	documentInfo.RunID = times.ToIsoDashUTC(times.DefaultClock.Now())
	documentInfo.DocumentStatus = contracts.ResultStatusInProgress

	cancelCommand := new(docModel.CancelCommandInfo)
	cancelCommand.Payload = *msg.Payload
	cancelCommand.CancelMessageID = parsedMsg.CancelMessageID
	commandID := getCommandID(parsedMsg.CancelMessageID)

	cancelCommand.CancelCommandID = commandID
	cancelCommand.DebugInfo = fmt.Sprintf("Command %v is yet to be cancelled", commandID)

	var documentType docModel.DocumentType
	if strings.HasPrefix(*msg.Topic, string(CancelCommandTopicPrefixOffline)) {
		documentType = docModel.CancelCommandOffline
	} else {
		documentType = docModel.CancelCommand
	}
	return docModel.DocumentState{
		DocumentInformation: documentInfo,
		CancelInformation:   *cancelCommand,
		DocumentType:        documentType,
	}
}
