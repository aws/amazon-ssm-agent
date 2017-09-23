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
	"time"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/runcommand/contracts"
	"github.com/aws/amazon-ssm-agent/agent/times"
)

// build SendReply Payload from the internal plugins map
func FormatPayload(log log.T, pluginID string, agentInfo contracts.AgentInfo, outputs map[string]*contracts.PluginResult) messageContracts.SendReplyPayload {
	status, statusCount, runtimeStatuses := contracts.DocumentResultAggregator(log, pluginID, outputs)
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
