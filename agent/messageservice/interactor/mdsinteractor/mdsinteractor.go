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

// Package mdsinteractor will be responsible for communicating with MDS
package mdsinteractor

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	model "github.com/aws/amazon-ssm-agent/agent/messageservice/contracts"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/interactor"
	messageHandler "github.com/aws/amazon-ssm-agent/agent/messageservice/messagehandler"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/utils"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/runcommand/contracts"
	mdsService "github.com/aws/amazon-ssm-agent/agent/runcommand/mds"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/amazon-ssm-agent/agent/ssmconnectionchannel"
	"github.com/aws/aws-sdk-go/service/ssmmds"
	"github.com/carlescere/scheduler"
)

// MDSInteractor defines the properties and methods to communicate with MDS
type MDSInteractor struct {
	context                 context.T
	config                  contracts.AgentConfiguration
	service                 mdsService.Service
	orchestrationRootDir    string
	messagePollJob          *scheduler.Job
	sendReplyJob            *scheduler.Job
	messagePollWaitGroup    *sync.WaitGroup
	lastPollTime            time.Time
	mutex                   sync.RWMutex
	processorStopPolicy     *sdkutil.StopPolicy
	messageHandler          messageHandler.IMessageHandler
	replyChan               chan contracts.DocumentResult
	ackSkipCodes            map[messageHandler.ErrorCode]struct{}
	ableToOpenMGSConnection *uint32
	mdsState                MDSState
	mdsStateMutex           sync.RWMutex
	stopJobChannel          chan interface{}
	stopPollingJobGrp       sync.WaitGroup
}

const (
	// Name of the interactor, to register to message service
	Name = "MDSInteractor"

	// pollMessageFrequencyMinutes is the frequency at which to resume poll for messages if the current thread dies due to stop policy
	// note: the connection timeout for MDSPoll should be less than this.
	pollMessageFrequencyMinutes = 15

	// the default stoppolicy error threshold. After 10 consecutive errors the plugin will stop for 15 minutes.
	stopPolicyErrorThreshold = 10
)

type MDSState string

const (
	MDSStartInProgress MDSState = "MDSStartInProgress"
	MDSStartCompleted  MDSState = "MDSStartCompleted"
	MDSStopInProgress  MDSState = "MDSStopInProgress"
	MDSStopCompleted   MDSState = "MDSStopCompleted"
	MDSShutDown        MDSState = "MDSShutDown"
)

var (
	timeAfter                            = time.After
	isPlatformWindowsServer2012OrEarlier = platform.IsPlatformWindowsServer2012OrEarlier
)

// New initiates and returns MDS Interactor when needed
func New(context context.T, msgHandler messageHandler.IMessageHandler, service mdsService.Service) (interactor.IInteractor, error) {
	mdsContext := context.With("[" + Name + "]")
	log := mdsContext.Log()

	config := mdsContext.AppConfig()
	identity := mdsContext.Identity()

	instanceID, err := identity.InstanceID()
	if instanceID == "" {
		log.Errorf("no instanceID provided, %v", err)
		return nil, err
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

	// create a service object for mds
	if service == nil {
		service = newMdsService(context)
	}

	// create a stop policy where we will stop after 10 consecutive errors and if time period expires.
	stopPolicy := newStopPolicy(Name)

	shortInstanceId, _ := identity.ShortInstanceID()
	orchestrationRootDir := filepath.Join(appconfig.DefaultDataStorePath, shortInstanceId, appconfig.DefaultDocumentRootDirName, config.Agent.OrchestrationRootDir)

	// initialize ack skip code
	ackSkipCodes := map[messageHandler.ErrorCode]struct{}{
		messageHandler.ClosedProcessor:                     {},
		messageHandler.UnexpectedDocumentType:              {},
		messageHandler.ProcessorErrorCodeTranslationFailed: {},
		messageHandler.InvalidDocument:                     {},
		//messageHandler.ProcessorBufferFull:                 {}, // For Processor Buffer Full, we retry indefinitely until we get Success or other error codes
		//messageHandler.DuplicateCommand:                    {}, // For Duplicate command, we think this error as a success and send ACK
	}
	mdsInteract := &MDSInteractor{
		context:              mdsContext,
		config:               agentConfig,
		service:              service,
		orchestrationRootDir: orchestrationRootDir,
		processorStopPolicy:  stopPolicy,
		replyChan:            make(chan contracts.DocumentResult),
		messageHandler:       msgHandler,
		ackSkipCodes:         ackSkipCodes,
		stopJobChannel:       make(chan interface{}, 1),
	}
	// registers reply chan to message handler for receiving replies with UpstreamServiceName as MessageDeliveryService
	msgHandler.RegisterReply(contracts.MessageDeliveryService, mdsInteract.replyChan)
	return mdsInteract, nil
}

// GetName used to get the name of interactor
func (mds *MDSInteractor) GetName() string {
	return Name
}

// GetSupportedWorkers returns the workers needed by the interactors
func (mds *MDSInteractor) GetSupportedWorkers() []utils.WorkerName {
	return []utils.WorkerName{utils.DocumentWorkerName}
}

// Initialize initializes MDSInteractor properties and starts failed reply job
func (mds *MDSInteractor) Initialize(ableToOpenMGSConnection *uint32) (err error) {
	log := mds.context.Log()
	mds.ableToOpenMGSConnection = ableToOpenMGSConnection

	log.Info("Starting message polling")
	mds.messagePollWaitGroup = &sync.WaitGroup{}

	log.Info("Starting send failed replies to MDS")
	if mds.sendReplyJob, err = scheduler.Every(utils.SendFailedReplyFrequencyMinutes).Minutes().Run(mds.sendReplyLoop); err != nil {
		log.Errorf("Unable to schedule send failed reply job. %v", err)
	}
	go mds.listenReply()
	return
}

// PostProcessorInitialization registers executes PostProcessorInitialization operations
// Will be executed after the processor initialization is done in MessageService
// Currently we use this only for command processors/document worker
func (mds *MDSInteractor) PostProcessorInitialization(worker utils.WorkerName) {
	switch worker {
	case utils.DocumentWorkerName:
		mds.postCommandProcessorInitialization()
	default:
	}
}

// PreProcessorClose defines operations to be performed before processor close
// Before command worker processor close, we try to close the message polling and send failed reply job in this function
func (mds *MDSInteractor) PreProcessorClose() {
	log := mds.context.Log()
	log.Debugf("pre close mds interactor :%v", Name)
	for _, worker := range mds.GetSupportedWorkers() {
		switch worker {
		case utils.DocumentWorkerName:
			mds.preDocumentProcessorClose()
		default:
		}
	}
	return
}

// Close closes connection. The closing operations for MDS interactor is done in BeforeClose itself.
// Hence, this function does not operation now.
func (mds *MDSInteractor) Close() error {
	// at this point, processor would have been closed
	mds.context.Log().Infof("%v closed", Name)
	return nil
}

// private functions

// postCommandProcessorInitialization is the post initialization handler which will get executed after the CommandProcessor is launched in the MessageHandler.
// this function basically schedules messagePollLoop
func (mds *MDSInteractor) postCommandProcessorInitialization() {
	mds.startMDSPollingJob()
	// This goroutine will be closed when the channel is closed in MGS Interactor
	go mds.mdsSwitcher()
	return
}

// listenReply listens to the replies and pushes to the reply queue
func (mds *MDSInteractor) listenReply() {
	log := mds.context.Log()
	log.Info("listen reply thread started")
	defer func() {
		log.Info("listen reply thread ended")
		if r := recover(); r != nil {
			log.Errorf("listen reply panicked: \n%v", r)
			log.Errorf("stacktrace:\n%s", debug.Stack())
			time.Sleep(5 * time.Second) // adding some delay here
			go mds.listenReply()
		}
	}()
	for result := range mds.replyChan {
		log.Debugf("start processing reply: %v", result.MessageID)
		pluginID := result.LastPlugin
		payloadDoc := messageContracts.SendReplyPayload{}

		if mds.ableToOpenMGSConnection != nil {
			ableToOpenMGSConnection := atomic.LoadUint32(mds.ableToOpenMGSConnection) != 0
			payloadDoc = utils.PrepareReplyPayloadFromIntermediatePluginResults(mds.context.Log(), pluginID, mds.config.AgentInfo, result.PluginResults, &ableToOpenMGSConnection)
		} else {
			payloadDoc = utils.PrepareReplyPayloadFromIntermediatePluginResults(mds.context.Log(), pluginID, mds.config.AgentInfo, result.PluginResults, nil)
		}

		mds.processSendReply(result.MessageID, payloadDoc)
		log.Debugf("ended processing reply: %v", result.MessageID)
	}
}

// mdsSwitcher is responsible for turning on and off MDS based on MGS status.
// The decision logic can be found in ssmconnectionchannel module
func (mds *MDSInteractor) mdsSwitcher() {
	log := mds.context.Log()
	defer func() {
		log.Info("MdsSwitcher thread ended")
		if r := recover(); r != nil {
			log.Errorf("MdsSwitcher panicked: \n%v", r)
			log.Errorf("stacktrace:\n%s", debug.Stack())
		}
	}()
	isWindows2012OrEarlier, isPlatformWindowsServer2012OrEarlierErr := isPlatformWindowsServer2012OrEarlier(log)
	if isPlatformWindowsServer2012OrEarlierErr != nil {
		log.Errorf("Unable to determine if platform is Windows Server 2012 or earlier: %v", isPlatformWindowsServer2012OrEarlierErr)
	}
	for mdsTurnOnFlag := range ssmconnectionchannel.GetMDSSwitchChannel() {
		// document with updateAgent plugin comes only via MDS.
		// Hence, the switch won't happen for Windows 2012 as this plugin is supported only for this platform.
		if isWindows2012OrEarlier || isPlatformWindowsServer2012OrEarlierErr != nil {
			log.Info("Single message channel will be disabled for Windows 2012 and earlier")
			continue
		}
		if mdsTurnOnFlag {
			mds.startMDSPollingJob()
		} else {
			mds.stopMDSPollingJob()
		}
	}
}

// startMDSPollingJob starts MDS long polling job
// This function is thread safe
func (mds *MDSInteractor) startMDSPollingJob() {
	mds.mdsStateMutex.Lock()
	defer mds.mdsStateMutex.Unlock()
	log := mds.context.Log()

	// If MDS is in shutdown status, do not switch ON MDS again
	if mds.mdsState == MDSShutDown {
		log.Info("MDS will not be started as it is already in shutdown status.")
		return
	}

	// Wait until the stop job is done
	mds.stopPollingJobGrp.Wait()

	mds.mdsState = MDSStartInProgress
	var err error
	mds.messagePollJob, err = scheduler.Every(pollMessageFrequencyMinutes).Minutes().Run(mds.messagePollLoop)
	mds.mdsState = MDSStartCompleted
	if err != nil {
		// We do not retry during any errors. Sticking to current behavior
		log.Errorf("MDS Polling job started with errors. %v", err)
		return
	}
	log.Info("MDS Polling job started.")
}

// stopMDSPollingJob stops MDS long polling job
func (mds *MDSInteractor) stopMDSPollingJob() {

	mds.context.Log().Info("MDS Polling stop job started.")
	// If MDS is in shutdown status, do not switch OFF MDS again
	if mds.getMDSState() == MDSShutDown {
		mds.context.Log().Info("MDS will not be stopped again as it is already in shutdown status.")
		return
	}

	mds.stopPollingJobGrp.Add(1)
	mds.setMDSState(MDSStopInProgress)

	// stop only when start completed
	go func() {
		defer func() {
			if r := recover(); r != nil {
				mds.context.Log().Errorf("StopMDSPollingJob panic: %v", r)
				mds.context.Log().Errorf("Stacktrace:\n%s", debug.Stack())
			}
			mds.setMDSState(MDSStopCompleted)
			mds.stopPollingJobGrp.Done()
		}()

		// We have this 1 min interval to make sure that we give an opportunity for agent to pull from MDS before shutting down MDS.
		select {
		case <-timeAfter(1 * time.Minute):
			mds.context.Log().Info("Moving to stop poll job after a minute")
			break
		case <-mds.stopJobChannel:
			mds.context.Log().Info("Received stop poll job request")
			return
		}

		if mds.messagePollJob != nil {
			// will stop the scheduler. In progress job will stop in next run
			mds.messagePollJob.Quit <- true
			mds.context.Log().Info("Sent termination signal to message poll job.")
		}
		mds.context.Log().Info("MDS Polling job stopped.")
	}()
}

func (mds *MDSInteractor) setMDSState(state MDSState) {
	mds.mdsStateMutex.Lock()
	defer mds.mdsStateMutex.Unlock()
	mds.mdsState = state
}

func (mds *MDSInteractor) getMDSState() MDSState {
	mds.mdsStateMutex.Lock()
	defer mds.mdsStateMutex.Unlock()
	return mds.mdsState
}

func (mds *MDSInteractor) sendStopSignalTsoStopJob() {
	select {
	case mds.stopJobChannel <- struct{}{}:
		break
	default:
		break
	}
}

// preDocumentProcessorClose does operations based on pre
func (mds *MDSInteractor) preDocumentProcessorClose() {
	log := mds.context.Log()
	log.Debugf("pre-closing %v", Name)

	mds.setMDSState(MDSShutDown)

	// Stop polling job takes 1 min to complete. This signal preempts this job to stop the agent quickly
	mds.sendStopSignalTsoStopJob()

	// get current MDS state
	currentState := mds.getMDSState()

	// If stop already triggered, do not initiate stop again
	if currentState == MDSStopInProgress || currentState == MDSStopCompleted {
		mds.stopPollingJobGrp.Wait()
	} else {
		// stop MDS long polling job
		mds.stopMDSPollingJob()
	}

	if mds.sendReplyJob != nil {
		mds.sendReplyJob.Quit <- true
	}

	// Stop any ongoing calls
	mds.service.Stop()

	// Wait for ongoing messagePoll loops to terminate
	log.Debugf("waiting for polling function to return")
	mds.messagePollWaitGroup.Wait()
	log.Debugf("message poll wait end in %v", Name)

}

// loop reads messages from MDS then processes them.
func (mds *MDSInteractor) messagePollLoop() {
	log := mds.context.Log()
	defer func() {
		if msg := recover(); msg != nil {
			log.Errorf("message poll loop panic: %v", msg)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()
	mds.messagePollWaitGroup.Add(1)
	defer mds.messagePollWaitGroup.Done()
	// time lock to only have one loop active anytime.
	// this is extra insurance to prevent any race condition
	pollStartTime := time.Now()
	log.Debug("Starting message poll")

	mds.updateLastPollTime(pollStartTime)

	if err := mds.checkStopPolicy(); err != nil {
		return
	}

	mds.pollOnce()
	log.Debugf("%v's stoppolicy after polling is %v", Name, mds.processorStopPolicy)

	// Slow down a bit in case GetMessages returns
	// without blocking, which may cause us to
	// flood the service with requests.
	if time.Since(pollStartTime) < time.Second {
		time.Sleep(time.Duration(2000+rand.Intn(500)) * time.Millisecond)
	}

	// check if any other poll loop has started in the meantime
	// to prevent any possible race condition due to the scheduler
	if pollStartTime.Equal(mds.getLastPollTime()) {
		// skip waiting for the next scheduler polling event and start polling immediately
		mds.messagePollJob.SkipWait <- true
	} else {
		log.Debugf("Other message poll already started at %v, scheduler wait will not be skipped", mds.getLastPollTime())
	}
}

func (mds *MDSInteractor) getLastPollTime() time.Time {
	mds.mutex.RLock()
	defer mds.mutex.RUnlock()
	return mds.lastPollTime
}

func (mds *MDSInteractor) updateLastPollTime(currentTime time.Time) {
	mds.mutex.Lock()
	defer mds.mutex.Unlock()
	mds.lastPollTime = currentTime
}

func (mds *MDSInteractor) processMessage(msg *ssmmds.Message) {
	var (
		docState *contracts.DocumentState
		err      error
	)

	// create separate logger that includes messageID with every log message
	mdsContext := mds.context.With("[messageID=" + *msg.MessageId + "]")
	log := mdsContext.Log()
	log.Debug("Processing message")
	if err = utils.Validate(msg); err != nil {
		log.Error("message not valid, ignoring: ", err)
		return
	}

	if strings.HasPrefix(*msg.Topic, string(utils.SendCommandTopicPrefix)) {
		docState, err = utils.ParseSendCommandMessage(mdsContext, toInstanceMessage(msg), mds.orchestrationRootDir, contracts.MessageDeliveryService)
		if err != nil {
			log.Error(err)
			mds.sendDocLevelResponse(*msg.MessageId, contracts.ResultStatusFailed, err.Error())
			return
		}
	} else if strings.HasPrefix(*msg.Topic, string(utils.CancelCommandTopicPrefix)) {
		docState, err = utils.ParseCancelCommandMessage(mdsContext, toInstanceMessage(msg), contracts.MessageDeliveryService)
	} else {
		err = fmt.Errorf("unexpected topic name %v", *msg.Topic)
	}

	// Fail on invalid message
	if err != nil {
		log.Error("format of received message is invalid ", err)
		if err = mds.service.FailMessage(log, *msg.MessageId, mdsService.InternalHandlerException); err != nil {
			sdkutil.HandleAwsError(log, err, mds.processorStopPolicy)
		}
		return
	}

	errorCode := mds.messageHandler.Submit(docState)

	// showLog is used to minimize warn log during ProcessorBufferFull error
	// this makes sure that warn message is showed only once
	showLog := true
	// sleep until the processor frees up.
	// added to minimize the long polling frequency during this case
	for errorCode == messageHandler.ProcessorBufferFull {
		if showLog {
			log.Warnf("skipping document %v due to the error: %v. Will wake up every 10 seconds till the buffer is free", docState.DocumentInformation.MessageID, errorCode)
			showLog = false
		} else {
			log.Tracef("skipping document %v due to the error: %v", docState.DocumentInformation.MessageID, errorCode)
		}
		time.Sleep(10 * time.Second)
		errorCode = mds.messageHandler.Submit(docState)
	}

	// we skip for the following error codes
	if _, ok := mds.ackSkipCodes[errorCode]; ok {
		log.Warnf("skipped document %v due to the error: %v", docState.DocumentInformation.MessageID, errorCode)
		return
	}
	log.Debugf("Pushed document type %v to channel for processing", docState.DocumentType)

	log.Debug("Processing to send a reply to update the document status to InProgress")
	mds.sendDocLevelResponse(*msg.MessageId, contracts.ResultStatusInProgress, "")

	// Ack valid message
	// TODO: check if the message is scheduled, otherwise throw error back to MDS
	if err = mds.service.AcknowledgeMessage(log, *msg.MessageId); err != nil {
		sdkutil.HandleAwsError(log, err, mds.processorStopPolicy)
		return
	}
	log.Debugf("Ack done. Received message - messageId - %v", *msg.MessageId)
}

func (mds *MDSInteractor) checkStopPolicy() (err error) {
	log := mds.context.Log()
	if mds.processorStopPolicy == nil {
		log.Debug("creating new stop-policy.")
		mds.processorStopPolicy = newStopPolicy(Name)
		return
	}

	log.Debugf("%v's stoppolicy before polling is %v", Name, mds.processorStopPolicy)
	if mds.processorStopPolicy.IsHealthy() == false {
		err := fmt.Errorf("%v stopped temporarily due to internal failure. We will retry automatically after %v minutes", Name, pollMessageFrequencyMinutes)
		log.Errorf("%v", err)
		mds.reset()
	}
	return
}

// pollOnce calls GetMessages once and processes the result.
func (mds *MDSInteractor) pollOnce() {
	log := mds.context.Log()
	log.Debug("Polling for messages")
	messages, err := mds.service.GetMessages(log, mds.config.InstanceID)
	if err != nil {
		sdkutil.HandleAwsError(log, err, mds.processorStopPolicy)
		return
	}

	if len(messages.Messages) > 0 {
		log.Debugf("Got %v messages", len(messages.Messages))
	}

	for _, msg := range messages.Messages {
		mds.processMessage(msg)
	}
	log.Debugf("Finished message poll")
}

// loop sends replies to MDS
func (mds *MDSInteractor) sendReplyLoop() {
	log := mds.context.Log()
	defer func() {
		if msg := recover(); msg != nil {
			log.Errorf("sendFailedReplies panicked: %v", msg)
			log.Errorf("stacktrace:\n%s", debug.Stack())
		}
	}()
	if err := mds.checkStopPolicy(); err != nil {
		return
	}

	mds.sendFailedReplies()

	log.Debugf("%v's stoppolicy after polling is %v", Name, mds.processorStopPolicy)
}

// sendFailedReplies loads replies from local disk and send it again to the service, if it fails no action is needed
func (mds *MDSInteractor) sendFailedReplies() {
	log := mds.context.Log()

	log.Debug("Checking if there are document replies that failed to reach the service, and retry sending them")
	replies := mds.service.LoadFailedReplies(log)

	if len(replies) == 0 {
		log.Debug("No failed document replies found")
		return
	}

	log.Infof("Found document replies that need to be sent to the service")
	for _, reply := range replies {
		log.Debug("Loading reply ", reply)
		if utils.IsValidReplyRequest(reply, contracts.MessageDeliveryService) == false {
			log.Debug("Reply is old, document execution must have timed out. Deleting the reply")
			mds.service.DeleteFailedReply(log, reply)
			continue
		}
		sendReplyRequest, err := mds.service.GetFailedReply(log, reply)
		if err != nil {
			log.Error("Couldn't load the reply from disk ", err)
			continue
		}

		log.Info("Sending reply ", reply)
		if err = mds.service.SendReplyWithInput(log, sendReplyRequest); err != nil {
			sdkutil.HandleAwsError(log, err, mds.processorStopPolicy)
			break
		}
		log.Infof("Sending reply %v succeeded, deleting the reply file from disk", reply)
		mds.service.DeleteFailedReply(log, reply)
	}
}

func (mds *MDSInteractor) sendDocLevelResponse(messageID string, resultStatus contracts.ResultStatus, documentTraceOutput string) {
	payloadDoc := messageContracts.SendReplyPayload{}

	if mds.ableToOpenMGSConnection != nil {
		ableToOpenMGSConnection := atomic.LoadUint32(mds.ableToOpenMGSConnection) != 0
		payloadDoc = utils.PrepareReplyPayloadToUpdateDocumentStatus(mds.config.AgentInfo, resultStatus, documentTraceOutput, &ableToOpenMGSConnection)
	} else {
		payloadDoc = utils.PrepareReplyPayloadToUpdateDocumentStatus(mds.config.AgentInfo, resultStatus, documentTraceOutput, nil)
	}

	mds.processSendReply(messageID, payloadDoc)
}

func (mds *MDSInteractor) reset() {
	log := mds.context.Log()
	log.Debugf("Resetting processor:%v", Name)
	// reset stop policy and let the scheduler start the polling after pollMessageFrequencyMinutes timeout
	mds.processorStopPolicy.ResetErrorCount()

	// creating a new mds service object for the retry
	// this is extra insurance to avoid service object getting corrupted - adding resiliency
	mds.service = newMdsService(mds.context)
}

func (mds *MDSInteractor) processSendReply(messageID string, payloadDoc messageContracts.SendReplyPayload) {
	log := mds.context.Log()
	payloadB, err := json.Marshal(payloadDoc)
	if err != nil {
		log.Error("could not marshal reply payload!", err)
		return
	}
	payload := string(payloadB)
	log.Info("Sending reply ", jsonutil.Indent(payload))
	if err = mds.service.SendReply(log, messageID, payload); err != nil {
		sdkutil.HandleAwsError(log, err, mds.processorStopPolicy)
	}
}

var newMdsService = func(context context.T) mdsService.Service {
	connectionTimeout := time.Duration(context.AppConfig().Mds.StopTimeoutMillis) * time.Millisecond
	return mdsService.NewService(
		context,
		connectionTimeout,
	)
}

var newStopPolicy = func(name string) *sdkutil.StopPolicy {
	return sdkutil.NewStopPolicy(name, stopPolicyErrorThreshold)
}

var toInstanceMessage = func(msg *ssmmds.Message) model.InstanceMessage {
	return model.InstanceMessage{
		CreatedDate: *msg.CreatedDate,
		Destination: *msg.Destination,
		MessageId:   *msg.MessageId,
		Payload:     *msg.Payload,
		Topic:       *msg.Topic,
	}
}
