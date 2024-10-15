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

// Package processorwrappers implements different processor wrappers to handle the processors which launches
// document worker and session worker for now
package processorwrappers

import (
	"runtime/debug"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/docmanager"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/utils"
)

// NewSessionWorkerProcessorWrapper initiates new processor wrapper which supports session workers
func NewSessionWorkerProcessorWrapper(context context.T, worker *utils.ProcessorWorkerConfig) IProcessorWrapper {
	processorContext := context.With("[" + string(worker.ProcessorName) + "Wrapper" + "]")

	startSessionWorker := processor.NewWorkerProcessorSpec(context, worker.StartWorkerLimit, worker.StartWorkerDocType, worker.StartWorkerBufferLimit)
	terminateSessionWorker := processor.NewWorkerProcessorSpec(context, worker.CancelWorkerLimit, worker.CancelWorkerDocType, worker.CancelWorkerBufferLimit)
	sessionProcessor := processor.NewEngineProcessor(
		context,
		startSessionWorker,
		terminateSessionWorker)

	processorDoc := &SessionWorkerProcessorWrapper{
		context:   processorContext,
		processor: sessionProcessor,
		name:      worker.ProcessorName,
	}
	processorDoc.startWorkerCmd = startSessionWorker.GetAssignedDocType()
	processorDoc.cancelWorkerCmd = terminateSessionWorker.GetAssignedDocType()
	return processorDoc
}

// SessionWorkerProcessorWrapper defines properties and methods to interact with the processor launched for the session worker
type SessionWorkerProcessorWrapper struct {
	context           context.T
	processor         processor.Processor
	commandResultChan chan contracts.DocumentResult
	name              utils.ProcessorName
	startWorkerCmd    contracts.DocumentType
	cancelWorkerCmd   contracts.DocumentType
	listenReplyEnded  chan struct{}
}

// Initialize initializes session processor and launches the reply thread
func (spw *SessionWorkerProcessorWrapper) Initialize(outputChan map[contracts.UpstreamServiceName]chan contracts.DocumentResult) error {
	var err error
	spw.commandResultChan, err = spw.processor.Start()
	if err != nil {
		return err
	}
	go spw.listenSessionReply(spw.commandResultChan, outputChan)
	err = spw.processor.InitialProcessing(false)
	if err != nil {
		return err
	}
	spw.listenReplyEnded = make(chan struct{}, 1)
	return nil
}

// GetName gets the name of the processor wrapper
func (spw *SessionWorkerProcessorWrapper) GetName() utils.ProcessorName {
	return spw.name
}

// GetStartWorker gets the command which launches the session worker in processor
func (spw *SessionWorkerProcessorWrapper) GetStartWorker() contracts.DocumentType {
	return spw.startWorkerCmd
}

// PushToProcessor submits the command to the processor
func (spw *SessionWorkerProcessorWrapper) PushToProcessor(message contracts.DocumentState) processor.ErrorCode {
	errorCode := processor.UnsupportedDocType
	if spw.startWorkerCmd == message.DocumentType {
		errorCode = spw.processor.Submit(message)
	} else if spw.cancelWorkerCmd == message.DocumentType {
		errorCode = spw.processor.Cancel(message)
	}
	return errorCode
}

// GetTerminateWorker gets the command which cancels the session worker in processor
func (spw *SessionWorkerProcessorWrapper) GetTerminateWorker() contracts.DocumentType {
	return spw.cancelWorkerCmd
}

// Stop stops the processor
func (spw *SessionWorkerProcessorWrapper) Stop() {
	spw.processor.Stop()
}

// listenSessionReply listens to document result from the executor and pushes to outputChan.
// outputChan is used by message handler to push it to the correct interacts
func (spw *SessionWorkerProcessorWrapper) listenSessionReply(resultChan chan contracts.DocumentResult, outputChan map[contracts.UpstreamServiceName]chan contracts.DocumentResult) {
	spw.listenReplyEnded = make(chan struct{}, 1)
	log := spw.context.Log()
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Listen reply panic: \n%v", r)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()
	log.Info("listening session reply.")

	// processor guarantees to close this channel upon stop
externalLabel:
	for {
		select {
		case res, resChannelOpen := <-resultChan:
			if !resChannelOpen {
				log.Infof("listen reply channel closed")
				break externalLabel
			}
			if res.LastPlugin != "" {
				log.Infof("received plugin: %s result from Processor", res.LastPlugin)
			} else {
				log.Infof("session: %s complete", res.MessageID)

				//Deleting Old Log Files
				shortInstanceID, _ := spw.context.Identity().ShortInstanceID()
				go docmanager.DeleteSessionOrchestrationDirectories(log,
					shortInstanceID,
					spw.context.AppConfig().Agent.OrchestrationRootDir,
					spw.context.AppConfig().Ssm.SessionLogsRetentionDurationHours)
			}
			// For SessionManager plugins, there is only one plugin in a document.
			// Send AgentTaskComplete when we get the plugin level result, and ignore this document level result.
			// For instance reboot scenarios, it only has document level result with "Failed" status, this result can't be ignored.
			if res.LastPlugin == "" && res.Status != contracts.ResultStatusFailed {
				continue
			}
			// assign result type here
			// this result type is used during send reply to create different reply type object
			res.ResultType = contracts.SessionResult
			if resultChanRef, ok := outputChan[contracts.MessageGatewayService]; ok {
				resultChanRef <- res
			} else {
				log.Errorf("dropping reply without pushing to any interactor %v", res.MessageID)
			}
		}
	}
	spw.listenReplyEnded <- struct{}{}
}
