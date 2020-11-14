// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// config package implement configuration retrieval for the session package.
package config

import (
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/rip"
)

const (
	SessionServiceName            = "MessageGatewayService"
	DotDelimiter                  = "."
	ServiceName                   = rip.MgsServiceName
	HttpsPrefix                   = "https://"
	WebSocketPrefix               = "wss://"
	ControlChannel                = "control-channel"
	DataChannel                   = "data-channel"
	StreamQueryParameter          = "stream"
	RoleQueryParameter            = "role"
	APIVersion                    = "v1"
	RolePublishSubscribe          = "publish_subscribe"
	RoleSubscribe                 = "subscribe"
	MessageSchemaVersion          = "1.0"
	RetryAttempt                  = 5
	DefaultTransmissionTimeout    = 200 * time.Millisecond
	DefaultRoundTripTime          = 100 * time.Millisecond
	DefaultRoundTripTimeVariation = 0
	ResendSleepInterval           = 100 * time.Millisecond
	WebSocketPingInterval         = 5 * time.Minute

	// Buffer capacity of 100000 items with each buffer item of 1024 bytes leads to max usage of 100MB (100000 * 1024 bytes = 100MB) of instance memory.
	// When changing StreamDataPayloadSize, make corresponding change to buffer capacity to ensure no more than 100MB of instance memory is used.
	StreamDataPayloadSize         = 1024
	OutgoingMessageBufferCapacity = 100000
	IncomingMessageBufferCapacity = 100000

	// Round trip time constant
	RTTConstant = 1.0 / 8.0
	// Round trip time variation constant
	RTTVConstant           = 1.0 / 4.0
	ClockGranularity       = 10 * time.Millisecond
	MaxTransmissionTimeout = 1 * time.Second

	RetryGeometricRatio                   = 2
	RetryJitterRatio                      = 0.5
	ControlChannelNumMaxRetries           = -1 //forever retries for control channel
	ControlChannelRetryInitialDelayMillis = 5000
	ControlChannelRetryMaxIntervalMillis  = 1000 * 60 * 40 // 40 mins

	DataChannelNumMaxAttempts          = 5
	DataChannelRetryInitialDelayMillis = 100
	DataChannelRetryMaxIntervalMillis  = 5000

	IpcFileName        = "ipcTempFile"
	ExecOutputFileName = "output"
	LogFileExtension   = ".log"
	ScreenBufferSize   = 30000
	Exit               = "exit"

	// ResumeReadExitCode indicates to resume reading from established connection.
	ResumeReadExitCode = -1
	// LocalPortForwarding is one of types supported by port plugin and is used to differentiate handling of error
	// with scenario of sshd server port forwarding.
	LocalPortForwarding = "LocalPortForwarding"

	CloudWatchEncryptionErrorMsg                     = "We couldn't start the session because encryption is not set up on the selected CloudWatch Logs log group. Either encrypt the log group or choose an option to enable logging without encryption."
	UnsupportedPowerShellVersionForStreamingErrorMsg = "The PowerShell version installed on the instance doesn’t support streaming logs to CloudWatch. Updated PowerShell to version 5.1 or later to stream session data to CloudWatch."
	PowerShellTranscriptLoggingEnabledErrorMsg       = "The PowerShell Transcription policy setting is configured on the instance. Update PowerShell Transcription policy setting to 'Not Configured' to stream session data to CloudWatch."
	S3EncryptionErrorMsg                             = "We couldn't start the session because encryption is not set up on the selected Amazon S3 bucket. Either encrypt the bucket or choose an option to enable logging without encryption."
)

var GetMgsEndpointFromRip = func(context context.T, region string) string {
	return rip.GetMgsEndpoint(context, region)
}
