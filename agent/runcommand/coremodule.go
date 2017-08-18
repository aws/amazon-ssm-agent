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
	"fmt"
	"strings"
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	asocitscheduler "github.com/aws/amazon-ssm-agent/agent/association/scheduler"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	mdsService "github.com/aws/amazon-ssm-agent/agent/runcommand/mds"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/aws-sdk-go/service/ssmmds"
	"github.com/carlescere/scheduler"
)

const (
	documentContent  = "DocumentContent"
	runtimeConfig    = "runtimeConfig"
	cloudwatchPlugin = "aws:cloudWatch"
	properties       = "properties"
	parameters       = "Parameters"
)

var singletonMapOfUnsupportedSSMDocs map[string]bool
var once sync.Once

var loadDocStateFromSendCommand = parseSendCommandMessage
var loadDocStateFromCancelCommand = parseCancelCommandMessage

// Name returns the module name
func (s *RunCommandService) ModuleName() string {
	return s.name
}

// Execute starts the scheduling of the message processor plugin
func (s *RunCommandService) ModuleExecute(context context.T) (err error) {

	log := s.context.Log()
	log.Info("Starting document processing engine...")
	var resultChan chan contracts.DocumentResult
	if resultChan, err = s.processor.Start(); err != nil {
		log.Errorf("unable to start document processor: %v", err)
		return
	}
	go s.listenReply(resultChan)
	log.Info("Starting message polling")
	if s.messagePollJob, err = scheduler.Every(pollMessageFrequencyMinutes).Minutes().Run(s.loop); err != nil {
		context.Log().Errorf("unable to schedule message poll job. %v", err)
	}
	//TODO move association polling out in the next CR
	if s.pollAssociations {
		associationFrequenceMinutes := context.AppConfig().Ssm.AssociationFrequencyMinutes
		log.Info("Starting association polling")
		log.Debugf("Association polling frequency is %v", associationFrequenceMinutes)
		var job *scheduler.Job
		if job, err = asocitscheduler.CreateScheduler(
			log,
			s.assocProcessor.ProcessAssociation,
			associationFrequenceMinutes); err != nil {
			context.Log().Errorf("unable to schedule association processor. %v", err)
		}
		s.assocProcessor.InitializeAssociationProcessor()
		s.assocProcessor.SetPollJob(job)
	}
	return
}

func (s *RunCommandService) ModuleRequestStop(stopType contracts.StopType) (err error) {
	//first stop the message poller
	s.stop()
	//second stop the message processor
	s.processor.Stop(stopType)

	//TODO move this out once we have association moved to a different core module
	var wg sync.WaitGroup
	// shutdown the association task pool in a separate go routine
	wg.Add(1)
	go func() {
		defer wg.Done()
		if s.assocProcessor != nil {
			s.assocProcessor.ShutdownAndWait(stopType)
		}
	}()

	// wait for everything to shutdown
	wg.Wait()
	return nil
}

func (s *RunCommandService) listenReply(resultChan chan contracts.DocumentResult) {
	log := s.context.Log()
	//processor guarantees to close this channel upon stop
	for res := range resultChan {

		s.handleRefreshAssociationPlugin(res.PluginResults)

		if res.LastPlugin != "" {
			log.Infof("received plugin: %v result from Processor", res.LastPlugin)
		} else {
			log.Infof("command: %v complete", res.MessageID)
		}
		s.sendResponse(res.MessageID, res)
	}
}

func (s *RunCommandService) handleRefreshAssociationPlugin(pluginRes map[string]*contracts.PluginResult) {
	var newRes contracts.PluginResult

	log := s.context.Log()

	for _, pluginRes := range pluginRes {
		if pluginRes.PluginName == appconfig.PluginNameRefreshAssociation {
			log.Infof("Found %v to invoke refresh association immediately", pluginRes.PluginName)

			orchestrationDir := fileutil.BuildPath(s.orchestrationRootDir, pluginRes.PluginName)

			s.assocProcessor.ProcessRefreshAssociation(log, pluginRes, orchestrationDir)

			log.Infof("Finished refreshing association immediately - response: %v", newRes)
		}
	}

}

func (s *RunCommandService) processMessage(msg *ssmmds.Message) {
	var (
		docState *model.DocumentState
		err      error
	)

	// create separate logger that includes messageID with every log message
	context := s.context.With("[messageID=" + *msg.MessageId + "]")
	log := context.Log()
	log.Debug("Processing message")

	if err = validate(msg); err != nil {
		log.Error("message not valid, ignoring: ", err)
		return
	}

	if strings.HasPrefix(*msg.Topic, string(SendCommandTopicPrefix)) {
		docState, err = loadDocStateFromSendCommand(context, msg, s.orchestrationRootDir)
		if err != nil {
			log.Error(err)
			s.sendDocLevelResponse(*msg.MessageId, contracts.ResultStatusFailed, err.Error())
			return
		}
	} else if strings.HasPrefix(*msg.Topic, string(CancelCommandTopicPrefix)) {
		docState, err = loadDocStateFromCancelCommand(context, msg, s.orchestrationRootDir)
	} else {
		err = fmt.Errorf("unexpected topic name %v", *msg.Topic)
	}

	if err != nil {
		log.Error("format of received message is invalid ", err)
		if err = s.service.FailMessage(log, *msg.MessageId, mdsService.InternalHandlerException); err != nil {
			sdkutil.HandleAwsError(log, err, s.processorStopPolicy)
		}
		return
	}
	if err = s.service.AcknowledgeMessage(log, *msg.MessageId); err != nil {
		sdkutil.HandleAwsError(log, err, s.processorStopPolicy)
		return
	}

	log.Debugf("Ack done. Received message - messageId - %v", *msg.MessageId)

	log.Debugf("Processing to send a reply to update the document status to InProgress")

	//TODO This function should be called in service when it submits the document to the engine
	s.sendDocLevelResponse(*msg.MessageId, contracts.ResultStatusInProgress, "")

	log.Debugf("SendReply done. Received message - messageId - %v", *msg.MessageId)
	switch docState.DocumentType {
	case model.SendCommand, model.SendCommandOffline:
		s.processor.Submit(*docState)
	case model.CancelCommand, model.CancelCommandOffline:
		s.processor.Cancel(*docState)

	default:
		log.Error("unexpected document type ", docState.DocumentType)
	}

}
