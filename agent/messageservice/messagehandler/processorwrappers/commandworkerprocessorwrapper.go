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
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	associationProcessor "github.com/aws/amazon-ssm-agent/agent/association/processor"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/docmanager"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/messagehandler/idempotency"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/utils"
	runCommandContracts "github.com/aws/amazon-ssm-agent/agent/runcommand/contracts"
)

var (
	isDocumentAlreadyReceived = idempotency.IsDocumentAlreadyReceived
)

// NewCommandWorkerProcessorWrapper initiates new processor wrapper which supports document workers
func NewCommandWorkerProcessorWrapper(context context.T, worker *utils.ProcessorWorkerConfig) IProcessorWrapper {
	processorContext := context.With("[" + string(worker.ProcessorName) + "Wrapper" + "]")

	startWorker := processor.NewWorkerProcessorSpec(context, worker.StartWorkerLimit, worker.StartWorkerDocType, worker.StartWorkerBufferLimit)
	terminateWorker := processor.NewWorkerProcessorSpec(context, worker.CancelWorkerLimit, worker.CancelWorkerDocType, worker.CancelWorkerBufferLimit)
	commandProcessor := processor.NewEngineProcessor(
		context,
		startWorker,
		terminateWorker)

	processorDoc := &CommandWorkerProcessorWrapper{
		context:   processorContext,
		processor: commandProcessor,
		name:      worker.ProcessorName,
	}
	// MDS interactor will not be loaded for containers.
	// Since mgs interactor also created this processor wrapper, we will stop loading the Command Processor Wrapper in MGS interactor itself
	processorDoc.assocProcessor = associationProcessor.NewAssociationProcessor(context)
	processorDoc.startWorkerCmd = startWorker.GetAssignedDocType()
	processorDoc.cancelWorkerCmd = terminateWorker.GetAssignedDocType()
	return processorDoc
}

// CommandWorkerProcessorWrapper defines properties and methods to interact with the processor launched for the document worker
type CommandWorkerProcessorWrapper struct {
	context           context.T
	assocProcessor    *associationProcessor.Processor
	processor         processor.Processor
	commandResultChan chan contracts.DocumentResult
	name              utils.ProcessorName
	mutex             sync.Mutex
	startWorkerCmd    contracts.DocumentType
	cancelWorkerCmd   contracts.DocumentType
	listenReplyEnded  chan struct{}
}

// Initialize initializes command processor and launches the reply thread
func (cpw *CommandWorkerProcessorWrapper) Initialize(outputChan map[contracts.UpstreamServiceName]chan contracts.DocumentResult) error {
	var err error
	// using mutex here to handle re-registration case
	cpw.mutex.Lock()
	defer cpw.mutex.Unlock()
	if cpw.commandResultChan != nil {
		cpw.context.Log().Infof("processor already initialized %v", cpw.name)
		return nil
	}
	cpw.commandResultChan, err = cpw.processor.Start()
	if err != nil {
		return err
	}
	go cpw.listenReply(cpw.commandResultChan, outputChan)
	err = cpw.processor.InitialProcessing(true)
	if err != nil {
		return err
	}
	if cpw.assocProcessor != nil {
		cpw.assocProcessor.ModuleExecute()
	}
	cpw.listenReplyEnded = make(chan struct{}, 1)
	return nil
}

// GetName returns the name of the processor
func (cpw *CommandWorkerProcessorWrapper) GetName() utils.ProcessorName {
	return cpw.name
}

// GetStartWorker gets the command which launches the command worker in processor
func (cpw *CommandWorkerProcessorWrapper) GetStartWorker() contracts.DocumentType {
	return cpw.startWorkerCmd
}

// GetTerminateWorker gets the command which cancels the command worker in processor
func (cpw *CommandWorkerProcessorWrapper) GetTerminateWorker() contracts.DocumentType {
	return cpw.cancelWorkerCmd
}

// PushToProcessor submits the command to the processor
func (cpw *CommandWorkerProcessorWrapper) PushToProcessor(message contracts.DocumentState) processor.ErrorCode {
	errorCode := processor.UnsupportedDocType
	cpw.mutex.Lock()
	defer cpw.mutex.Unlock()
	commandPresent := isDocumentAlreadyReceived(cpw.context, &message)
	if commandPresent {
		return processor.DuplicateCommand
	}
	if cpw.startWorkerCmd == message.DocumentType {
		errorCode = cpw.processor.Submit(message)
	} else if cpw.cancelWorkerCmd == message.DocumentType {
		errorCode = cpw.processor.Cancel(message)
	}
	if errorCode == "" {
		err := idempotency.CreateIdempotencyEntry(cpw.context, &message)
		if err != nil {
			cpw.context.Log().Errorf("error while creating idempotency entry for command %v", message.DocumentInformation.DocumentID)
		}
	}
	return errorCode
}

// Stop stops the processor
func (cpw *CommandWorkerProcessorWrapper) Stop() {
	log := cpw.context.Log()
	// takes care of making sure that not jobs are pending in the job queue
	// this closes the result chan too.
	// should not expect many replies after this stop
	cpw.processor.Stop()
	if cpw.assocProcessor != nil {
		cpw.assocProcessor.ModuleStop()
	}
	select {
	case <-cpw.listenReplyEnded:
		log.Info("listen reply thread ended")
	case <-time.After(2 * time.Second): // this can be changed based on stopType in future.
		log.Info("listen reply thread close timed out")
	}
}

// listenReply listens to document result from the executor and pushes to outputChan.
// outputChan is used by messageHandler to push it to the correct interactor
func (cpw *CommandWorkerProcessorWrapper) listenReply(resChan chan contracts.DocumentResult, outputChan map[contracts.UpstreamServiceName]chan contracts.DocumentResult) {
	log := cpw.context.Log()
	cpw.listenReplyEnded = make(chan struct{}, 1)
	log.Info("started listening command reply thread")
	defer func() {
		log.Info("ended command reply thread")
		if r := recover(); r != nil {
			log.Errorf("listen reply thread panicked in CommandWorkerProcessorWrapper: \n%v", r)
			log.Errorf("stacktrace:\n%s", debug.Stack())
			time.Sleep(5 * time.Second)
			// restart the thread after 5 seconds wait time during panic
			go cpw.listenReply(cpw.commandResultChan, outputChan)
		}
	}()

	// processor guarantees to close this channel upon stop
externalLabel:
	for {
		select {
		case res, resChannelOpen := <-resChan:
			if !resChannelOpen {
				log.Infof("listen reply channel closed")
				break externalLabel
			}

			if cpw.assocProcessor != nil {
				cpw.handleSpecialPlugin(res.LastPlugin, res.PluginResults, res.MessageID)
			}

			if res.LastPlugin != "" {
				log.Infof("received plugin: %s result from Processor", res.LastPlugin)
			} else {
				log.Infof("command: %s complete", res.MessageID)

				//Deleting Old Log Files
				shortInstanceID, _ := cpw.context.Identity().ShortInstanceID()
				go docmanager.DeleteOldOrchestrationDirectories(log,
					shortInstanceID,
					cpw.context.AppConfig().Agent.OrchestrationRootDir,
					cpw.context.AppConfig().Ssm.RunCommandLogsRetentionDurationHours,
					cpw.context.AppConfig().Ssm.AssociationLogsRetentionDurationHours)
			}
			res.ResultType = contracts.RunCommandResult

			if resultChanRef, ok := outputChan[res.UpstreamServiceName]; ok {
				resultChanRef <- res
			} else if resultChanRef, ok = outputChan[contracts.MessageDeliveryService]; ok { // choosing mds as next option
				// by default, we use MDS
				log.Debugf("choosing default as MDS result chan %v", res.MessageID)
				resultChanRef <- res
			} else {
				log.Errorf("dropping reply without pushing to any interactor %v", res.MessageID)
				break
			}
			log.Debugf("reply submission ended %v", res.MessageID)
		}
	}
	cpw.listenReplyEnded <- struct{}{}
}

// handleSpecialPlugin temporary solution on plugins with shared responsibility with agent
func (cpw *CommandWorkerProcessorWrapper) handleSpecialPlugin(lastPluginID string, pluginResults map[string]*contracts.PluginResult, messageID string) {
	var newRes contracts.PluginResult
	log := cpw.context.Log()
	for ID, pluginRes := range pluginResults {
		if pluginRes.PluginName == appconfig.PluginNameRefreshAssociation {
			log.Infof("Found %v to invoke refresh association immediately", pluginRes.PluginName)
			commandID, _ := runCommandContracts.GetCommandID(messageID)
			shortInstanceID, _ := cpw.context.Identity().ShortInstanceID()
			orchestrationDir := filepath.Join(appconfig.DefaultDataStorePath,
				shortInstanceID,
				appconfig.DefaultDocumentRootDirName,
				cpw.context.AppConfig().Agent.OrchestrationRootDir,
				commandID)

			//apply association only when this is the last plugin run
			cpw.assocProcessor.ProcessRefreshAssociation(log, pluginRes, orchestrationDir, lastPluginID == ID)

			log.Infof("Finished refreshing association immediately - response: %v", newRes)
		}
	}
}
