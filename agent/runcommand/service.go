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
	"path/filepath"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	associationProcessor "github.com/aws/amazon-ssm-agent/agent/association/processor"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/runcommand/contracts"
	mdsService "github.com/aws/amazon-ssm-agent/agent/runcommand/mds"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/carlescere/scheduler"
)

// TopicPrefix is the prefix of the Topic field in an MDS message.
type TopicPrefix string

const (
	// SendCommandTopicPrefix is the topic prefix for a send command MDS message.
	SendCommandTopicPrefix TopicPrefix = "aws.ssm.sendCommand."

	// CancelCommandTopicPrefix is the topic prefix for a cancel command MDS message.
	CancelCommandTopicPrefix TopicPrefix = "aws.ssm.cancelCommand."

	// SendCommandTopicPrefixOffline is the topic prefix for a send command MDS message received from the offline service.
	SendCommandTopicPrefixOffline TopicPrefix = "aws.ssm.sendCommand.offline."

	// CancelCommandTopicPrefix is the topic prefix for a cancel command MDS message received from the offline service.
	CancelCommandTopicPrefixOffline TopicPrefix = "aws.ssm.cancelCommand.offline."

	CancelWorkersLimit = 3

	// mdsname is the core module name for the MDS processor
	mdsName = "MessagingDeliveryService"

	// offlinename is the core module name for the offline command document processor
	offlineName = "OfflineService"

	// pollMessageFrequencyMinutes is the frequency at which to resume poll for messages if the current thread dies due to stop policy
	// note: the connection timeout for MDSPoll should be less than this.
	pollMessageFrequencyMinutes = 15

	// sendReplyFrequencyMinutes is the frequency at which to send failed reply requests back to MDS
	sendReplyFrequencyMinutes = 10

	// the default stoppolicy error threshold. After 10 consecutive errors the plugin will stop for 15 minutes.
	stopPolicyErrorThreshold = 10

	// CloudWatch output's log group name prefix
	CloudWatchLogGroupNamePrefix = "/aws/ssm/"
)

type persistData func(state *contracts.DocumentState, bookkeeping string)

type ExecuterCreator func(ctx context.T) executer.Executer

// SendDocumentLevelResponse is used to send status response before plugin begins
type SendDocumentLevelResponse func(messageID string, resultStatus contracts.ResultStatus, documentTraceOutput string)
type SendResponse func(messageID string, res contracts.DocumentResult)

// Processor is an object that can process MDS messages.
type RunCommandService struct {
	context              context.T
	name                 string
	config               contracts.AgentConfiguration
	service              mdsService.Service
	sendDocLevelResponse SendDocumentLevelResponse
	sendResponse         SendResponse
	orchestrationRootDir string
	messagePollJob       *scheduler.Job
	sendReplyJob         *scheduler.Job
	//TODO move association poller out, we surely have to
	assocProcessor      *associationProcessor.Processor
	processorStopPolicy *sdkutil.StopPolicy
	pollAssociations    bool
	processor           processor.Processor
}

// NewOfflineProcessor initialize a new offline command document processor
func NewOfflineService(context context.T) (*RunCommandService, error) {
	messageContext := context.With("[" + offlineName + "]")
	log := messageContext.Log()

	log.Debug("Creating offline command document service")
	offlineService, err := newOfflineService(log)
	if err != nil {
		return nil, err
	}

	return NewService(messageContext, offlineName, offlineService, 1, 1, false, []contracts.DocumentType{contracts.SendCommandOffline, contracts.CancelCommandOffline}), nil
}

// NewMdsProcessor initializes a new mds processor with the given parameters.
func NewMDSService(context context.T) *RunCommandService {
	messageContext := context.With("[" + mdsName + "]")
	mdsService := newMdsService(context.AppConfig())
	config := context.AppConfig()

	return NewService(messageContext, mdsName, mdsService, config.Mds.CommandWorkersLimit, CancelWorkersLimit, true, []contracts.DocumentType{contracts.SendCommand, contracts.CancelCommand})
}

// NewProcessor performs common initialization for Mds and Offline processors
func NewService(ctx context.T, serviceName string, service mdsService.Service, commandWorkerLimit int, cancelWorkerLimit int, pollAssoc bool, supportedDocs []contracts.DocumentType) *RunCommandService {
	log := ctx.Log()
	config := ctx.AppConfig()

	instanceID, err := platform.InstanceID()
	if instanceID == "" {
		log.Errorf("no instanceID provided, %v", err)
		return nil
	}

	agentInfo := contracts.AgentInfo{
		Lang:      config.Os.Lang,
		Name:      config.Agent.Name,
		Version:   config.Agent.Version,
		Os:        config.Os.Name,
		OsVersion: config.Os.Version,
	}

	agentConfig := contracts.AgentConfiguration{
		AgentInfo:  agentInfo,
		InstanceID: instanceID,
	}

	// create new message processor
	orchestrationRootDir := filepath.Join(appconfig.DefaultDataStorePath, instanceID, appconfig.DefaultDocumentRootDirName, config.Agent.OrchestrationRootDir)

	// create a stop policy where we will stop after 10 consecutive errors and if time period expires.
	stopPolicy := newStopPolicy(serviceName)

	// SendDocLevelResponse is used to send document level update
	// Specify a new status of the document
	sendDocLevelResponse := func(messageID string, resultStatus contracts.ResultStatus, documentTraceOutput string) {
		payloadDoc := prepareReplyPayloadToUpdateDocumentStatus(agentInfo, resultStatus, documentTraceOutput)
		processSendReply(log, messageID, service, payloadDoc, stopPolicy)
	}

	sendResponse := func(messageID string, res contracts.DocumentResult) {
		pluginID := res.LastPlugin
		processSendReply(log, messageID, service, FormatPayload(log, pluginID, agentInfo, res.PluginResults), stopPolicy)
	}

	var assocProc *associationProcessor.Processor
	if pollAssoc {
		assocProc = associationProcessor.NewAssociationProcessor(ctx)
	}

	processor := processor.NewEngineProcessor(ctx, commandWorkerLimit, cancelWorkerLimit, supportedDocs)
	return &RunCommandService{
		context:              ctx,
		name:                 serviceName,
		config:               agentConfig,
		service:              service,
		sendDocLevelResponse: sendDocLevelResponse,
		sendResponse:         sendResponse,
		orchestrationRootDir: orchestrationRootDir,
		processorStopPolicy:  stopPolicy,
		assocProcessor:       assocProc,
		pollAssociations:     pollAssoc,
		processor:            processor,
	}
}

// prepareReplyPayloadToUpdateDocumentStatus creates the payload object for SendReply based on document status change.
func prepareReplyPayloadToUpdateDocumentStatus(agentInfo contracts.AgentInfo, documentStatus contracts.ResultStatus, documentTraceOutput string) (payload messageContracts.SendReplyPayload) {

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

func processSendReply(log log.T, messageID string, mdsService mdsService.Service, payloadDoc messageContracts.SendReplyPayload, processorStopPolicy *sdkutil.StopPolicy) {
	payloadB, err := json.Marshal(payloadDoc)
	if err != nil {
		log.Error("could not marshal reply payload!", err)
	}
	payload := string(payloadB)
	log.Info("Sending reply ", jsonutil.Indent(payload))
	err = mdsService.SendReply(log, messageID, payload)
	if err != nil {
		sdkutil.HandleAwsError(log, err, processorStopPolicy)
	}
}

var newOfflineService = func(log log.T) (mdsService.Service, error) {
	return mdsService.NewOfflineService(log, string(SendCommandTopicPrefixOffline))
}

var newMdsService = func(config appconfig.SsmagentConfig) mdsService.Service {
	connectionTimeout := time.Duration(config.Mds.StopTimeoutMillis) * time.Millisecond

	return mdsService.NewService(
		config.Agent.Region,
		config.Mds.Endpoint,
		nil,
		connectionTimeout,
	)
}

var newStopPolicy = func(name string) *sdkutil.StopPolicy {
	return sdkutil.NewStopPolicy(name, stopPolicyErrorThreshold)
}
