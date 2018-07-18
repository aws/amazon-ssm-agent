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

import "time"

const (
	DotDelimiter                  = "."
	ServiceName                   = "ssmmessages"
	HttpsPrefix                   = "https://"
	WebSocketPrefix               = "wss://"
	ControlChannel                = "control-channel"
	DataChannel                   = "data-channel"
	StreamQueryParameter          = "stream"
	RoleQueryParameter            = "role"
	APIVersion                    = "v1"
	RolePublishSubscribe          = "publish_subscribe"
	MessageSchemaVersion          = "1.0"
	RetryAttempt                  = 5
	DefaultTransmissionTimeout    = 200 * time.Millisecond
	DefaultRoundTripTime          = 100 * time.Millisecond
	DefaultRoundTripTimeVariation = 0
	ResendSleepInterval           = 100 * time.Millisecond
	StreamDataPayloadSize         = 1024
	OutgoingMessageBufferCapacity = 10000
	IncomingMessageBufferCapacity = 10000
	// Round trip time constant
	RTTConstant = 1.0 / 8.0
	// Round trip time variation constant
	RTTVConstant           = 1.0 / 4.0
	ClockGranularity       = 10 * time.Millisecond
	MaxTransmissionTimeout = 1 * time.Second
)

// TODO: use RIP to get hostname.
// GetHostName gets the host name. e.g. ssmmessages.{region}.amazonaws.com(.cn)
func GetHostName() (string, error) {
	return "ssmmessages.us-east-1.amazonaws.com", nil
}
