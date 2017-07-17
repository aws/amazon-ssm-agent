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
	"time"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager"
	"github.com/aws/amazon-ssm-agent/agent/log"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/message/contracts"
	"github.com/aws/amazon-ssm-agent/agent/times"
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
	status, statusCount, runtimeStatuses := docmanager.DocumentResultAggregator(log, pluginID, builder.pluginResults)
	additionalInfo := contracts.AdditionalInfo{
		Agent:               agentInfo,
		DateTime:            times.ToIso8601UTC(time.Now()),
		RuntimeStatusCounts: statusCount,
	}
	payload := messageContracts.SendReplyPayload{
		AdditionalInfo:      additionalInfo,
		DocumentStatus:      status,
		DocumentTraceOutput: "", // TODO: Fill me appropriately
		RuntimeStatus:       runtimeStatuses,
	}
	return payload
}
