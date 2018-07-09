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
	"strconv"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docparser"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/runcommand/contracts"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/aws/aws-sdk-go/service/ssmmds"
	"github.com/gabs"
)

// empty returns true if string is empty
func empty(s *string) bool {
	return s == nil || *s == ""
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
func newDocumentInfo(msg ssmmds.Message, parsedMsg messageContracts.SendCommandPayload) contracts.DocumentInfo {

	documentInfo := new(contracts.DocumentInfo)

	documentInfo.CommandID, _ = messageContracts.GetCommandID(*msg.MessageId)
	documentInfo.DocumentID = documentInfo.CommandID
	documentInfo.InstanceID = *msg.Destination
	documentInfo.MessageID = *msg.MessageId
	documentInfo.RunID = times.ToIsoDashUTC(times.DefaultClock.Now())
	documentInfo.CreatedDate = *msg.CreatedDate
	documentInfo.DocumentName = parsedMsg.DocumentName
	documentInfo.DocumentStatus = contracts.ResultStatusInProgress

	return *documentInfo
}

func parseCancelCommandMessage(context context.T, msg *ssmmds.Message, messagesOrchestrationRootDir string) (*contracts.DocumentState, error) {
	log := context.Log()

	log.Debug("Processing cancel command message - ", *msg.MessageId)

	var payload messageContracts.CancelPayload
	err := json.Unmarshal([]byte(*msg.Payload), &payload)
	if err != nil {
		return nil, err
	}
	var docState contracts.DocumentState
	documentInfo := contracts.DocumentInfo{}
	documentInfo.InstanceID = *msg.Destination
	documentInfo.CreatedDate = *msg.CreatedDate
	documentInfo.MessageID = *msg.MessageId
	documentInfo.CommandID, _ = messageContracts.GetCommandID(*msg.MessageId)
	documentInfo.DocumentID = documentInfo.CommandID
	documentInfo.RunID = times.ToIsoDashUTC(times.DefaultClock.Now())
	documentInfo.DocumentStatus = contracts.ResultStatusInProgress

	cancelCommand := new(contracts.CancelCommandInfo)
	cancelCommand.Payload = *msg.Payload
	cancelCommand.CancelMessageID = payload.CancelMessageID
	commandID, _ := messageContracts.GetCommandID(payload.CancelMessageID)

	cancelCommand.CancelCommandID = commandID
	cancelCommand.DebugInfo = fmt.Sprintf("Command %v is yet to be cancelled", commandID)

	var documentType contracts.DocumentType
	if strings.HasPrefix(*msg.Topic, string(CancelCommandTopicPrefixOffline)) {
		documentType = contracts.CancelCommandOffline
	} else {
		documentType = contracts.CancelCommand
	}
	docState = contracts.DocumentState{
		DocumentInformation: documentInfo,
		CancelInformation:   *cancelCommand,
		DocumentType:        documentType,
	}
	return &docState, nil
}

//generateCloudWatchLogStreamPrefix creates the LogStreamPrefix for cloudWatch output. LogStreamPrefix = <CommandID>/<InstanceID>
func generateCloudWatchLogStreamPrefix(commandID string) (string, error) {

	instanceID, err := systemInfo.InstanceID()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s", commandID, instanceID), nil
}

func generateCloudWatchConfigFromPayload(parsedMessage messageContracts.SendCommandPayload) (contracts.CloudWatchConfiguration, error) {
	cloudWatchOutputEnabled, err := strconv.ParseBool(parsedMessage.CloudWatchOutputEnabled)
	cloudWatchConfig := contracts.CloudWatchConfiguration{}
	if err != nil || !cloudWatchOutputEnabled {
		return cloudWatchConfig, err
	}
	cloudWatchConfig.LogStreamPrefix, err = generateCloudWatchLogStreamPrefix(parsedMessage.CommandID)
	if err != nil {
		return cloudWatchConfig, err
	}
	if parsedMessage.CloudWatchLogGroupName != "" {
		cloudWatchConfig.LogGroupName = parsedMessage.CloudWatchLogGroupName
	} else {
		cloudWatchConfig.LogGroupName = fmt.Sprintf("%s%s", CloudWatchLogGroupNamePrefix, parsedMessage.DocumentName)
	}
	return cloudWatchConfig, nil
}

func parseSendCommandMessage(context context.T, msg *ssmmds.Message, messagesOrchestrationRootDir string) (*contracts.DocumentState, error) {
	log := context.Log()
	commandID, _ := messageContracts.GetCommandID(*msg.MessageId)

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

	cloudWatchConfig, err := generateCloudWatchConfigFromPayload(parsedMessage)
	if err != nil {
		log.Errorf("Encountered error while generating cloudWatch config from send command payload, err: %s", err)
	}

	messageOrchestrationDirectory := filepath.Join(messagesOrchestrationRootDir, commandID)

	var documentType contracts.DocumentType
	if strings.HasPrefix(*msg.Topic, string(SendCommandTopicPrefixOffline)) {
		documentType = contracts.SendCommandOffline
	} else {
		documentType = contracts.SendCommand
	}
	documentInfo := newDocumentInfo(*msg, parsedMessage)
	parserInfo := docparser.DocumentParserInfo{
		OrchestrationDir: messageOrchestrationDirectory,
		S3Bucket:         parsedMessage.OutputS3BucketName,
		S3Prefix:         s3KeyPrefix,
		MessageId:        documentInfo.MessageID,
		DocumentId:       documentInfo.DocumentID,
		CloudWatchConfig: cloudWatchConfig,
	}

	docContent := &docparser.DocContent{
		SchemaVersion: parsedMessage.DocumentContent.SchemaVersion,
		Description:   parsedMessage.DocumentContent.Description,
		RuntimeConfig: parsedMessage.DocumentContent.RuntimeConfig,
		MainSteps:     parsedMessage.DocumentContent.MainSteps,
		Parameters:    parsedMessage.DocumentContent.Parameters}
	//Data format persisted in Current Folder is defined by the struct - CommandState
	docState, err := docparser.InitializeDocState(log, documentType, docContent, documentInfo, parserInfo, parsedMessage.Parameters)
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

	if isMI && contracts.IsManagedInstanceIncompatibleAWSSSMDocument(docState.DocumentInformation.DocumentName) {
		log.Debugf("Running incompatible AWS SSM Document %v on managed instance", docState.DocumentInformation.DocumentName)
		if err = contracts.RemoveDependencyOnInstanceMetadata(context, &docState); err != nil {
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
