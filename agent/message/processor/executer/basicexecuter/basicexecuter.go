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

// Package executer provides interfaces as document execution logic
package basicexecuter

import (
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/message/processor/executer"
	"github.com/aws/amazon-ssm-agent/agent/message/processor/executer/plugin"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// BasicExecuter is a thin wrapper over runPlugins().
type BasicExecuter struct {
	//TODO 3. populate the attribute once we get 1 and 2 done
	//TODO possible attributes: inbound/outbound channel, context, registered plugins
}

var pluginRunner = func(context context.T,
	documentID string,
	plugins []model.PluginState,
	updateAssoc runpluginutil.UpdateAssociation,
	sendResponse runpluginutil.SendResponse,
	cancelFlag task.CancelFlag) (pluginOutputs map[string]*contracts.PluginResult) {
	return runPlugins(context, documentID, "", plugins, plugin.RegisteredWorkerPlugins(context), sendResponse, updateAssoc, cancelFlag)
}

func NewBasicExecuter() executer.Executer {
	return BasicExecuter{}
}

//TODO 2. do not use callback for sendreply
func (e BasicExecuter) Run(context context.T,
	cancelFlag task.CancelFlag,
	buildReply executer.ReplyBuilder,
	updateAssoc runpluginutil.UpdateAssociation,
	sendResponse runpluginutil.SendResponse,
	docState *model.DocumentState) {
	log := context.Log()
	//TODO split plugin state and docState into 2 different classes?
	log.Debug("Running plugins...")
	outputs := pluginRunner(context, docState.DocumentInformation.MessageID, docState.InstancePluginsInformation, updateAssoc, sendResponse, cancelFlag)
	pluginOutputContent, _ := jsonutil.Marshal(outputs)
	log.Debugf("Plugin outputs %v", jsonutil.Indent(pluginOutputContent))

	//TODO this part should be moved to IOHandler
	//TODO buildReply function will be depracated, with part of its job moved to service and part moved to IOHandler
	payloadDoc := buildReply("", outputs)
	//update documentInfo in interim cmd state file
	newCmdState := docmanager.GetDocumentInterimState(log,
		docState.DocumentInformation.DocumentID,
		docState.DocumentInformation.InstanceID,
		appconfig.DefaultLocationOfCurrent)

	// set document level information which wasn't set previously
	newCmdState.DocumentInformation.AdditionalInfo = payloadDoc.AdditionalInfo
	newCmdState.DocumentInformation.DocumentStatus = payloadDoc.DocumentStatus
	newCmdState.DocumentInformation.DocumentTraceOutput = payloadDoc.DocumentTraceOutput
	newCmdState.DocumentInformation.RuntimeStatus = payloadDoc.RuntimeStatus

	//persist final documentInfo.
	docmanager.PersistDocumentInfo(log,
		newCmdState.DocumentInformation,
		newCmdState.DocumentInformation.DocumentID,
		newCmdState.DocumentInformation.InstanceID,
		appconfig.DefaultLocationOfCurrent)
	log.Debug("Sending reply on message completion ", outputs)
	if sendResponse != nil {
		sendResponse(newCmdState.DocumentInformation.MessageID, "", outputs)

	}
	*docState = newCmdState
}
