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

// Package messagehandler defines methods to be used by Interactors for submission of commands to the processors through ProcessorWrappers
// It also forwards the replies receives from processor wrapper
package messagehandler

import (
	"runtime/debug"
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/messagehandler/idempotency"
	processorWrapperTypes "github.com/aws/amazon-ssm-agent/agent/messageservice/messagehandler/processorwrappers"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/utils"
	"github.com/carlescere/scheduler"
)

type IMessageHandler interface {
	GetName() string
	Initialize() error
	InitializeAndRegisterProcessor(proc processorWrapperTypes.IProcessorWrapper) error
	RegisterReply(name contracts.UpstreamServiceName, reply chan contracts.DocumentResult)
	Submit(message *contracts.DocumentState) ErrorCode
	Stop() (err error)
}

// MessageHandler defines methods to be used by Interactors for submission of commands to the processors through ProcessorWrappers
type MessageHandler struct {
	context                         context.T
	agentConfig                     contracts.AgentConfiguration
	name                            string
	persistedCommandDeletionJob     *scheduler.Job
	replyMap                        map[contracts.UpstreamServiceName]chan contracts.DocumentResult
	docTypeProcessorFuncMap         map[contracts.DocumentType]processorWrapperTypes.IProcessorWrapper
	processorsLoaded                map[utils.ProcessorName]processorWrapperTypes.IProcessorWrapper
	resultChan                      chan contracts.DocumentResult
	mhMutex                         sync.Mutex
	processorMsgHandlerErrorCodeMap map[processor.ErrorCode]ErrorCode
}

type ErrorCode string

const (
	// Name represents name of the service
	Name = "MessageHandler"

	// UnexpectedDocumentType represents that message handler received unexpected document type
	UnexpectedDocumentType ErrorCode = "UnexpectedDocumentType"

	// idempotencyFileDeletionTimeout represents file deletion timeout after persisting command for idempotency
	idempotencyFileDeletionTimeout = 10

	// ProcessorBufferFull represents that the processor buffer is full
	ProcessorBufferFull ErrorCode = "ProcessorBufferFull"

	// ClosedProcessor represents that the processor is closed
	ClosedProcessor ErrorCode = "ClosedProcessor"

	// ProcessorErrorCodeTranslationFailed represents that the processor to message handler error code translation failed
	ProcessorErrorCodeTranslationFailed ErrorCode = "ProcessorErrorCodeTranslationFailed"

	// DuplicateCommand represents duplicate command in the buffer
	DuplicateCommand ErrorCode = "DuplicateCommand"

	// InvalidDocument represents invalid document received in processor
	InvalidDocument ErrorCode = "InvalidDocument"
)

// NewMessageHandler returns new message handler
func NewMessageHandler(context context.T) IMessageHandler {
	messageContext := context.With("[" + Name + "]")
	messageHandler := &MessageHandler{
		context:                 messageContext,
		name:                    Name,
		replyMap:                make(map[contracts.UpstreamServiceName]chan contracts.DocumentResult),
		docTypeProcessorFuncMap: make(map[contracts.DocumentType]processorWrapperTypes.IProcessorWrapper),
		processorsLoaded:        make(map[utils.ProcessorName]processorWrapperTypes.IProcessorWrapper),
	}
	return messageHandler
}

// GetName returns the name
func (mh *MessageHandler) GetName() string {
	return mh.name
}

// Initialize initializes MessageHandler
func (mh *MessageHandler) Initialize() (err error) {
	logger := mh.context.Log()
	logger.Info("initializing message handler")
	mh.processorMsgHandlerErrorCodeMap = map[processor.ErrorCode]ErrorCode{
		processor.CommandBufferFull:  ProcessorBufferFull,
		processor.ClosedProcessor:    ClosedProcessor,
		processor.DuplicateCommand:   DuplicateCommand,
		processor.InvalidDocumentId:  InvalidDocument,
		processor.UnsupportedDocType: UnexpectedDocumentType,
	}
	if mh.persistedCommandDeletionJob == nil {
		if mh.persistedCommandDeletionJob, err = scheduler.Every(idempotencyFileDeletionTimeout).Minutes().NotImmediately().Run(func() {
			mh.context.Log().Info("started idempotency deletion thread")
			defer func() {
				mh.context.Log().Infof("ended idempotency deletion thread")
				if msg := recover(); msg != nil {
					mh.context.Log().Errorf("cleanup entries in idempotency panicked: %v", msg)
					mh.context.Log().Errorf("stacktrace:\n%s", debug.Stack())
				}
			}()
			idempotency.CleanupOldIdempotencyEntries(mh.context)
		}); err != nil {
			logger.Errorf("unable to schedule idempotency file deletion job - %v", err)
		}
	}
	return nil
}

// Submit submits the command to the processor wrapper
func (mh *MessageHandler) Submit(message *contracts.DocumentState) ErrorCode {
	log := mh.context.Log()
	log.Debugf("submit incoming message %v", message.DocumentInformation.MessageID)
	mhErrorCode := ErrorCode("") // Success
	// safety panic handler
	defer func() {
		if msg := recover(); msg != nil {
			mh.context.Log().Errorf("message handler submit panicked: %v", msg)
			mh.context.Log().Errorf("stacktrace:\n%s", debug.Stack())
		}
	}()
	if proc, ok := mh.docTypeProcessorFuncMap[message.DocumentType]; ok {
		errorCode := proc.PushToProcessor(*message)
		if errorCode != "" {
			if msgErrorCode, ok := mh.processorMsgHandlerErrorCodeMap[errorCode]; ok {
				mhErrorCode = msgErrorCode
			} else {
				mhErrorCode = ProcessorErrorCodeTranslationFailed
			}
		}
	} else {
		mhErrorCode = UnexpectedDocumentType
	}
	return mhErrorCode
}

// InitializeAndRegisterProcessor registers processors from Message service
// Should be called before Initialization being called
func (mh *MessageHandler) InitializeAndRegisterProcessor(proc processorWrapperTypes.IProcessorWrapper) error {
	newProc := mh.registerProcessor(proc)
	// loads all pending and in-progress documents
	// this is a blocking call until the documents are loaded fully
	// we intentionally call the same processor twice, to block the MGS agent job incoming messages during pending and in-progress document execution.
	if err := newProc.Initialize(mh.replyMap); err != nil {
		return err
	}
	mh.registerDocTypeWithProcessor(newProc)
	return nil
}

func (mh *MessageHandler) registerDocTypeWithProcessor(proc processorWrapperTypes.IProcessorWrapper) {
	mh.mhMutex.Lock()
	defer mh.mhMutex.Unlock()
	mh.docTypeProcessorFuncMap[proc.GetStartWorker()] = proc
	mh.docTypeProcessorFuncMap[proc.GetTerminateWorker()] = proc
}

func (mh *MessageHandler) registerProcessor(proc processorWrapperTypes.IProcessorWrapper) processorWrapperTypes.IProcessorWrapper {
	mh.mhMutex.Lock()
	defer mh.mhMutex.Unlock()
	if procVal, ok := mh.processorsLoaded[proc.GetName()]; ok {
		return procVal
	}
	// two different maps are used for performance reasons
	mh.processorsLoaded[proc.GetName()] = proc
	return proc
}

// RegisterReply registers the reply to the MessageHandler
func (mh *MessageHandler) RegisterReply(name contracts.UpstreamServiceName, reply chan contracts.DocumentResult) {
	mh.mhMutex.Lock()
	defer mh.mhMutex.Unlock()
	mh.replyMap[name] = reply
}

// Stop stops the message handlers
func (mh *MessageHandler) Stop() (err error) {
	log := mh.context.Log()
	log.Infof("stopping %s.", Name)
	var wg sync.WaitGroup
	for _, processorObj := range mh.processorsLoaded {
		wg.Add(1)
		go func(processorObj processorWrapperTypes.IProcessorWrapper) {
			defer func() {
				wg.Done()
				if msg := recover(); msg != nil {
					log.Errorf("message handler stop run panic: %v", msg)
					log.Errorf("stacktrace:\n%s", debug.Stack())
				}
			}()

			processorObj.Stop()
		}(processorObj)
	}
	wg.Wait()

	// closes the registered reply channels
	for _, replyChan := range mh.replyMap {
		close(replyChan)
	}

	return nil
}
