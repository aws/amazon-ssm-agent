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

// Package runcommand implements runcommand core processing module
package runcommand

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/docparser"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/framework/service/runcommand/contracts"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/aws/aws-sdk-go/service/ssmmds"
	"github.com/gabs"
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
func newDocumentInfo(msg ssmmds.Message, parsedMsg messageContracts.SendCommandPayload) model.DocumentInfo {

	documentInfo := new(model.DocumentInfo)

	documentInfo.CommandID = getCommandID(*msg.MessageId)
	documentInfo.DocumentID = documentInfo.CommandID
	documentInfo.InstanceID = *msg.Destination
	documentInfo.MessageID = *msg.MessageId
	documentInfo.RunID = times.ToIsoDashUTC(times.DefaultClock.Now())
	documentInfo.CreatedDate = *msg.CreatedDate
	documentInfo.DocumentName = parsedMsg.DocumentName
	documentInfo.IsCommand = true
	documentInfo.DocumentStatus = contracts.ResultStatusInProgress
	documentInfo.DocumentTraceOutput = ""

	return *documentInfo
}

func parseCancelCommandMessage(context context.T, msg *ssmmds.Message, messagesOrchestrationRootDir string) (*model.DocumentState, error) {
	log := context.Log()

	log.Debug("Processing cancel command message - ", *msg.MessageId)

	var payload messageContracts.CancelPayload
	err := json.Unmarshal([]byte(*msg.Payload), &payload)
	if err != nil {
		return nil, err
	}
	var docState model.DocumentState
	documentInfo := model.DocumentInfo{}
	documentInfo.InstanceID = *msg.Destination
	documentInfo.CreatedDate = *msg.CreatedDate
	documentInfo.MessageID = *msg.MessageId
	documentInfo.CommandID = getCommandID(*msg.MessageId)
	documentInfo.DocumentID = documentInfo.CommandID
	documentInfo.RunID = times.ToIsoDashUTC(times.DefaultClock.Now())
	documentInfo.DocumentStatus = contracts.ResultStatusInProgress

	cancelCommand := new(model.CancelCommandInfo)
	cancelCommand.Payload = *msg.Payload
	cancelCommand.CancelMessageID = payload.CancelMessageID
	commandID := getCommandID(payload.CancelMessageID)

	cancelCommand.CancelCommandID = commandID
	cancelCommand.DebugInfo = fmt.Sprintf("Command %v is yet to be cancelled", commandID)

	var documentType model.DocumentType
	if strings.HasPrefix(*msg.Topic, string(CancelCommandTopicPrefixOffline)) {
		documentType = model.CancelCommandOffline
	} else {
		documentType = model.CancelCommand
	}
	docState = model.DocumentState{
		DocumentInformation: documentInfo,
		CancelInformation:   *cancelCommand,
		DocumentType:        documentType,
	}
	return &docState, nil
}

func parseSendCommandMessage(context context.T, msg *ssmmds.Message, messagesOrchestrationRootDir string) (*model.DocumentState, error) {
	log := context.Log()
	commandID := getCommandID(*msg.MessageId)

	log.Debug("Processing send command message ", *msg.MessageId)
	log.Trace("Processing send command message ", jsonutil.Indent(*msg.Payload))

	// parse message to retrieve parameters
	var parsedMessage messageContracts.SendCommandPayload
	err := json.Unmarshal([]byte(*msg.Payload), &parsedMessage)
	if err != nil {
		errorMsg := "Encountered error while parsing input - internal error"
		log.Errorf(errorMsg)
		return nil, fmt.Errorf("%v", errorMsg)
	}

	// adapt plugin configuration format from MDS to plugin expected format
	s3KeyPrefix := path.Join(parsedMessage.OutputS3KeyPrefix, parsedMessage.CommandID, *msg.Destination)

	messageOrchestrationDirectory := filepath.Join(messagesOrchestrationRootDir, commandID)

	var documentType model.DocumentType
	if strings.HasPrefix(*msg.Topic, string(SendCommandTopicPrefixOffline)) {
		documentType = model.SendCommandOffline
	} else {
		documentType = model.SendCommand
	}
	documentInfo := newDocumentInfo(*msg, parsedMessage)
	parserInfo := docparser.DocumentParserInfo{
		OrchestrationDir: messageOrchestrationDirectory,
		S3Bucket:         parsedMessage.OutputS3BucketName,
		S3Prefix:         s3KeyPrefix,
		MessageId:        documentInfo.MessageID,
		DocumentId:       documentInfo.DocumentID,
	}

	//Data format persisted in Current Folder is defined by the struct - CommandState
	docState, err := docparser.InitializeDocState(log, documentType, &parsedMessage.DocumentContent, documentInfo, parserInfo, parsedMessage.Parameters)
	if err != nil {
		return nil, err
	}
	parsedMessageContent, _ := jsonutil.Marshal(parsedMessage)

	var parsedContentJson *gabs.Container

	if parsedContentJson, err = gabs.ParseJSON([]byte(parsedMessageContent)); err != nil {
		log.Debugf("Parsed message is in the wrong json format. Error is ", err)
	}
	//Search for "DocumentContent" > "runtimeConfig" > "aws:cloudWatch" > "properties" which has the cloudwatch
	// config file and scrub the credentials, if present
	obj := parsedContentJson.Search(documentContent, runtimeConfig, cloudwatchPlugin, properties).String()
	if obj != "{}" {
		//This will be true only for aws:cloudwatch
		stripConfig := strings.Replace(strings.Replace(strings.Replace(obj, "\\t", "", -1), "\\n", "", -1), "\\", "", -1)
		stripConfig = strings.TrimSuffix(strings.TrimPrefix(stripConfig, "\""), "\"")

		finalLogConfig := logger.PrintCWConfig(stripConfig, log)

		// Parameters > properties is another path where the config file is printed
		if _, err = parsedContentJson.Set(finalLogConfig, parameters, properties); err != nil {
			log.Debug("Error occurred when setting Parameters->properties with scrubbed credentials - ", err)
		}
		if _, err = parsedContentJson.Set(finalLogConfig, documentContent, runtimeConfig, cloudwatchPlugin, properties); err != nil {
			log.Debug("Error occurred when setting aws:cloudWatch->properties with scrubbed credentials - ", err)
		}
		log.Debug("ParsedMessage is ", parsedContentJson.StringIndent("", "  "))
	} else {
		//For plugins that are not aws:cloudwatch
		log.Debug("ParsedMessage is ", jsonutil.Indent(parsedMessageContent))
	}
	// Check if it is a managed instance and its executing managed instance incompatible AWS SSM public document.
	// A few public AWS SSM documents contain code which is not compatible when run on managed instances.
	// isManagedInstanceIncompatibleAWSSSMDocument makes sure to find such documents at runtime and replace the incompatible code.
	isMI, err := platform.IsManagedInstance()
	if err != nil {
		log.Errorf("Error determining managed instance. error: %v", err)
	}

	if isMI && model.IsManagedInstanceIncompatibleAWSSSMDocument(docState.DocumentInformation.DocumentName) {
		log.Debugf("Running incompatible AWS SSM Document %v on managed instance", docState.DocumentInformation.DocumentName)
		if err = model.RemoveDependencyOnInstanceMetadata(context, &docState); err != nil {
			return nil, err
		}
	}

	return &docState, nil
}

func isUpdatePlugin(plugins map[string]*contracts.PluginResult) bool {
	for name, _ := range plugins {
		if name == appconfig.PluginEC2ConfigUpdate || name == appconfig.PluginNameAwsAgentUpdate {
			return true
		}
	}
	return false
}
