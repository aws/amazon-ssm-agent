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
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package mgsinteractor contains logic to open control channel and communicate with MGS
package mgsinteractor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"path/filepath"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/interactor"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/interactor/mgsinteractor/replytypes"
	interactorutils "github.com/aws/amazon-ssm-agent/agent/messageservice/interactor/mgsinteractor/utils"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/messagehandler"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/utils"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/runcommand/contracts"
	mgsConfig "github.com/aws/amazon-ssm-agent/agent/session/config"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/aws/amazon-ssm-agent/agent/session/controlchannel"
	"github.com/aws/amazon-ssm-agent/agent/session/retry"
	"github.com/aws/amazon-ssm-agent/agent/session/service"
	"github.com/aws/amazon-ssm-agent/agent/ssmconnectionchannel"
	"github.com/gorilla/websocket"
	"github.com/twinj/uuid"
)

const (
	Name          = "MGSInteractor"
	ISO8601Format = "2006-01-02T15:04:05.000Z"
)

// MGSInteractor defines the properties and methods to communicate with MDS
type MGSInteractor struct {
	context                  context.T
	agentConfig              contracts.AgentConfiguration
	name                     string
	mgsService               service.Service
	controlChannel           controlchannel.IControlChannel
	incomingAgentMessageChan chan mgsContracts.AgentMessage
	sendReplyProp            *sendReplyProperties
	messageHandler           messagehandler.IMessageHandler
	replyChan                chan contracts.DocumentResult
	channelOpen              bool
	ackSkipCodes             map[messagehandler.ErrorCode]string
	listenReplyThreadEnded   chan struct{}
	mutex                    sync.Mutex
	updateWatcherDone        chan bool
	handledUpdateReplies     sync.Map
}

// New initiates and returns MGS Interactor when needed
func New(context context.T, messageHandler messagehandler.IMessageHandler) (interactor.IInteractor, error) {
	mgsContext := context.With("[" + Name + "]")
	log := mgsContext.Log()
	appConfig := context.AppConfig()

	instanceID, err := context.Identity().InstanceID()
	if instanceID == "" {
		errorMsg := log.Errorf("no instanceID provided, %v", err)
		return nil, errorMsg
	}
	agentInfo := contracts.AgentInfo{
		Lang:      appConfig.Os.Lang,
		Name:      appConfig.Agent.Name,
		Version:   appConfig.Agent.Version,
		Os:        appConfig.Os.Name,
		OsVersion: appConfig.Os.Version,
	}

	agentConfig := contracts.AgentConfiguration{
		AgentInfo:  agentInfo,
		InstanceID: instanceID,
	}

	messageGatewayServiceConfig := appConfig.Mgs
	if messageGatewayServiceConfig.Region == "" {
		fetchedRegion, err := context.Identity().Region()
		if err != nil {
			errorMsg := log.Errorf("Failed to get region with error: %s", err)
			return nil, errorMsg
		}
		messageGatewayServiceConfig.Region = fetchedRegion
	}

	if messageGatewayServiceConfig.Endpoint == "" {
		fetchedEndpoint, err := getMgsEndpoint(context, messageGatewayServiceConfig.Region)
		if err != nil {
			errorMsg := log.Errorf("Failed to get MessageGatewayService endpoint with error: %s", err)
			return nil, errorMsg
		}
		messageGatewayServiceConfig.Endpoint = fetchedEndpoint
	}

	connectionTimeout := time.Duration(messageGatewayServiceConfig.StopTimeoutMillis) * time.Millisecond
	mgsService := service.NewService(context, messageGatewayServiceConfig, connectionTimeout)
	controlChannel := &controlchannel.ControlChannel{}
	incomingAgentMessageChan := make(chan mgsContracts.AgentMessage)
	sendReplyProp := &sendReplyProperties{
		replyQueueLimit: appConfig.Mds.CommandWorkersLimit, // assigning the command worker limit as the number of reply threads
		replyThreadDone: make(chan struct{}),
		reply:           make(chan *agentReplyLocalContract),
		allReplyClosed:  make(chan struct{}, 1),
	}

	mgsInteract := &MGSInteractor{
		context:                  mgsContext,
		name:                     Name,
		mgsService:               mgsService,
		controlChannel:           controlChannel,
		agentConfig:              agentConfig,
		incomingAgentMessageChan: incomingAgentMessageChan,
		sendReplyProp:            sendReplyProp,
		messageHandler:           messageHandler,
		replyChan:                make(chan contracts.DocumentResult),
	}

	// the below line makes sure that the interactor receives all the replies from documents with
	// upstream service name as contracts.MessageGatewayService in this replyChan
	mgsInteract.messageHandler.RegisterReply(contracts.MessageGatewayService, mgsInteract.replyChan)
	return mgsInteract, nil
}

// GetName used to get the name of interactor
func (mgs *MGSInteractor) GetName() string {
	return mgs.name
}

// GetSupportedWorkers returns the processors needed by the interactors
// this function can be changed to GetRequiredWorkers in future
func (mgs *MGSInteractor) GetSupportedWorkers() []utils.WorkerName {
	workers := make([]utils.WorkerName, 0)
	// This is added to block command processor wrapper load for containers
	if !mgs.context.AppConfig().Agent.ContainerMode {
		workers = append(workers, utils.DocumentWorkerName)
	}
	workers = append(workers, utils.SessionWorkerName)
	return workers
}

// Initialize initializes interactor properties and starts failed reply job
func (mgs *MGSInteractor) Initialize(ableToOpenMGSConnection *uint32) (err error) {
	log := mgs.context.Log()
	// initialize ack skip codes
	mgs.ackSkipCodes = map[messagehandler.ErrorCode]string{
		messagehandler.ClosedProcessor:                     "51401",
		messagehandler.ProcessorBufferFull:                 "51402",
		messagehandler.UnexpectedDocumentType:              "51403",
		messagehandler.ProcessorErrorCodeTranslationFailed: "51404",
		messagehandler.DuplicateCommand:                    "51405",
		messagehandler.InvalidDocument:                     "51406",
		messagehandler.ContainerNotSupported:               "51407",
		messagehandler.AgentJobMessageParseError:           "51408",
		messagehandler.UnexpectedError:                     "51499",
		messagehandler.Successful:                          "200",
	}

	mgs.listenReplyThreadEnded = make(chan struct{}, 1)

	mgs.updateWatcherDone = make(chan bool, 1)

	// listens incoming channel for agent related messages
	go mgs.listenIncomingAgentMessages()

	// below go routines should be started before the control channel connection
	// this is because processors will start sending replies from inProgress/pending documents during agent restart
	// we may also receive documents that came through MGS
	// in this case, it will go to failed reply queue
	//
	// The below goroutine starts the reply queue
	go mgs.startReplyProcessingQueue()
	// listen to the replies received from message handler and pushes to the reply queue
	go mgs.listenReply()

	log.Info("SSM Agent is trying to setup control channel for MGSInteractor")
	mgs.controlChannel, err = setupControlChannel(mgs.context, mgs.mgsService, mgs.agentConfig.InstanceID, mgs.incomingAgentMessageChan, ableToOpenMGSConnection)
	if err != nil {
		log.Errorf("Error setting up control channel: %v", err)
		return err
	}
	log.Info("Set up control channel successfully")
	if ableToOpenMGSConnection != nil {
		atomic.StoreUint32(ableToOpenMGSConnection, 1)
		ssmconnectionchannel.SetConnectionChannel(ableToOpenMGSConnection)
	}
	return nil
}

// PostProcessorInitialization registers executes PostProcessorInitialization operations
// Will be executed after the processor initialization is done in MessageService.
func (mgs *MGSInteractor) PostProcessorInitialization(worker utils.WorkerName) {
	switch worker {
	case utils.DocumentWorkerName:
		mgs.setChannelOpenVal(true)
		go mgs.startUpdateReplyFileWatcher()
	default:
	}

	// adding it here because it will be helpful to send the failed replies from Command processor initialization immediately
	if !mgs.isSendFailedReplyJobScheduled() {
		// sleep for 2 seconds to wait for the current failed replies to be saved
		// this won't block the current running thread in message service
		time.Sleep(2 * time.Second)
		// starts the job to load the failed replies in the previous execution and pushes to the reply queue
		mgs.startSendFailedReplyJob()
	}
}

func (mgs *MGSInteractor) isChannelOpenForAgentJobMsgs() bool {
	mgs.mutex.Lock()
	defer mgs.mutex.Unlock()
	return mgs.channelOpen
}

func (mgs *MGSInteractor) setChannelOpenVal(openVal bool) {
	mgs.mutex.Lock()
	defer mgs.mutex.Unlock()
	mgs.channelOpen = openVal
}

// PreProcessorClose defines actions to be performed before MGS connection close
func (mgs *MGSInteractor) PreProcessorClose() {
	mgs.setChannelOpenVal(false) // close the incoming agent job message
	mgs.closeSendFailedReplyJob()
	mgs.context.Log().Info("MGS send failed reply job closed")
	mgs.stopUpdateReplyFileWatcher()
}

// Close closes the existing MGS connection
func (mgs *MGSInteractor) Close() (err error) {
	log := mgs.context.Log()

	// processors would have been stopped at this point and expect no new reply to receive
	<-mgs.listenReplyThreadEnded
	close(mgs.sendReplyProp.reply)
	<-mgs.sendReplyProp.allReplyClosed

	if mgs.controlChannel != nil {
		if err = mgs.controlChannel.Close(log); err != nil {
			log.Errorf("Stopping control channel encountered error: %s", err)
			return err
		}
	}
	return nil
}

// listenReply listens to the replies and pushes to the reply queue
func (mgs *MGSInteractor) listenReply() {
	log := mgs.context.Log()
	mgs.listenReplyThreadEnded = make(chan struct{}, 1)
	log.Info("listen reply thread in MGS interactor started")
	defer func() {
		log.Info("listen reply thread in MGS interactor ended")
		if r := recover(); r != nil {
			log.Errorf("listen reply in mgsinteractor panicked: \n%v", r)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
			time.Sleep(2 * time.Second)
			go mgs.listenReply()
		}
	}()

externalLoop:
	for {
		select {
		case reply, isOpen := <-mgs.replyChan:
			if !isOpen {
				log.Info("reply channel closed")
				break externalLoop
			}
			replyUUID := uuid.NewV4()
			log.Infof("received reply for %v %v with message id %v", reply.ResultType, reply.MessageID, replyUUID.String())
			replyObject, err := replytypes.GetReplyTypeObject(mgs.context, reply, replyUUID, 0)
			if err != nil {
				log.Errorf("error while constructing reply object %v", err)
				break // break from select
			}
			replyObjectLocalContract := &agentReplyLocalContract{
				documentResult: replyObject,
			}
			replyStr, err := jsonutil.Marshal(reply)
			if err != nil {
				log.Debugf("Could not parse result %v ", err)
			}
			log.Debugf("Processing reply message %v", jsonutil.Indent(replyStr))
			mgs.sendReplyProp.reply <- replyObjectLocalContract
		}
	}
	close(mgs.listenReplyThreadEnded)
}

// listenIncomingAgentMessages listens to the incoming messages and submits to the message handler
func (mgs *MGSInteractor) listenIncomingAgentMessages() {
	log := mgs.context.Log()
	log.Info("listen incoming messages thread in MGS interactor started")
	defer func() {
		log.Info("listen incoming messages thread in MGS interactor ended")
		if r := recover(); r != nil {
			log.Errorf("listen incoming messages panic: \n%v", r)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
			time.Sleep(2 * time.Second)
			go mgs.listenIncomingAgentMessages()
		}
	}()

	for agentMessage := range mgs.incomingAgentMessageChan {
		log.Infof("Processing AgentMessage: MessageType - %s, Id - %s", agentMessage.MessageType, agentMessage.MessageId)
		switch agentMessage.MessageType {
		case mgsContracts.AgentJobMessage:
			mgs.processAgentJobMessage(agentMessage)
		case mgsContracts.InteractiveShellMessage, mgsContracts.ChannelClosedMessage:
			mgs.processSessionRelatedMessages(agentMessage)
		case mgsContracts.TaskAcknowledgeMessage:
			mgs.processTaskAcknowledgeMessage(agentMessage)
		case mgsContracts.AgentJobReplyAck:
			mgs.processJobReplyAck(log, agentMessage)
		default:
			log.Errorf("invalid message type in message: %+v", agentMessage)
		}
	}
}

func (mgs *MGSInteractor) processSessionRelatedMessages(agentMessage mgsContracts.AgentMessage) {
	appConfig := mgs.context.AppConfig()
	blockChan := make(chan struct{})
	log := mgs.context.Log()
	shortInstanceId, _ := mgs.context.Identity().ShortInstanceID()
	sessionOrchestrationRootDir := filepath.Join(appconfig.DefaultDataStorePath, shortInstanceId, appconfig.DefaultSessionRootDirName, appConfig.Agent.OrchestrationRootDir)
	docState, err := agentMessage.ParseAgentMessage(mgs.context, sessionOrchestrationRootDir, mgs.agentConfig.InstanceID)
	if err != nil {
		log.Errorf("Cannot parse AgentTask message to documentState: %s, err: %v.", agentMessage.MessageId, err)
		return
	}
	log.Debugf("Pushing %s message %s to MessageHandler incoming message chan", agentMessage.MessageType, agentMessage.MessageId)
	var errorCode messagehandler.ErrorCode
	retryLimit := 5
	// 5 retries for ProcessorBufferFull. This should not happen most of the time as we have a higher default session limit.
	for retryNumber := 1; retryNumber <= retryLimit; retryNumber++ {
		errorCode = mgs.messageHandler.Submit(docState)
		if errorCode == messagehandler.ProcessorBufferFull && retryNumber != retryLimit {
			log.Errorf("received error code while checking processor buffer space for session messages %v", errorCode)
			time.Sleep(time.Second)
			continue
		}
		break
	}
	if errorCode == "" { // Successful submission
		log.Debugf("pushed message %s with document id %s to processor", agentMessage.MessageId.String(), docState.DocumentInformation.DocumentID)
		return
	}
	if errorCode == messagehandler.ProcessorBufferFull {
		log.Errorf("blocking control channel because of error code in session processor: %v", errorCode)
		blockChan <- struct{}{}
	}
	// should not happen
	if _, ok := mgs.ackSkipCodes[errorCode]; ok {
		log.Warnf("dropping session message %v due to error code: %v", docState.DocumentInformation.DocumentID, errorCode)
		return
	}
}

func (mgs *MGSInteractor) processJobReplyAck(log log.T, agentMessage mgsContracts.AgentMessage) {
	replyAcknowledge := &mgsContracts.AgentJobReplyAckContent{}
	if err := replyAcknowledge.Deserialize(log, agentMessage); err != nil {
		log.Errorf("Cannot parse AgentReply message to taskAck message: %s, err: %v.", agentMessage.MessageId, err)
	}
	log.Infof("received ack id %v for message id %v", replyAcknowledge.AcknowledgedMessageId, agentMessage.MessageId)
	if ackChan, ok := mgs.sendReplyProp.replyAckChan.Load(replyAcknowledge.AcknowledgedMessageId); ok {
		ackChan.(chan bool) <- true
		mgs.sendReplyProp.replyAckChan.Delete(replyAcknowledge.AcknowledgedMessageId) // deletion happens in the reply queue too
	} else {
		log.Warnf("acknowledgement %v received but could not find any reply threads running", replyAcknowledge.AcknowledgedMessageId)
	}
}

func (mgs *MGSInteractor) processAgentJobMessage(agentMessage mgsContracts.AgentMessage) {
	appConfig := mgs.context.AppConfig()
	log := mgs.context.Log()
	if !mgs.isChannelOpenForAgentJobMsgs() {
		log.Infof("dropping message because the channel is not open: %s", agentMessage.MessageId.String())
		return
	}

	shortInstanceId, _ := mgs.context.Identity().ShortInstanceID()
	commandOrchestrationRootDir := filepath.Join(appconfig.DefaultDataStorePath, shortInstanceId, appconfig.DefaultDocumentRootDirName, appConfig.Agent.OrchestrationRootDir)
	docState, err := agentMessage.ParseAgentMessage(mgs.context, commandOrchestrationRootDir, mgs.agentConfig.InstanceID)
	// just dropping all errors - MDS will take care of these messages
	// we should handle few errors differently in future
	if err != nil {
		log.Errorf("dropping message because cannot parse AgentJob message %s to Document State, err: %v", agentMessage.MessageId.String(), err)
		agentJobId, _ := agentMessage.GetAgentJobId(mgs.context)
		mgs.buildAgentJobAckMessageAndSend(agentMessage.MessageId, agentJobId, agentMessage.CreatedDate, messagehandler.AgentJobMessageParseError)
		return
	} else {
		if mgs.context.AppConfig().Agent.ContainerMode {
			log.Errorf("dropping message because job messages are not supported for containers: %s", agentMessage.MessageId.String())
			mgs.buildAgentJobAckMessageAndSend(agentMessage.MessageId, docState.DocumentInformation.MessageID, agentMessage.CreatedDate, messagehandler.ContainerNotSupported)
			return
		}
		log.Debugf("pushing AgentJob message %s to MessageHandler incoming message chan", agentMessage.MessageId.String())
		errorCode := mgs.messageHandler.Submit(docState)
		if errorCode != "" {
			if _, ok := mgs.ackSkipCodes[errorCode]; ok {
				log.Warnf("dropping message %v because of error code %v", docState.DocumentInformation.DocumentID, errorCode)
				mgs.buildAgentJobAckMessageAndSend(agentMessage.MessageId, docState.DocumentInformation.MessageID, agentMessage.CreatedDate, errorCode)
				return
			}
		}
		err = mgs.buildAgentJobAckMessageAndSend(agentMessage.MessageId, docState.DocumentInformation.MessageID, agentMessage.CreatedDate, messagehandler.Successful)
		if err != nil { // proceed without returning during error as the doc would have been already persisted
			log.Errorf("could not send ack for message %v because of error: %v", docState.DocumentInformation.DocumentID, err)
		}

		payloadDoc := utils.PrepareReplyPayloadToUpdateDocumentStatus(mgs.agentConfig.AgentInfo, contracts.ResultStatusInProgress, "", nil)
		// no persisting done for this message as this does not impact the command result
		mgs.sendDocResponse(payloadDoc, docState)
		log.Debugf("pushed message %s with document id %s to processor", agentMessage.MessageId.String(), docState.DocumentInformation.DocumentID)
	}
}

func (mgs *MGSInteractor) sendDocResponse(payloadDoc messageContracts.SendReplyPayload, docState *contracts.DocumentState) {
	log := mgs.context.Log()
	replyUUID := uuid.NewV4()
	commandTopic := interactorutils.GetTopicFromDocResult(contracts.RunCommandResult, docState.DocumentType)
	agentMsg, err := interactorutils.GenerateAgentJobReplyPayload(log, replyUUID, docState.DocumentInformation.MessageID, payloadDoc, commandTopic)
	if err != nil {
		log.Errorf("error while generating agent job reply payload: %v", err)
		return
	}
	msg, err := agentMsg.Serialize(log)
	if err != nil {
		// Should never happen
		log.Errorf("error serializing agent message: %v", err)
		return
	}
	if err = mgs.controlChannel.SendMessage(log, msg, websocket.BinaryMessage); err == nil {
		log.Debugf("successfully sent document response with client message id : %v for CommandId %s", replyUUID, docState.DocumentInformation.CommandID)
	} else {
		log.Errorf("error while sending document response message with client message id : %v, err: %v", replyUUID, err)
	}
}

func (mgs *MGSInteractor) buildAgentJobAckMessageAndSend(ackMessageId uuid.UUID, jobId string, createdDate uint64, errorCode messagehandler.ErrorCode) error {
	log := mgs.context.Log()

	statusCode, ok := mgs.ackSkipCodes[errorCode]
	if !ok {
		// Should never happen
		errorCode = messagehandler.UnexpectedError
		statusCode = mgs.ackSkipCodes[errorCode]
	}

	ackMsg := &mgsContracts.AgentJobAck{
		JobId:        jobId,
		MessageId:    ackMessageId.String(),
		CreatedDate:  toISO8601(createdDate),
		StatusCode:   statusCode,
		ErrorMessage: string(errorCode),
	}

	replyBytes, err := json.Marshal(ackMsg)
	if err != nil {
		// should not happen
		log.Errorf("Cannot build AgentJobAck message %s", err)
		return err
	}
	replyUUID := uuid.NewV4()
	agentMessage := &mgsContracts.AgentMessage{
		MessageType:    mgsContracts.AgentJobAcknowledgeMessage,
		SchemaVersion:  1,
		CreatedDate:    uint64(time.Now().UnixNano() / 1000000),
		SequenceNumber: 0,
		Flags:          0,
		MessageId:      replyUUID,
		Payload:        replyBytes,
	}

	msg, err := agentMessage.Serialize(log)
	if err != nil {
		// Should never happen
		log.Errorf("Error serializing agent message: %v", err)
		return err
	}

	if err = mgs.controlChannel.SendMessage(log, msg, websocket.BinaryMessage); err != nil {
		log.Errorf("Error sending agent job ack message, ID [%v], err: %v", ackMessageId.String(), err)
		return err
	}
	log.Infof("Successfully sent ack message id %s back for JobID %s", replyUUID, jobId)
	return nil
}

func (mgs *MGSInteractor) processTaskAcknowledgeMessage(agentMessage mgsContracts.AgentMessage) {
	log := mgs.context.Log()
	log.Debugf("Processing Task Acknowledge message %s", agentMessage.MessageId.String())

	taskAcknowledge := &mgsContracts.AcknowledgeTaskContent{}
	if err := taskAcknowledge.Deserialize(log, agentMessage); err != nil {
		log.Errorf("Cannot parse AgentTask message to TaskAcknowledgeMessage message: %s, err: %v.", agentMessage.MessageId.String(), err)
		return
	}

	log.Debugf("TaskAcknowledgeMessage for topic [%s] and sessionId [%s]", taskAcknowledge.Topic, taskAcknowledge.TaskId)
	if ackChan, ok := mgs.sendReplyProp.replyAckChan.Load(taskAcknowledge.MessageId); ok {
		ackChan.(chan bool) <- true
		mgs.sendReplyProp.replyAckChan.Delete(taskAcknowledge.MessageId) // deletion happens in the reply queue too
	}
}

var setupControlChannel = func(context context.T, mgsService service.Service, instanceId string, agentMessageIncomingMessageChan chan mgsContracts.AgentMessage, ableToOpenMGSConnection *uint32) (controlchannel.IControlChannel, error) {
	retryer := retry.ExponentialRetryer{
		CallableFunc: func() (channel interface{}, err error) {
			controlChannel := &controlchannel.ControlChannel{}
			controlChannel.Initialize(context, mgsService, instanceId, agentMessageIncomingMessageChan)
			if err := controlChannel.SetWebSocket(context, mgsService, ableToOpenMGSConnection); err != nil {
				return nil, err
			}

			if err := controlChannel.Open(context.Log(), ableToOpenMGSConnection); err != nil {
				return nil, err
			}
			controlChannel.AuditLogScheduler.ScheduleAuditEvents()
			return controlChannel, nil
		},
		GeometricRatio:      mgsConfig.RetryGeometricRatio,
		JitterRatio:         mgsConfig.RetryJitterRatio,
		InitialDelayInMilli: rand.Intn(mgsConfig.ControlChannelRetryInitialDelayMillis) + mgsConfig.ControlChannelRetryInitialDelayMillis,
		MaxDelayInMilli:     mgsConfig.ControlChannelRetryMaxIntervalMillis,
		MaxAttempts:         mgsConfig.ControlChannelNumMaxRetries,
	}
	retryer.Init()
	channel, err := retryer.Call()
	if err != nil {
		// should never happen
		return nil, err
	}
	controlChannel := channel.(*controlchannel.ControlChannel)
	return controlChannel, nil
}

// getMgsEndpoint builds mgs endpoint.
func getMgsEndpoint(context context.T, region string) (string, error) {
	hostName := mgsConfig.GetMgsEndpoint(context, region)
	if hostName == "" {
		return "", fmt.Errorf("no MGS endpoint found for region %s", region)
	}
	var endpointBuilder bytes.Buffer
	endpointBuilder.WriteString(mgsConfig.HttpsPrefix)
	endpointBuilder.WriteString(hostName)
	return endpointBuilder.String(), nil
}

// convert uint64 to ISO-8601 time stamp
func toISO8601(createdDate uint64) string {
	timeVal := time.Unix(0, int64(createdDate)*int64(time.Millisecond)).UTC()
	return timeVal.Format(ISO8601Format)
}
