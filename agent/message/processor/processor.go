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
	"github.com/aws/amazon-ssm-agent/agent/framework/engine"
	"github.com/aws/amazon-ssm-agent/agent/framework/plugin"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/message/contracts"
	"github.com/aws/amazon-ssm-agent/agent/message/parser"
	"github.com/aws/amazon-ssm-agent/agent/message/service"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/reply"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/amazon-ssm-agent/agent/statemanager"
	"github.com/aws/amazon-ssm-agent/agent/statemanager/model"
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

	CancelWorkersLimit = 3

	// name is the core plugin name
	name = "MessageProcessor"

	// pollMessageFrequencyMinutes is the frequency at which to resume poll for messages if the current thread dies due to stop policy
	// note: the connection timeout for MDSPoll should be less than this.
	pollMessageFrequencyMinutes = 15

	// pollAssociationFrequencyMinutes is the frequency at which to resume poll for Association if the current thread dies due to stop policy
	pollAssociationFrequencyMinutes = 30

	// hardstopTimeout is the time before the processor will be shutdown during a hardstop
	// TODO:  load this value from config
	hardStopTimeout = time.Second * 4

	// the default stoppolicy error threshold. After 10 consecutive errors the plugin will stop for 15 minutes.
	stopPolicyErrorThreshold = 10
)

type replyBuilder func(pluginID string, results map[string]*contracts.PluginResult) messageContracts.SendReplyPayload

type statusReplyBuilder func(agentInfo contracts.AgentInfo, resultStatus contracts.ResultStatus)

type persistData func(state *model.DocumentState, bookkeeping string)

// Processor is an object that can process MDS messages.
type Processor struct {
	context              context.T
	stopSignal           chan bool
	config               contracts.AgentConfiguration
	service              service.Service
	pluginRunner         PluginRunner
	sendCommandPool      task.Pool
	cancelCommandPool    task.Pool
	buildReply           replyBuilder
	sendResponse         engine.SendResponse
	sendDocLevelResponse engine.SendDocumentLevelResponse
	persistData          persistData
	orchestrationRootDir string
	messagePollJob       *scheduler.Job
	assocProcessor       *processor.Processor
	processorStopPolicy  *sdkutil.StopPolicy
}

// PluginRunner is a function that can run a set of plugins and return their outputs.
type PluginRunner func(context context.T, documentID string, plugins []model.PluginState, sendResponse engine.SendResponse, cancelFlag task.CancelFlag) (pluginOutputs map[string]*contracts.PluginResult)

var pluginRunner = func(context context.T, documentID string, plugins []model.PluginState, sendResponse engine.SendResponse, cancelFlag task.CancelFlag) (pluginOutputs map[string]*contracts.PluginResult) {
	return engine.RunPlugins(context, documentID, "", plugins, plugin.RegisteredWorkerPlugins(context), sendResponse, nil, cancelFlag)
}

// NewProcessor initializes a new mds processor with the given parameters.
func NewProcessor(context context.T) *Processor {
	messageContext := context.With("[" + name + "]")
	log := messageContext.Log()
	config := messageContext.AppConfig()

	instanceID, err := platform.InstanceID()
	if instanceID == "" {
		log.Errorf("no instanceID provided, %v", err)
		return nil
	}

	mdsService := newMdsService(config)

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
	sendCommandTaskPool := task.NewPool(log, config.Mds.CommandWorkersLimit, cancelWaitDuration, clock)
	cancelCommandTaskPool := task.NewPool(log, CancelWorkersLimit, cancelWaitDuration, clock)

	// create new message processor
	orchestrationRootDir := filepath.Join(appconfig.DefaultDataStorePath, instanceID, appconfig.DefaultDocumentRootDirName, config.Agent.OrchestrationRootDir)

	replyBuilder := func(pluginID string, results map[string]*contracts.PluginResult) messageContracts.SendReplyPayload {
		runtimeStatuses := reply.PrepareRuntimeStatuses(log, results)
		return reply.PrepareReplyPayload(pluginID, runtimeStatuses, clock.Now(), agentConfig.AgentInfo)
	}

	statusReplyBuilder := func(agentInfo contracts.AgentInfo, resultStatus contracts.ResultStatus, documentTraceOutput string) messageContracts.SendReplyPayload {
		return parser.PrepareReplyPayloadToUpdateDocumentStatus(agentInfo, resultStatus, documentTraceOutput)

	}
	// create a stop policy where we will stop after 10 consecutive errors and if time period expires.
	processorStopPolicy := newStopPolicy()

	// SendResponse is used to send response on plugin completion.
	// If pluginID is empty it will send responses of all plugins.
	// If pluginID is specified, response will be sent of that particular plugin.
	sendResponse := func(messageID string, pluginID string, results map[string]*contracts.PluginResult) {
		payloadDoc := replyBuilder(pluginID, results)
		processSendReply(log, messageID, mdsService, payloadDoc, processorStopPolicy)
	}

	// SendDocLevelResponse is used to send document level update
	// Specify a new status of the document
	sendDocLevelResponse := func(messageID string, resultStatus contracts.ResultStatus, documentTraceOutput string) {
		payloadDoc := statusReplyBuilder(agentInfo, resultStatus, documentTraceOutput)
		processSendReply(log, messageID, mdsService, payloadDoc, processorStopPolicy)
	}

	// PersistData is used to persist the data into a bookkeeping folder
	persistData := func(state *model.DocumentState, bookkeeping string) {
		statemanager.PersistData(log, state.DocumentInformation.DocumentID, state.DocumentInformation.InstanceID, bookkeeping, *state)
	}

	assocProcessor := processor.NewAssociationProcessor(context, instanceID)

	return &Processor{
		context:              messageContext,
		stopSignal:           make(chan bool),
		config:               agentConfig,
		service:              mdsService,
		pluginRunner:         pluginRunner,
		sendCommandPool:      sendCommandTaskPool,
		cancelCommandPool:    cancelCommandTaskPool,
		buildReply:           replyBuilder,
		sendResponse:         sendResponse,
		sendDocLevelResponse: sendDocLevelResponse,
		orchestrationRootDir: orchestrationRootDir,
		persistData:          persistData,
		processorStopPolicy:  processorStopPolicy,
		assocProcessor:       assocProcessor,
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

var newMdsService = func(config appconfig.SsmagentConfig) service.Service {
	connectionTimeout := time.Duration(config.Mds.StopTimeoutMillis) * time.Millisecond

	return service.NewService(
		config.Agent.Region,
		config.Mds.Endpoint,
		nil,
		connectionTimeout,
	)
}

var newStopPolicy = func() *sdkutil.StopPolicy {
	return sdkutil.NewStopPolicy(name, stopPolicyErrorThreshold)
}
