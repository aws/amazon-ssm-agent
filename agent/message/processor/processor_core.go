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
// processor_core contains functions that fetch messages and schedule them to be executed
package processor

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"strings"

	"sync"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/engine"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/message/contracts"
	"github.com/aws/amazon-ssm-agent/agent/message/parser"
	"github.com/aws/amazon-ssm-agent/agent/message/service"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	commandStateHelper "github.com/aws/amazon-ssm-agent/agent/statemanager"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/aws-sdk-go/service/ssmmds"
)

var singletonMapOfUnsupportedSSMDocs map[string]bool
var once sync.Once

// processOlderMessages processes older messages that have been persisted in various locations (like from pending & current folders)
func (p *Processor) processOlderMessages() {
	log := p.context.Log()

	instanceID, err := platform.InstanceID()
	if err != nil {
		log.Errorf("error fetching instance id, %v", err)
		return
	}

	//process older messages from PENDING folder
	unprocessedMsgsLocation := path.Join(appconfig.DefaultDataStorePath,
		instanceID,
		appconfig.DefaultCommandRootDirName,
		appconfig.DefaultLocationOfState,
		appconfig.DefaultLocationOfPending)

	if isDirectoryEmpty, _ := fileutil.IsDirEmpty(unprocessedMsgsLocation); isDirectoryEmpty {

		log.Debugf("No messages to process from %v", unprocessedMsgsLocation)

	} else {

		//get all pending messages
		files, err := ioutil.ReadDir(unprocessedMsgsLocation)

		//TODO:  revisit this when bookkeeping is made invasive
		if err != nil {
			log.Errorf("skipping reading pending messages from %v. unexpected error encountered - %v", unprocessedMsgsLocation, err)
			return
		}

		//iterate through all pending messages
		for _, f := range files {
			log.Debugf("Processing an older message with messageID - %v", f.Name())

			//construct the absolute path - safely assuming that interim state for older messages are already present in Pending folder
			file := path.Join(appconfig.DefaultDataStorePath,
				instanceID,
				appconfig.DefaultCommandRootDirName,
				appconfig.DefaultLocationOfState,
				appconfig.DefaultLocationOfPending,
				f.Name())

			var msg ssmmds.Message

			//parse the message
			if err := jsonutil.UnmarshalFile(file, &msg); err != nil {
				log.Errorf("skipping processsing of pending messages. encountered error %v while reading pending message from file - %v", err, f)
			} else {
				//call processMessage for every message read from the pending folder

				//we will end up sending acks again for this message - but that's ok because
				//unless a message has been deleted - multiple acks are allowed by MDS.
				//If this becomes an issue - we can stub out ack part from processMessage
				p.processMessage(&msg)
			}
		}
	}

	//process older messages from CURRENT folder
	p.processMessagesFromCurrent(instanceID)

	return
}

// processOlderMessagesFromCurrent processes older messages that were persisted in CURRENT folder
func (p *Processor) processMessagesFromCurrent(instanceID string) {
	log := p.context.Log()
	config := p.context.AppConfig()

	unprocessedMsgsLocation := path.Join(appconfig.DefaultDataStorePath,
		instanceID,
		appconfig.DefaultCommandRootDirName,
		appconfig.DefaultLocationOfState,
		appconfig.DefaultLocationOfCurrent)

	if isDirectoryEmpty, _ := fileutil.IsDirEmpty(unprocessedMsgsLocation); isDirectoryEmpty {

		log.Debugf("no older messages to process from %v", unprocessedMsgsLocation)

	} else {

		//get all older messages from previous run of agent
		files, err := ioutil.ReadDir(unprocessedMsgsLocation)

		//TODO:  revisit this when bookkeeping is made invasive
		if err != nil {
			log.Errorf("skipping reading inprogress messages from %v. unexpected error encountered - %v", unprocessedMsgsLocation, err)
			return
		}

		//iterate through all old executing messages
		for _, f := range files {
			log.Debugf("processing previously unexecuted message - %v", f.Name())

			//construct the absolute path - safely assuming that interim state for older messages are already present in Current folder
			file := path.Join(appconfig.DefaultDataStorePath,
				instanceID,
				appconfig.DefaultCommandRootDirName,
				appconfig.DefaultLocationOfState,
				appconfig.DefaultLocationOfCurrent,
				f.Name())

			var oldCmdState messageContracts.CommandState

			//parse the message
			//TODO:  Not all messages in Current folder correspond to SendCommand.
			if err := jsonutil.UnmarshalFile(file, &oldCmdState); err != nil {
				log.Errorf("skipping processsing of previously unexecuted messages. encountered error %v while reading unprocessed message from file - %v", err, f)
			} else {
				if oldCmdState.DocumentInformation.RunCount >= config.Mds.CommandRetryLimit {
					//TODO:  Move command to corrupt/failed
					// do not process as the command has failed too many times
					break
				}

				pluginOutputs := make(map[string]*contracts.PluginResult)

				// increment the command run count
				oldCmdState.DocumentInformation.RunCount++
				// Update reboot status
				for v := range oldCmdState.PluginsInformation {
					plugin := oldCmdState.PluginsInformation[v]
					if plugin.HasExecuted && plugin.Result.Status == contracts.ResultStatusSuccessAndReboot {
						log.Debugf("plugin %v has completed a reboot. Setting status to Success.", v)
						plugin.Result.Status = contracts.ResultStatusSuccess
						oldCmdState.PluginsInformation[v] = plugin
						pluginOutputs[v] = &plugin.Result
						p.sendResponse(oldCmdState.DocumentInformation.MessageID, v, pluginOutputs)
					}
				}

				// func PersistData(log log.T, commandID, instanceID, locationFolder string, object interface{}) {
				commandStateHelper.PersistData(log, oldCmdState.DocumentInformation.CommandID, instanceID, appconfig.DefaultLocationOfCurrent, oldCmdState)

				//Submit the work to Job Pool so that we don't block for processing of new messages
				err := p.sendCommandPool.Submit(log, oldCmdState.DocumentInformation.MessageID, func(cancelFlag task.CancelFlag) {
					p.runCmdsUsingCmdState(p.context.With("[messageID="+oldCmdState.DocumentInformation.MessageID+"]"),
						p.service,
						p.pluginRunner,
						cancelFlag,
						p.buildReply,
						p.sendResponse,
						oldCmdState)
				})
				if err != nil {
					log.Error("SendCommand failed for previously unexecuted commands", err)
					return
				}
			}
		}
	}
}

// runCmdsUsingCmdState takes commandState as an input and executes only those plugins which haven't yet executed. This is functionally
// very similar to processSendCommandMessage because everything to do with cmd execution is part of that function right now.
func (p *Processor) runCmdsUsingCmdState(context context.T,
	mdsService service.Service,
	runPlugins PluginRunner,
	cancelFlag task.CancelFlag,
	buildReply replyBuilder,
	sendResponse engine.SendResponse,
	command messageContracts.CommandState) {

	log := context.Log()
	var pluginConfigurations map[string]*contracts.Configuration
	pendingPlugins := false
	pluginConfigurations = make(map[string]*contracts.Configuration)

	//iterate through all plugins to find all plugins that haven't executed yet.
	for k, v := range command.PluginsInformation {
		if v.HasExecuted {
			log.Debugf("skipping execution of Plugin - %v of command - %v since it has already executed.", k, command.DocumentInformation.CommandID)
		} else {
			log.Debugf("Plugin - %v of command - %v will be executed", k, command.DocumentInformation.CommandID)
			pluginConfigurations[k] = &v.Configuration
			pendingPlugins = true
		}
	}

	//execute plugins that haven't been executed yet
	//individual plugins after execution will update interim cmd state file accordingly
	if pendingPlugins {

		log.Debugf("executing following plugins of command - %v", command.DocumentInformation.CommandID)
		for k := range pluginConfigurations {
			log.Debugf("Plugin: %v", k)
		}

		//Since only some plugins of a cmd gets executed here - there is no need to get output from engine & construct the sendReply output.
		//Instead after all plugins of a command get executed, use persisted data to construct sendReply payload
		runPlugins(context, command.DocumentInformation.MessageID, pluginConfigurations, sendResponse, cancelFlag)
	}

	//read from persisted file
	newCmdState := commandStateHelper.GetCommandInterimState(log,
		command.DocumentInformation.CommandID,
		command.DocumentInformation.Destination,
		appconfig.DefaultLocationOfCurrent)

	//construct sendReply payload
	outputs := make(map[string]*contracts.PluginResult)

	for k, v := range newCmdState.PluginsInformation {
		outputs[k] = &v.Result
	}

	pluginOutputContent, _ := jsonutil.Marshal(outputs)
	log.Debugf("plugin outputs %v", jsonutil.Indent(pluginOutputContent))

	payloadDoc := buildReply("", outputs)

	//update interim cmd state file with document level information
	var documentInfo messageContracts.DocumentInfo

	// set document level information which wasn't set previously
	documentInfo.AdditionalInfo = payloadDoc.AdditionalInfo
	documentInfo.DocumentStatus = payloadDoc.DocumentStatus
	documentInfo.DocumentTraceOutput = payloadDoc.DocumentTraceOutput
	documentInfo.RuntimeStatus = payloadDoc.RuntimeStatus

	//persist final documentInfo.
	commandStateHelper.PersistDocumentInfo(log,
		documentInfo,
		command.DocumentInformation.CommandID,
		command.DocumentInformation.Destination,
		appconfig.DefaultLocationOfCurrent)

	// Skip sending response when the document requires a reboot
	if documentInfo.DocumentStatus == contracts.ResultStatusSuccessAndReboot {
		log.Debug("skipping sending response of %v since the document requires a reboot", newCmdState.DocumentInformation.MessageID)
		return
	}

	//send document level reply
	log.Debug("sending reply on message completion ", outputs)
	sendResponse(command.DocumentInformation.MessageID, "", outputs)

	//persist : commands execution in completed folder (terminal state folder)
	log.Debugf("execution of %v is over. Moving interimState file from Current to Completed folder", newCmdState.DocumentInformation.MessageID)

	commandStateHelper.MoveCommandState(log,
		newCmdState.DocumentInformation.CommandID,
		newCmdState.DocumentInformation.Destination,
		appconfig.DefaultLocationOfCurrent,
		appconfig.DefaultLocationOfCompleted)

	log.Debugf("deleting message")
	isUpdate := false
	for pluginName := range pluginConfigurations {
		if pluginName == appconfig.PluginNameAwsAgentUpdate {
			isUpdate = true
		}
	}
	if !isUpdate {
		err := mdsService.DeleteMessage(log, newCmdState.DocumentInformation.MessageID)
		if err != nil {
			sdkutil.HandleAwsError(log, err, p.processorStopPolicy)
		}
	} else {
		log.Debug("messageDeletion skipped as it will be handled by external process")
	}
}

func (p *Processor) processMessage(msg *ssmmds.Message) {
	// create separate logger that includes messageID with every log message
	context := p.context.With("[messageID=" + *msg.MessageId + "]")
	log := context.Log()

	log.Debug("Processing message")

	if err := validate(msg); err != nil {
		log.Error("message not valid, ignoring: ", err)
		return
	}

	err := p.service.AcknowledgeMessage(log, *msg.MessageId)
	if err != nil {
		sdkutil.HandleAwsError(log, err, p.processorStopPolicy)
		return
	}
	log.Debugf("Ack done. Received message - messageId - %v, MessageString - %v", *msg.MessageId, msg.GoString())

	//persisting received msg in file-system [pending folder]
	p.persistData(msg, appconfig.DefaultLocationOfPending)

	log.Debugf("Processing to send a reply to update the document status to InProgress")

	p.sendDocLevelResponse(*msg.MessageId, contracts.ResultStatusInProgress, "")

	log.Debugf("SendReply done. Received message - messageId - %v, MessageString - %v", *msg.MessageId, msg.GoString())

	switch {
	case strings.HasPrefix(*msg.Topic, string(SendCommandTopicPrefix)):
		err := p.sendCommandPool.Submit(log, *msg.MessageId, func(cancelFlag task.CancelFlag) {
			p.processSendCommandMessage(context,
				p.service,
				p.orchestrationRootDir,
				p.pluginRunner,
				cancelFlag,
				p.buildReply,
				p.sendResponse,
				*msg)
		})
		if err != nil {
			log.Error("SendCommand failed", err)
			return
		}

	case strings.HasPrefix(*msg.Topic, string(CancelCommandTopicPrefix)):
		err := p.cancelCommandPool.Submit(log, *msg.MessageId, func(cancelFlag task.CancelFlag) {
			p.processCancelCommandMessage(context, p.service, p.sendCommandPool, *msg)
		})
		if err != nil {
			log.Error("CancelCommand failed", err)
			return
		}

	default:
		log.Error("unexpected topic name ", *msg.Topic)
	}
}

// processSendCommandMessage processes a single send command message received from MDS.
func (p *Processor) processSendCommandMessage(context context.T,
	mdsService service.Service,
	messagesOrchestrationRootDir string,
	runPlugins PluginRunner,
	cancelFlag task.CancelFlag,
	buildReply replyBuilder,
	sendResponse engine.SendResponse,
	msg ssmmds.Message) {

	commandID := getCommandID(*msg.MessageId)

	log := context.Log()
	log.Debug("Processing send command message ", *msg.MessageId)
	log.Trace("Processing send command message ", jsonutil.Indent(*msg.Payload))

	parsedMessage, err := parser.ParseMessageWithParams(log, *msg.Payload)
	if err != nil {
		log.Error("format of received message is invalid ", err)
		err = mdsService.FailMessage(log, *msg.MessageId, service.InternalHandlerException)
		if err != nil {
			sdkutil.HandleAwsError(log, err, p.processorStopPolicy)
		}
		return
	}

	parsedMessageContent, _ := jsonutil.Marshal(parsedMessage)
	log.Debug("ParsedMessage is ", jsonutil.Indent(parsedMessageContent))

	// adapt plugin configuration format from MDS to plugin expected format
	s3KeyPrefix := path.Join(parsedMessage.OutputS3KeyPrefix, parsedMessage.CommandID, *msg.Destination)

	messageOrchestrationDirectory := filepath.Join(messagesOrchestrationRootDir, commandID)

	// Check if it is a managed instance and its executing managed instance incompatible AWS SSM public document.
	// A few public AWS SSM documents contain code which is not compatible when run on managed instances.
	// isManagedInstanceIncompatibleAWSSSMDocument makes sure to find such documents at runtime and replace the incompatible code.
	isMI, err := isManagedInstance()
	if err != nil {
		log.Errorf("Error determining managed instance. error: %v", err)
	}

	if isMI && isManagedInstanceIncompatibleAWSSSMDocument(parsedMessage.DocumentName) {
		log.Debugf("Running Incompatible AWS SSM Document %v on managed instance", parsedMessage.DocumentName)
		if parsedMessage, err = removeDependencyOnInstanceMetadataForManagedInstance(context, parsedMessage); err != nil {
			return
		}
	}

	pluginConfigurations := getPluginConfigurations(
		parsedMessage.DocumentContent.RuntimeConfig,
		messageOrchestrationDirectory,
		parsedMessage.OutputS3BucketName,
		s3KeyPrefix,
		*msg.MessageId)

	//persist : all information in current folder
	log.Info("Persisting message in current execution folder")

	//Data format persisted in Current Folder is defined by the struct - CommandState
	interimCmdState := initializeCommandState(pluginConfigurations, msg, parsedMessage)

	// persist new interim command state in current folder
	commandStateHelper.PersistData(log, commandID, *msg.Destination, appconfig.DefaultLocationOfCurrent, interimCmdState)

	//Deleting from pending folder since the command is getting executed
	commandStateHelper.RemoveData(log, commandID, *msg.Destination, appconfig.DefaultLocationOfPending)

	log.Debug("Running plugins...")
	outputs := runPlugins(context, *msg.MessageId, pluginConfigurations, sendResponse, cancelFlag)
	pluginOutputContent, _ := jsonutil.Marshal(outputs)
	log.Debugf("Plugin outputs %v", jsonutil.Indent(pluginOutputContent))

	payloadDoc := buildReply("", outputs)

	//update documentInfo in interim cmd state file
	documentInfo := commandStateHelper.GetDocumentInfo(log, commandID, *msg.Destination, appconfig.DefaultLocationOfCurrent)

	// set document level information which wasn't set previously
	documentInfo.AdditionalInfo = payloadDoc.AdditionalInfo
	documentInfo.DocumentStatus = payloadDoc.DocumentStatus
	documentInfo.DocumentTraceOutput = payloadDoc.DocumentTraceOutput
	documentInfo.RuntimeStatus = payloadDoc.RuntimeStatus

	//persist final documentInfo.
	commandStateHelper.PersistDocumentInfo(log,
		documentInfo,
		commandID,
		*msg.Destination,
		appconfig.DefaultLocationOfCurrent)

	// Skip sending response when the document requires a reboot
	if documentInfo.DocumentStatus == contracts.ResultStatusSuccessAndReboot {
		log.Debug("skipping sending response of %v since the document requires a reboot", *msg.MessageId)
		return
	}

	log.Debug("Sending reply on message completion ", outputs)
	sendResponse(*msg.MessageId, "", outputs)

	//persist : commands execution in completed folder (terminal state folder)
	log.Debugf("execution of %v is over. Moving interimState file from Current to Completed folder", *msg.MessageId)

	commandStateHelper.MoveCommandState(log,
		commandID,
		*msg.Destination,
		appconfig.DefaultLocationOfCurrent,
		appconfig.DefaultLocationOfCompleted)

	log.Debugf("Deleting message")
	isUpdate := false
	for pluginName := range pluginConfigurations {
		if pluginName == appconfig.PluginNameAwsAgentUpdate {
			isUpdate = true
		}
	}
	if !isUpdate {
		err = mdsService.DeleteMessage(log, *msg.MessageId)
		if err != nil {
			sdkutil.HandleAwsError(log, err, p.processorStopPolicy)
		}
	} else {
		log.Debug("MessageDeletion skipped as it will be handled by external process")
	}
}

// processCancelCommandMessage processes a single send command message received from MDS.
func (p *Processor) processCancelCommandMessage(context context.T,
	mdsService service.Service,
	sendCommandPool task.Pool,
	msg ssmmds.Message) {

	log := context.Log()
	log.Debug("Processing cancel command message ", jsonutil.Indent(*msg.Payload))

	var parsedMessage messageContracts.CancelPayload
	err := json.Unmarshal([]byte(*msg.Payload), &parsedMessage)
	if err != nil {
		log.Error("format of received cancel message is invalid ", err)
		err = mdsService.FailMessage(log, *msg.MessageId, service.InternalHandlerException)
		if err != nil {
			sdkutil.HandleAwsError(log, err, p.processorStopPolicy)
		}
		return
	}
	log.Debugf("ParsedMessage is %v", parsedMessage)

	//persist in current folder here
	cancelCmd := initializeCancelCommandState(msg, parsedMessage)
	commandID := getCommandID(*msg.MessageId)
	// persist new interim command state in current folder
	commandStateHelper.PersistData(log, commandID, *msg.Destination, appconfig.DefaultLocationOfCurrent, cancelCmd)

	//remove from pending folder
	commandStateHelper.RemoveData(log, commandID, *msg.Destination, appconfig.DefaultLocationOfPending)

	log.Debugf("Canceling job with id %v...", parsedMessage.CancelMessageID)

	if found := sendCommandPool.Cancel(parsedMessage.CancelMessageID); !found {
		log.Debugf("Job with id %v not found (possibly completed)", parsedMessage.CancelMessageID)
		cancelCmd.DebugInfo = fmt.Sprintf("Command %v couldn't be cancelled", cancelCmd.CancelCommandID)
		cancelCmd.Status = contracts.ResultStatusFailed
	} else {
		cancelCmd.DebugInfo = fmt.Sprintf("Command %v cancelled", cancelCmd.CancelCommandID)
		cancelCmd.Status = contracts.ResultStatusSuccess
	}

	//persist the final status of cancel-message in current folder
	commandStateHelper.PersistData(log, commandID, *msg.Destination, appconfig.DefaultLocationOfCurrent, cancelCmd)

	//persist : commands execution in completed folder (terminal state folder)
	log.Debugf("Execution of %v is over. Moving interimState file from Current to Completed folder", *msg.MessageId)

	commandStateHelper.MoveCommandState(log,
		commandID,
		*msg.Destination,
		appconfig.DefaultLocationOfCurrent,
		appconfig.DefaultLocationOfCompleted)

	log.Debugf("Deleting message")
	err = mdsService.DeleteMessage(log, *msg.MessageId)
	if err != nil {
		sdkutil.HandleAwsError(log, err, p.processorStopPolicy)
	}
}
