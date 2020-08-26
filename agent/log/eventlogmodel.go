// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package log is used to initialize the logger(main logger and event logger). This package should be imported once, usually from main, then call GetLogger.
package log

const (
	AmazonAgentStartEvent       = "amazon-ssm-agent.start" // Amazon core agent Start Event
	AmazonAgentWorkerStartEvent = "ssm-agent-worker.start" //Amazon agent worker start event

	AuditSentSuccessFooter = "AuditSent="
	SchemaVersionHeader    = "SchemaVersion="

	// Message types for the event log chunks created
	AgentTelemetryMessage    = "agent_telemetry"     // AgentTelemetryMessage represents message type for number Legacy Agent/Agent Reboot
	AgentUpdateResultMessage = "agent_update_result" // AgentUpdateResultMessage represents message type for number Agent update result

	BytePatternLen = 9 // BytePatternLen represents length of last read byte section in footer of audit file. Considered the audit file max file size to be 999.99MB

	VersionRegexPattern = "^\\d+(\\.\\d+){3}$" // pattern to filter out invalid versions
)
