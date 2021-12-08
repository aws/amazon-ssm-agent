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

// Package messageservice will be responsible for initializing MDS and MGS interactors and then
// launch message handlers to handle the commands received from interactors.
// This package is the starting point for the message service module.
package messageservice

import (
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/interactor"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/interactor/mdsinteractor"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/messagehandler"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/messagehandler/processorwrappers"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/utils"
	"github.com/aws/amazon-ssm-agent/agent/platform"
)

var (
	isPlatformNanoServer           = platform.IsPlatformNanoServer
	getProcessorWrapperDelegateMap = processorwrappers.GetProcessorWrapperDelegateMap
)

const (
	// ServiceName represents MessageService ICoreModule name
	ServiceName = "MessageService"
)

// MessageService is the core module for initializing MDS and MGS interactors
// and then launch message handlers which in turn initiates processors
// to process the commands from interactors
type MessageService struct {
	context         context.T
	name            string
	messageHandler  messagehandler.IMessageHandler
	interactors     []interactor.IInteractor
	msgServiceMutex sync.Mutex
}

// NewService instantiates MessageService object and assigns value if needed
func NewService(context context.T) contracts.ICoreModule {
	messageContext := context.With("[" + ServiceName + "]")
	log := messageContext.Log()

	instanceID, err := context.Identity().InstanceID()
	if instanceID == "" {
		log.Errorf("no instanceID provided, %s", err)
		return nil
	}

	messageService := &MessageService{
		context:        messageContext,
		name:           ServiceName,
		messageHandler: messagehandler.NewMessageHandler(messageContext),
	}

	/*isNanoServer, _ := isPlatformNanoServer(log)
	if !isNanoServer {
		messageService.interactors = append(messageService.interactors, mgsinteractor.New(messageContext, messageService.messageHandler))
	}*/
	if !messageContext.AppConfig().Agent.ContainerMode {
		mdsRef, err := mdsinteractor.New(messageContext, messageService.messageHandler)
		if err == nil {
			messageService.interactors = append(messageService.interactors, mdsRef)
		}
	}

	return messageService
}

// ICoreModule implementation

// ModuleName returns the name of module
func (msgSvc *MessageService) ModuleName() string {
	return msgSvc.name
}

// ModuleExecute starts the MessageService module
func (msgSvc *MessageService) ModuleExecute() (err error) {
	log := msgSvc.context.Log()

	// initialize message handler
	msgSvc.messageHandler.Initialize()

	var wg sync.WaitGroup
	errArr := make([]error, 0)
	for _, interactRef := range msgSvc.interactors {
		// this is a safety check
		if interactRef == nil {
			log.Error("skipping as the loaded interactor is nil")
			return
		}
		wg.Add(1)
		go func(interactor interactor.IInteractor) {
			interactorName := interactor.GetName()
			log.Infof("%v initialization started", interactorName)
			defer func() {
				wg.Done()
				log.Infof("%v initialization completed", interactorName)
				if msg := recover(); msg != nil {
					log.Errorf("%v initialization panicked: %v", interactorName, msg)
					log.Errorf("stacktrace:\n%s", debug.Stack())
				}
			}()
			// In MGS Interactor, control channel connection may retry indefinitely
			// This will be blocked during that case
			if err = interactor.Initialize(); err != nil {
				errorMsg := fmt.Errorf("error occurred while initializing Interactor %v: %v", interactorName, err)
				log.Error(errorMsg)
				errArr = append(errArr, errorMsg)
				return
			}

			supportedWorkers := interactor.GetSupportedWorkers()
			log.Infof("supported workers for the interactor %v: %v", interactorName, supportedWorkers)

			// initializes and registers the processor with message handler
			msgSvc.initializeProcessor(interactor, supportedWorkers)
		}(interactRef)
	}
	wg.Wait()
	if len(errArr) != 0 {
		return fmt.Errorf("message service module execution threw error: %v", errArr)
	}
	return err
}

// initializeProcessor initializes and registers the processors with message handler required by interactors.
// Also, opens up the connections to receive commands after initialization is done
func (msgSvc *MessageService) initializeProcessor(interactorRef interactor.IInteractor, supportedWorkers []utils.WorkerName) {
	log := msgSvc.context.Log()

	processorWorkerConfigs := utils.LoadProcessorWorkerConfig(msgSvc.context)

	// we do not write anything in the delegates map once loaded
	// so concurrent read is fine
	processorWrapperDelegateMap := getProcessorWrapperDelegateMap()
	var wg sync.WaitGroup
	for _, workerName := range supportedWorkers {
		wg.Add(1)
		log.Infof("initialize processor started for worker %v belonging to interactor %v", workerName, interactorRef.GetName())
		go func(worker utils.WorkerName) {
			defer func() {
				wg.Done()
				log.Infof("initialize processor completed for worker %v belonging to interactor %v", workerName, interactorRef.GetName())
				if msg := recover(); msg != nil {
					log.Errorf("%v processor initialization panicked: %v", worker, msg)
					log.Errorf("stacktrace:\n%s", debug.Stack())
				}
			}()
			if _, ok := processorWorkerConfigs[worker]; !ok {
				log.Errorf("worker name not present in the config: %v", worker)
				return
			}
			if _, ok := processorWrapperDelegateMap[worker]; !ok {
				log.Errorf("processor wrapper delegate not available for the worker name: %v", worker)
				return
			}
			procWrapper := processorWrapperDelegateMap[worker](msgSvc.context, processorWorkerConfigs[worker])
			procWrapperName := procWrapper.GetName()
			log.Infof("registering processor %v for the interactor: %v", procWrapperName, interactorRef.GetName())
			// When we try to re-register processor wrapper with same name, InitializeAndRegisterProcessor blocks until the first registered processor initialization is done.
			// This is done intentionally to make sure that we do not open connections to receive commands when in-progress and pending commands are not yet loaded
			err := msgSvc.messageHandler.InitializeAndRegisterProcessor(procWrapper)
			if err != nil {
				log.Warnf("error during initialization of processor wrapper %v: %v", procWrapperName, err) // No error is returned for now
			}
			// post processor initialization opens agent incoming job messages in the interactors after initialization of processor id done
			interactorRef.PostProcessorInitialization()
		}(workerName)
	}
	wg.Wait()
}

// ModuleRequestStop stops the MessageService module
func (msgSvc *MessageService) ModuleRequestStop(stopType contracts.StopType) error {
	log := msgSvc.context.Log()
	log.Infof("Stopping %v", msgSvc.name)
	var err error
	for _, interactRef := range msgSvc.interactors {
		// Perform actions to do by interactors before the message handler close
		// This function performs following operations:
		// close send failed reply job and message polling in MDS interactor
		// drop incoming agent job messages in MGS interactor
		interactRef.PreProcessorClose()
	}
	// close the launched processors
	err = msgSvc.messageHandler.Stop(stopType)
	if err != nil {
		log.Errorf("error occurred during closing message handlers: %v", err)
	}

	var wg sync.WaitGroup
	for _, interactRef := range msgSvc.interactors {
		// no action in MDS interactor
		// control channel is closed
		wg.Add(1)
		go func(interactor interactor.IInteractor) {
			defer wg.Done()
			closeErr := interactor.Close()
			if closeErr != nil {
				log.Errorf("error occurred while closing connection in interactor %v: %v", interactor.GetName(), closeErr)
			}
		}(interactRef)
	}
	wg.Wait()
	log.Infof("Stopped %v", msgSvc.name)
	return nil
}
