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
package executer

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/message/contracts"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// Executer is an interface that execute a given docuemnt
//TODO change callback to go channel, remove service dependency
type Executer interface {
	Run(context context.T,
		cancelFlag task.CancelFlag,
		buildReply ReplyBuilder,
		sendResponse runpluginutil.SendResponse,
		docState *model.DocumentState)
	//TODO 1. expose these 2 functions instead of using cancelFlag
	//Shutdown()
	//Cancel()
}

type PluginRunner func(context context.T, documentID string, plugins []model.PluginState, sendResponse runpluginutil.SendResponse, cancelFlag task.CancelFlag) (pluginOutputs map[string]*contracts.PluginResult)
type ReplyBuilder func(pluginID string, results map[string]*contracts.PluginResult) messageContracts.SendReplyPayload
