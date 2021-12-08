// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing`
// permissions and limitations under the License.

// Package contracts defines the common structs needed for messageservice
package contracts

// InstanceMessage is an interface between agent and both upstream services - MDS, MGS
// Messages from MDS (ssmmds.Message) and MGS (AgentPayload) will be converted to InstanceMessage
// ssmmds.Message, AgentPayload (upstream) --> InstanceMessage --> SendCommandPayload, CancelCommandPayload (agent)
type InstanceMessage struct {
	CreatedDate string
	Destination string
	MessageId   string
	Payload     string
	Topic       string
}
