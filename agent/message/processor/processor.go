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
package processor

import (
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/association/processor"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/message/contracts"
	"github.com/aws/amazon-ssm-agent/agent/message/parser"
	"github.com/aws/amazon-ssm-agent/agent/message/processor/executer"
	"github.com/aws/amazon-ssm-agent/agent/message/processor/executer/basicexecuter"
	"github.com/aws/amazon-ssm-agent/agent/message/service"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/reply"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
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
	mdsName = "MessageProcessor"

	// offlinename is the core module name for the offline command document processor
	offlineName = "OfflineProcessor"

	// pollMessageFrequencyMinutes is the frequency at which to resume poll for messages if the current thread dies due to stop policy
	// note: the connection timeout for MDSPoll should be less than this.
	pollMessageFrequencyMinutes = 15

	// hardstopTimeout is the time before the processor will be shutdown during a hardstop
	// TODO:  load this value from config
	hardStopTimeout = time.Second * 4

	// the default stoppolicy error threshold. After 10 consecutive errors the plugin will stop for 15 minutes.
	stopPolicyErrorThreshold = 10
)

type statusReplyBuilder func(agentInfo contracts.AgentInfo, resultStatus contracts.ResultStatus)

type persistData func(state *model.DocumentState, bookkeeping string)

type ExecuterCreator func(ctx context.T) executer.Executer

//TODO move these 2 type to service
// SendDocumentLevelResponse is used to send status response before plugin begins
type SendDocumentLevelResponse func(messageID string, resultStatus contracts.ResultStatus, documentTraceOutput string)
type SendResponse func(pluginID string, res contracts.PluginResult)

// responseProvider is a closure to hold replyBuilder, before we create the service interface
var responseProvider = func(log log.T, messageID string, mdsService service.Service, agentInfo contracts.AgentInfo, stopPolicy *sdkutil.StopPolicy) SendResponse {
	replyBuilder := reply.NewSendReplyBuilder()
	return func(pluginID string, res contracts.PluginResult) {
		//TODO this is temporarily solution before we have service; once we have it, a nice and clean protocol will be defined in terms of status update
		if pluginID == "" {
			processSendReply(log, messageID, mdsService, replyBuilder.FormatPayload(log, "", agentInfo), stopPolicy)
			return
		}
		replyBuilder.UpdatePluginResult(res)
		processSendReply(log, messageID, mdsService, replyBuilder.FormatPayload(log, res.PluginName, agentInfo), stopPolicy)
	}
}

// Processor is an object that can process MDS messages.
type Processor struct {
	context              context.T
	name                 string
	stopSignal           chan bool
	config               contracts.AgentConfiguration
	service              service.Service
	executerCreator      ExecuterCreator
	sendCommandPool      task.Pool
	cancelCommandPool    task.Pool
	sendDocLevelResponse SendDocumentLevelResponse
	persistData          persistData
	orchestrationRootDir string
	messagePollJob       *scheduler.Job
	assocProcessor       *processor.Processor
	processorStopPolicy  *sdkutil.StopPolicy
	pollAssociations     bool
	supportedDocTypes    []model.DocumentType
}

// NewOfflineProcessor initialize a new offline command document processor
func NewOfflineProcessor(context context.T) (*Processor, error) {
	messageContext := context.With("[" + offlineName + "]")
	log := messageContext.Log()

	log.Debug("Creating offline command document service")
	offlineService, err := newOfflineService(log)
	if err != nil {
		return nil, err
	}

	return NewProcessor(messageContext, offlineName, offlineService, 1, 1, false, []model.DocumentType{model.SendCommandOffline, model.CancelCommandOffline}), nil
}

// NewMdsProcessor initializes a new mds processor with the given parameters.
func NewMdsProcessor(context context.T) *Processor {
	messageContext := context.With("[" + mdsName + "]")
	mdsService := newMdsService(context.AppConfig())
	config := context.AppConfig()

	return NewProcessor(messageContext, mdsName, mdsService, config.Mds.CommandWorkersLimit, CancelWorkersLimit, true, []model.DocumentType{model.SendCommand, model.CancelCommand})
}

// NewProcessor performs common initialization for Mds and Offline processors
func NewProcessor(ctx context.T, processorName string, processorService service.Service, commandWorkerLimit int, cancelWorkerLimit int, pollAssoc bool, supportedDocs []model.DocumentType) *Processor {
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

	// sendCommand and cancelCommand will be processed by separate worker pools
	// so we can define the number of workers per each
	cancelWaitDuration := 10000 * time.Millisecond
	clock := times.DefaultClock
	sendCommandTaskPool := task.NewPool(log, commandWorkerLimit, cancelWaitDuration, clock)
	cancelCommandTaskPool := task.NewPool(log, CancelWorkersLimit, cancelWaitDuration, clock)

	// create new message processor
	orchestrationRootDir := filepath.Join(appconfig.DefaultDataStorePath, instanceID, appconfig.DefaultDocumentRootDirName, config.Agent.OrchestrationRootDir)

	statusReplyBuilder := func(agentInfo contracts.AgentInfo, resultStatus contracts.ResultStatus, documentTraceOutput string) messageContracts.SendReplyPayload {
		return parser.PrepareReplyPayloadToUpdateDocumentStatus(agentInfo, resultStatus, documentTraceOutput)

	}
	// create a stop policy where we will stop after 10 consecutive errors and if time period expires.
	processorStopPolicy := newStopPolicy(processorName)

	//TODO move this function to service
	// SendDocLevelResponse is used to send document level update
	// Specify a new status of the document
	sendDocLevelResponse := func(messageID string, resultStatus contracts.ResultStatus, documentTraceOutput string) {
		payloadDoc := statusReplyBuilder(agentInfo, resultStatus, documentTraceOutput)
		processSendReply(log, messageID, processorService, payloadDoc, processorStopPolicy)
	}

	// PersistData is used to persist the data into a bookkeeping folder
	persistData := func(state *model.DocumentState, bookkeeping string) {
		docmanager.PersistData(log, state.DocumentInformation.DocumentID, state.DocumentInformation.InstanceID, bookkeeping, *state)
	}

	var assocProc *processor.Processor
	if pollAssoc {
		assocProc = processor.NewAssociationProcessor(ctx, instanceID)
	}
	//TODO in future, this attribute should be injected by service
	var executerCreator = func(ctx context.T) executer.Executer {
		return basicexecuter.NewBasicExecuter(ctx)
	}

	return &Processor{
		context:              ctx,
		name:                 processorName,
		stopSignal:           make(chan bool),
		config:               agentConfig,
		service:              processorService,
		executerCreator:      executerCreator,
		sendCommandPool:      sendCommandTaskPool,
		cancelCommandPool:    cancelCommandTaskPool,
		sendDocLevelResponse: sendDocLevelResponse,
		orchestrationRootDir: orchestrationRootDir,
		persistData:          persistData,
		processorStopPolicy:  processorStopPolicy,
		assocProcessor:       assocProc,
		pollAssociations:     pollAssoc,
		supportedDocTypes:    supportedDocs,
	}
}

func processSendReply(log log.T, messageID string, mdsService service.Service, payloadDoc messageContracts.SendReplyPayload, processorStopPolicy *sdkutil.StopPolicy) {
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

var newOfflineService = func(log log.T) (service.Service, error) {
	return service.NewOfflineService(log, string(SendCommandTopicPrefixOffline))
}

var newMdsService = func(config appconfig.SsmagentConfig) service.Service {
	connectionTimeout := time.Duration(config.Mds.StopTimeoutMillis) * time.Millisecond

	return service.NewService(
		config.Agent.Region,
		config.Mds.Endpoint,
		nil,
		connectionTimeout,
	)
}

var newStopPolicy = func(name string) *sdkutil.StopPolicy {
	return sdkutil.NewStopPolicy(name, stopPolicyErrorThreshold)
}
