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

// Package reply provides utilities to parse reply payload
package reply

import (
	"fmt"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/message/contracts"
	"github.com/aws/amazon-ssm-agent/agent/times"
)

// PrepareReplyPayload creates the payload object for SendReply based on plugin outputs.
func PrepareReplyPayload(pluginID string, runtimeStatuses map[string]*contracts.PluginRuntimeStatus, dateTime time.Time, agentInfo contracts.AgentInfo) (payload messageContracts.SendReplyPayload) {

	// TODO instance this needs to be revised to be in parity with ec2config
	documentStatus := contracts.ResultStatusSuccess
	var runtimeStatusCounts = map[string]int{}
	pluginCounts := len(runtimeStatuses)

	for _, pluginResult := range runtimeStatuses {
		if pluginResult.Status == contracts.ResultStatusFailed {
			documentStatus = contracts.ResultStatusFailed
		}
		runtimeStatusCounts[string(pluginResult.Status)]++
	}

	//	  New precedence order of plugin states
	//	  Failed > TimedOut > Cancelled > Success > Cancelling > InProgress > Pending
	//	  The above order is a contract between SSM service and agent and hence for the calculation of aggregate
	//	  status of a (command) document, we follow the above precedence order.
	//
	//	  Note:
	//	  A command could have been failed/cancelled even before a plugin started executing, during which pendingItems > 0
	//	  but overallResult.Status would be Failed/Cancelled. That's the reason we check for OverallResult status along
	//	  with number of failed/cancelled items.
	//    TODO : We need to handle above to be able to send document traceoutput in case of document level errors.

	if runtimeStatusCounts[string(contracts.ResultStatusSuccessAndReboot)] > 0 {
		documentStatus = contracts.ResultStatusSuccessAndReboot
	} else if runtimeStatusCounts[string(contracts.ResultStatusFailed)] > 0 {
		documentStatus = contracts.ResultStatusFailed
	} else if runtimeStatusCounts[string(contracts.ResultStatusTimedOut)] > 0 {
		documentStatus = contracts.ResultStatusTimedOut
	} else if runtimeStatusCounts[string(contracts.ResultStatusCancelled)] > 0 {
		documentStatus = contracts.ResultStatusCancelled
	} else if runtimeStatusCounts[string(contracts.ResultStatusSuccess)] == pluginCounts {
		documentStatus = contracts.ResultStatusSuccess
	} else {
		documentStatus = contracts.ResultStatusInProgress
	}

	runtimeStatusesFiltered := make(map[string]*contracts.PluginRuntimeStatus)

	if pluginID != "" {
		runtimeStatusesFiltered[pluginID] = runtimeStatuses[pluginID]
	} else {
		runtimeStatusesFiltered = runtimeStatuses
	}

	payload = messageContracts.SendReplyPayload{
		AdditionalInfo: contracts.AdditionalInfo{
			Agent:               agentInfo,
			DateTime:            times.ToIso8601UTC(dateTime),
			RuntimeStatusCounts: runtimeStatusCounts,
		},
		DocumentStatus:      documentStatus,
		DocumentTraceOutput: "", // TODO: Fill me appropriately
		RuntimeStatus:       runtimeStatusesFiltered,
	}
	return
}

// PrepareRuntimeStatuses creates runtime statuses from plugin outputs.
// contracts.PluginResult and contracts.PluginRuntimeStatus are mostly same.
// however they are decoupled on purpose so that we can do any special handling / serializing when sending response to server side.
func PrepareRuntimeStatuses(log log.T, pluginOutputs map[string]*contracts.PluginResult) (runtimeStatuses map[string]*contracts.PluginRuntimeStatus) {
	runtimeStatuses = make(map[string]*contracts.PluginRuntimeStatus)
	for pluginID, pluginResult := range pluginOutputs {
		rs := prepareRuntimeStatus(log, *pluginResult)
		runtimeStatuses[pluginID] = &rs
	}
	return
}

// prepareRuntimeStatus creates the structure for the runtimeStatus section of the payload of SendReply
// for a particular plugin.
func prepareRuntimeStatus(log log.T, pluginResult contracts.PluginResult) contracts.PluginRuntimeStatus {
	var resultAsString string

	if err := pluginResult.Error; err == nil {
		resultAsString = fmt.Sprintf("%v", pluginResult.Output)
	} else {
		resultAsString = err.Error()
	}

	runtimeStatus := contracts.PluginRuntimeStatus{
		Code:          pluginResult.Code,
		Name:          pluginResult.PluginName,
		Status:        pluginResult.Status,
		Output:        resultAsString,
		StartDateTime: times.ToIso8601UTC(pluginResult.StartDateTime),
		EndDateTime:   times.ToIso8601UTC(pluginResult.EndDateTime),
	}

	if pluginResult.OutputS3BucketName != "" {
		runtimeStatus.OutputS3BucketName = pluginResult.OutputS3BucketName
		if pluginResult.OutputS3KeyPrefix != "" {
			runtimeStatus.OutputS3KeyPrefix = pluginResult.OutputS3KeyPrefix
		}
	}

	if runtimeStatus.Status == contracts.ResultStatusFailed && runtimeStatus.Code == 0 {
		runtimeStatus.Code = 1
	}

	return runtimeStatus
}
