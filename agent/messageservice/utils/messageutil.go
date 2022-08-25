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
// either express or implied. See the License for the specific language governing`
// permissions and limitations under the License.

// Package utils provides utility functions to be used by interactors
package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Jeffail/gabs"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/docparser"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	model "github.com/aws/amazon-ssm-agent/agent/messageservice/contracts"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/runcommand/contracts"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/aws/aws-sdk-go/service/ssmmds"
)

// TopicPrefix is the prefix of the Topic field in an MDS message.
type TopicPrefix string

const (
	// CloudWatchLogGroupNamePrefix CloudWatch output's log group name prefix
	CloudWatchLogGroupNamePrefix = "/aws/ssm/"

	documentContent  = "DocumentContent"
	runtimeConfig    = "runtimeConfig"
	cloudwatchPlugin = "aws:cloudWatch"
	properties       = "properties"
	parameters       = "Parameters"

	// MDS service will mark document as timeout if it didn't receive any response from the agent after 2 hours
	documentLevelTimeOutDurationHour = 2

	// SendCommandTopicPrefix is the topic prefix for a send command MDS message.
	SendCommandTopicPrefix TopicPrefix = "aws.ssm.sendCommand"

	// CancelCommandTopicPrefix is the topic prefix for a cancel command MDS message.
	CancelCommandTopicPrefix TopicPrefix = "aws.ssm.cancelCommand"

	// SendFailedReplyFrequencyMinutes is the frequency at which to send failed reply requests back to MDS
	SendFailedReplyFrequencyMinutes = 5
)

// empty returns true if string is empty
func empty(s *string) bool {
	return s == nil || *s == ""
}

// Validate returns error if the message is invalid
func Validate(msg *ssmmds.Message) error {
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
func newDocumentInfo(msg model.InstanceMessage, parsedMsg messageContracts.SendCommandPayload) contracts.DocumentInfo {

	documentInfo := new(contracts.DocumentInfo)

	documentInfo.CommandID, _ = messageContracts.GetCommandID(msg.MessageId)
	documentInfo.DocumentID = documentInfo.CommandID
	documentInfo.InstanceID = msg.Destination
	documentInfo.MessageID = msg.MessageId
	documentInfo.RunID = times.ToIsoDashUTC(times.DefaultClock.Now())
	documentInfo.CreatedDate = msg.CreatedDate
	documentInfo.DocumentName = parsedMsg.DocumentName
	documentInfo.DocumentStatus = contracts.ResultStatusInProgress

	return *documentInfo
}

// ParseCancelCommandMessage parses send command message
func ParseCancelCommandMessage(context context.T, msg model.InstanceMessage, upstreamService contracts.UpstreamServiceName) (*contracts.DocumentState, error) {
	log := context.Log()

	log.Debug("Processing cancel command message - ", msg.MessageId)

	var payload messageContracts.CancelPayload
	err := json.Unmarshal([]byte(msg.Payload), &payload)
	if err != nil {
		return nil, err
	}

	commandID, _ := messageContracts.GetCommandID(msg.MessageId)
	var docState contracts.DocumentState
	documentInfo := contracts.DocumentInfo{
		DocumentID:     commandID,
		CommandID:      commandID,
		InstanceID:     msg.Destination,
		MessageID:      msg.MessageId,
		RunID:          times.ToIsoDashUTC(times.DefaultClock.Now()),
		CreatedDate:    msg.CreatedDate,
		DocumentStatus: contracts.ResultStatusInProgress,
	}

	cancelCommand := new(contracts.CancelCommandInfo)
	cancelCommand.Payload = msg.Payload
	cancelCommand.CancelMessageID = payload.CancelMessageID
	cancelCommandID, _ := messageContracts.GetCommandID(payload.CancelMessageID)

	cancelCommand.CancelCommandID = cancelCommandID
	cancelCommand.DebugInfo = fmt.Sprintf("Command %v is yet to be cancelled", commandID)

	docState = contracts.DocumentState{
		DocumentInformation: documentInfo,
		CancelInformation:   *cancelCommand,
		DocumentType:        contracts.CancelCommand,
		UpstreamServiceName: upstreamService,
	}
	return &docState, nil
}

// generateCloudWatchLogStreamPrefix creates the LogStreamPrefix for cloudWatch output. LogStreamPrefix = <CommandID>/<InstanceID>
func generateCloudWatchLogStreamPrefix(context context.T, commandID string) (string, error) {

	instanceID, err := context.Identity().ShortInstanceID()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s", commandID, instanceID), nil
}

func generateCloudWatchConfigFromPayload(context context.T, parsedMessage messageContracts.SendCommandPayload) (contracts.CloudWatchConfiguration, error) {
	cloudWatchOutputEnabled, err := strconv.ParseBool(parsedMessage.CloudWatchOutputEnabled)
	cloudWatchConfig := contracts.CloudWatchConfiguration{}
	if err != nil || !cloudWatchOutputEnabled {
		return cloudWatchConfig, err
	}
	cloudWatchConfig.LogStreamPrefix, err = generateCloudWatchLogStreamPrefix(context, parsedMessage.CommandID)
	if err != nil {
		return cloudWatchConfig, err
	}
	if parsedMessage.CloudWatchLogGroupName != "" {
		cloudWatchConfig.LogGroupName = parsedMessage.CloudWatchLogGroupName
	} else {
		logGroupName := fmt.Sprintf("%s%s", CloudWatchLogGroupNamePrefix, parsedMessage.DocumentName)
		cloudWatchConfig.LogGroupName = cleanupLogGroupName(logGroupName)
	}
	return cloudWatchConfig, nil
}

func cleanupLogGroupName(logGroupName string) string {
	// log group pattern referred from below URL
	// https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_CreateLogGroup.html
	if reg, err := regexp.Compile(`[^a-zA-Z0-9_\-/\.#]`); reg != nil && err == nil {
		// replace invalid chars with dot(.)
		return reg.ReplaceAllString(logGroupName, ".")
	}
	return logGroupName
}

// ParseSendCommandMessage parses send command message
func ParseSendCommandMessage(context context.T, msg model.InstanceMessage, messagesOrchestrationRootDir string, upstreamService contracts.UpstreamServiceName) (*contracts.DocumentState, error) {
	log := context.Log()
	commandID, _ := messageContracts.GetCommandID(msg.MessageId)

	log.Debug("Processing send command message ", msg.MessageId)
	log.Trace("Processing send command payload:  ", jsonutil.Indent(msg.Payload))

	// parse message to retrieve parameters
	var parsedMessage messageContracts.SendCommandPayload
	err := json.Unmarshal([]byte(msg.Payload), &parsedMessage)
	if err != nil {
		errorMsg := fmt.Errorf("encountered error while parsing input - internal error %v", err)
		log.Error(errorMsg)
		return nil, errorMsg
	}

	// adapt plugin configuration format from MDS to plugin expected format
	s3KeyPrefix := path.Join(parsedMessage.OutputS3KeyPrefix, parsedMessage.CommandID, msg.Destination)

	cloudWatchConfig, err := generateCloudWatchConfigFromPayload(context, parsedMessage)
	if err != nil {
		log.Errorf("encountered error while generating cloudWatch config from send command payload, err: %s", err)
	}

	messageOrchestrationDirectory := filepath.Join(messagesOrchestrationRootDir, commandID)

	documentType := contracts.SendCommand
	documentInfo := newDocumentInfo(msg, parsedMessage)
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
	docState, err := docparser.InitializeDocState(context, documentType, docContent, documentInfo, parserInfo, parsedMessage.Parameters)
	if err != nil {
		return nil, err
	}
	docState.UpstreamServiceName = upstreamService
	parsedMessageContent, _ := jsonutil.Marshal(parsedMessage)

	var parsedContentJson *gabs.Container

	if parsedContentJson, err = gabs.ParseJSON([]byte(parsedMessageContent)); err != nil {
		log.Debugf("Parsed message is in the wrong json format. Error is ", err)
	}
	// Search for "DocumentContent" > "runtimeConfig" > "aws:cloudWatch" > "properties" which has the cloudwatch
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

	// Check if it is a managed instance and is executing managed instance incompatible AWS SSM public document.
	// A few public AWS SSM documents contain code which is not compatible when run on managed instances.
	// isManagedInstanceIncompatibleAWSSSMDocument makes sure to find such documents at runtime and replace the incompatible code.
	isMI := identity.IsOnPremInstance(context.Identity())

	if isMI && contracts.IsManagedInstanceIncompatibleAWSSSMDocument(docState.DocumentInformation.DocumentName) {
		log.Debugf("Running incompatible AWS SSM Document %v on managed instance", docState.DocumentInformation.DocumentName)
		if err = contracts.RemoveDependencyOnInstanceMetadata(context, &docState); err != nil {
			return nil, err
		}
	}

	return &docState, nil
}

// IsValidReplyRequest checks whether the reply is valid and had timed or not
func IsValidReplyRequest(filename string, name contracts.UpstreamServiceName) bool {
	splitFileName := strings.Split(filename, "_")
	if len(splitFileName) < 2 {
		return false
	}
	timeInFileName := ""
	if name == contracts.MessageGatewayService { // MGS uses this format to have proper time based sorting
		timeInFileName = splitFileName[0]
	} else {
		timeInFileName = splitFileName[1]
	}
	t, _ := time.Parse("2006-01-02T15-04-05", timeInFileName)
	curTime := time.Now().UTC()
	delta := curTime.Sub(t).Hours()
	if delta > documentLevelTimeOutDurationHour {
		return false
	} else {
		return true
	}
}

// PrepareReplyPayloadToUpdateDocumentStatus creates the payload object for SendReply based on document status change.
func PrepareReplyPayloadToUpdateDocumentStatus(agentInfo contracts.AgentInfo, documentStatus contracts.ResultStatus, documentTraceOutput string) (payload messageContracts.SendReplyPayload) {
	payload = messageContracts.SendReplyPayload{
		AdditionalInfo: contracts.AdditionalInfo{
			Agent:    agentInfo,
			DateTime: times.ToIso8601UTC(times.DefaultClock.Now()),
		},
		DocumentStatus:      documentStatus,
		DocumentTraceOutput: documentTraceOutput,
		RuntimeStatus:       nil,
	}
	return
}

// PrepareReplyPayloadFromIntermediatePluginResults parses send reply payload
func PrepareReplyPayloadFromIntermediatePluginResults(log logger.T, pluginID string, agentInfo contracts.AgentInfo, outputs map[string]*contracts.PluginResult) (payload messageContracts.SendReplyPayload) {
	status, statusCount, runtimeStatuses, _ := contracts.DocumentResultAggregator(log, pluginID, outputs)
	additionalInfo := contracts.AdditionalInfo{
		Agent:               agentInfo,
		DateTime:            times.ToIso8601UTC(time.Now()),
		RuntimeStatusCounts: statusCount,
	}
	payload = messageContracts.SendReplyPayload{
		AdditionalInfo:      additionalInfo,
		DocumentStatus:      status,
		DocumentTraceOutput: "", // TODO: Fill me appropriately
		RuntimeStatus:       runtimeStatuses,
	}
	return
}
