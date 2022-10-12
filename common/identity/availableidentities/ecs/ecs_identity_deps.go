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

package ecs

import (
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

var cachedTaskResponse *taskResponse
var cachedContainerResponse *containerResponse
var lock sync.RWMutex

const (
	containerMetadataEnvVarV3 = "ECS_CONTAINER_METADATA_URI"
	containerMetadataEnvVarV4 = "ECS_CONTAINER_METADATA_URI_V4"
	maxRetries                = 4
	ecsInstanceType           = "Container"

	// IdentityType is the identity type for ECS
	IdentityType = "ECS"
)

// Identity is the struct defining the IAgentIdentityInner for ECS metadata service
type Identity struct {
	Log log.T
}

// taskResponse defines the schema for the task response JSON object
type taskResponse struct {
	Cluster          string
	TaskARN          string
	Family           string
	Revision         string
	DesiredStatus    string `json:",omitempty"`
	KnownStatus      string
	AvailabilityZone string    `json:",omitempty"`
	Networks         []network `json:",omitempty"`
}

// containerResponse defines the schema for the container response
// JSON object
type containerResponse struct {
	ID            string `json:"DockerId"`
	Name          string
	DockerName    string
	Image         string
	ImageID       string
	Ports         []portResponse    `json:",omitempty"`
	Labels        map[string]string `json:",omitempty"`
	DesiredStatus string
	KnownStatus   string
	ExitCode      *int `json:",omitempty"`
	Limits        limitsResponse
	CreatedAt     *time.Time `json:",omitempty"`
	StartedAt     *time.Time `json:",omitempty"`
	FinishedAt    *time.Time `json:",omitempty"`
	Type          string
	Networks      []network `json:",omitempty"`
}

// limitsResponse defines the schema for task/cpu limits response
// JSON object
type limitsResponse struct {
	CPU    *float64 `json:"CPU,omitempty"`
	Memory *int64   `json:"Memory,omitempty"`
}

// portResponse defines the schema for portmapping response JSON
// object.
type portResponse struct {
	ContainerPort uint16 `json:"ContainerPort,omitempty"`
	Protocol      string `json:"Protocol,omitempty"`
	HostPort      uint16 `json:"HostPort,omitempty"`
}

// network is a struct that keeps track of metadata of a network interface
type network struct {
	NetworkMode         string   `json:"NetworkMode,omitempty"`
	IPv4Addresses       []string `json:"IPv4Addresses,omitempty"`
	IPv6Addresses       []string `json:"IPv6Addresses,omitempty"`
	IPv4SubnetCIDRBlock string   `json:"IPv4SubnetCIDRBlock,omitempty"`
	IPv6SubnetCIDRBlock string   `json:"IPv6SubnetCIDRBlock,omitempty"`
}
