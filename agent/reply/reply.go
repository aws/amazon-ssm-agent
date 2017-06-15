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

//TODO move this package to service, or contract package
// Package reply provides utilities to parse reply payload
package reply

import (
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/log"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/message/contracts"
)

//TODO once we remove the callback "SendReply", use this class to build reply
//ReplyBuilder is used by RunCommand Service to accumulate plugin updates, and format SendReply payload
type ReplyBuilder interface {
	// UpdatePluginResult updates the internal store with the latest update PluginResult
	UpdatePluginResult(res contracts.PluginResult) error
	// Format the Payload send back to MDS service
	FormatPayload(log.T, string, contracts.AgentInfo) messageContracts.SendReplyPayload
}

//SendReplyBuilder impl ReplyBuilder used by MDS service to receive updates and send reply payload
type SendReplyBuilder struct {
	pluginResults map[string]*contracts.PluginResult
}

func NewSendReplyBuilder() SendReplyBuilder {
	return SendReplyBuilder{
		pluginResults: make(map[string]*contracts.PluginResult),
	}
}

// UpdatePluginResult the internal plugin map with the latest Result
func (builder SendReplyBuilder) UpdatePluginResult(res contracts.PluginResult) error {
	builder.pluginResults[res.PluginName] = &res
	return nil
}

// build SendReply Payload from the internal plugins map
func (builder SendReplyBuilder) FormatPayload(log log.T, pluginID string, agentInfo contracts.AgentInfo) messageContracts.SendReplyPayload {
	docInfo := docmanager.DocumentResultAggregator(log, pluginID, builder.pluginResults)
	return PrepareReplyPayload(docInfo, agentInfo)
}

// PrepareReplyPayload creates the payload object for SendReply based on plugin outputs.
func PrepareReplyPayload(docInfo model.DocumentInfo, agentInfo contracts.AgentInfo) (payload messageContracts.SendReplyPayload) {
	docInfo.AdditionalInfo.Agent = agentInfo
	payload = messageContracts.SendReplyPayload{
		AdditionalInfo:      docInfo.AdditionalInfo,
		DocumentStatus:      docInfo.DocumentStatus,
		DocumentTraceOutput: "", // TODO: Fill me appropriately
		RuntimeStatus:       docInfo.RuntimeStatus,
	}
	return
}
