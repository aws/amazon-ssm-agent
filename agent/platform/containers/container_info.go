// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

//Package provides container info
package containers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var cachedTaskResponse *TaskResponse
var cachedContainerResponse *ContainerResponse
var lock sync.RWMutex

const (
	ContainerMetadataEnvVar = "ECS_CONTAINER_METADATA_URI"
	MaxRetries              = 2
)

// TaskResponse defines the schema for the task response JSON object
type TaskResponse struct {
	Cluster       string
	TaskARN       string
	Family        string
	Revision      string
	DesiredStatus string `json:",omitempty"`
	KnownStatus   string
}

// ContainerResponse defines the schema for the container response
// JSON object
type ContainerResponse struct {
	ID            string `json:"DockerId"`
	Name          string
	DockerName    string
	Image         string
	ImageID       string
	Ports         []PortResponse    `json:",omitempty"`
	Labels        map[string]string `json:",omitempty"`
	DesiredStatus string
	KnownStatus   string
	ExitCode      *int `json:",omitempty"`
	Limits        LimitsResponse
	CreatedAt     *time.Time `json:",omitempty"`
	StartedAt     *time.Time `json:",omitempty"`
	FinishedAt    *time.Time `json:",omitempty"`
	Type          string
	Networks      []Network `json:",omitempty"`
}

// LimitsResponse defines the schema for task/cpu limits response
// JSON object
type LimitsResponse struct {
	CPU    *float64 `json:"CPU,omitempty"`
	Memory *int64   `json:"Memory,omitempty"`
}

// PortResponse defines the schema for portmapping response JSON
// object.
type PortResponse struct {
	ContainerPort uint16 `json:"ContainerPort,omitempty"`
	Protocol      string `json:"Protocol,omitempty"`
	HostPort      uint16 `json:"HostPort,omitempty"`
}

// Network is a struct that keeps track of metadata of a network interface
type Network struct {
	NetworkMode   string   `json:"NetworkMode,omitempty"`
	IPv4Addresses []string `json:"IPv4Addresses,omitempty"`
	IPv6Addresses []string `json:"IPv6Addresses,omitempty"`
}

type Container struct{}

type IContainer interface {
	TargetID() (string, error)
	ContainerID() (string, error)
	Region() (string, error)
}

// metadataResponse returns metadata response with retries
func metadataResponse(endpoint string, respType string) ([]byte, error) {
	var resp []byte
	var err error
	client := &http.Client{
		Timeout: 2 * time.Second,
	}
	for i := 0; i < MaxRetries; i++ {
		resp, err = metadataResponseOnce(client, endpoint, respType)
		if err == nil {
			return resp, nil
		}
		// duration between retries
		time.Sleep(time.Second)
	}

	return nil, err
}

// metadataResponseOnce returns metadata response
var metadataResponseOnce = func(client *http.Client, endpoint string, respType string) ([]byte, error) {
	resp, err := client.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("%s: unable to get response: %v", respType, err)
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: incorrect status code  %d", respType, resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: unable to read response body: %v", respType, err)
	}

	return body, nil
}

// TargetID returns the current target identifier
func (container *Container) TargetID() (string, error) {

	clusterName, taskId, err := fetchClusterNameAndTaskId()
	if err != nil {
		return "", err
	}
	containerId, err := fetchContainerId()
	if err != nil {
		return "", err
	}
	return "ecs:" + clusterName + "_" + taskId + "_" + containerId, nil
}

// ContainerID returns the current container identifier
func (container *Container) ContainerID() (string, error) {
	return fetchContainerId()
}

// Region returns the target region
func (container *Container) Region() (string, error) {
	return fetchRegion()
}

// fetchRegion returns the region
func fetchRegion() (string, error) {
	taskMetadata, err := taskMetadataResponse()
	if err != nil {
		return "", err
	}
	taskArn := taskMetadata.TaskARN
	seperatedArray := strings.Split(taskArn, ":")
	region := seperatedArray[3]

	return region, nil
}

// fetchClusterNameAndTaskId returns the clusterName and taskId of the target
func fetchClusterNameAndTaskId() (string, string, error) {
	taskMetadata, err := taskMetadataResponse()
	//The TaskID itself is everything that comes after /
	if err != nil {
		return "", "", err
	}
	taskArn := taskMetadata.TaskARN
	seperatedTaskArn := strings.Split(taskArn, "/")
	TaskID := seperatedTaskArn[len(seperatedTaskArn)-1]
	cluster := taskMetadata.Cluster
	seperatedCluster := strings.Split(cluster, "/")
	clusterName := seperatedCluster[len(seperatedCluster)-1]
	return clusterName, TaskID, nil
}

// fetchContainerId returns the containerId of the target
func fetchContainerId() (string, error) {
	containerMetadata, err := containerMetadataResponse()
	if err != nil {
		return "", err
	}
	return containerMetadata.ID, nil
}

// taskMetadataResponse returns taskMetadataResponse
func taskMetadataResponse() (taskMetadata *TaskResponse, err error) {
	lock.RLock()
	defer lock.RUnlock()
	if cachedTaskResponse != nil {
		return cachedTaskResponse, nil
	}

	cachedTaskResponse, err = getTaskMetadataResponse()
	return cachedTaskResponse, err
}

// containerMetadataResponse returns containerMetadataResponse
func containerMetadataResponse() (containerMetadata *ContainerResponse, err error) {
	lock.RLock()
	defer lock.RUnlock()
	if cachedContainerResponse != nil {
		return cachedContainerResponse, nil
	}

	cachedContainerResponse, err = getContainerMetadataResponse()
	return cachedContainerResponse, err
}

// getTaskMetadataResponse returns task metadata response
func getTaskMetadataResponse() (taskMetadata *TaskResponse, err error) {
	var taskResp []byte
	v3MetadataEndpoint, err := getV3MetadataEndpoint()
	if err != nil {
		return nil, err
	}

	taskResp, err = metadataResponse(v3MetadataEndpoint+"/task", "v3 task metadata")
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(taskResp, &taskMetadata)
	if err != nil {
		return nil, err
	}
	return
}

// getContainerMetadataResponse returns container metadata response
func getContainerMetadataResponse() (containerMetadata *ContainerResponse, err error) {
	var containerResp []byte
	v3MetadataEndpoint, err := getV3MetadataEndpoint()
	if err != nil {
		return nil, err
	}

	containerResp, err = metadataResponse(v3MetadataEndpoint, "v3 container metadata")
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(containerResp, &containerMetadata)
	if err != nil {
		return nil, err
	}
	return
}

// getV3MetadataEndpoint returns ECS metadata V3 base endpoint
var getV3MetadataEndpoint = func() (string, error) {
	// looks for the ECS_CONTAINER_METADATA_URI environment variables which contains the metadata endpoint V3
	// Please refer more info about ECS metadata via the link below
	// https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-metadata-endpoint-v3.html
	metadataEndpoint := os.Getenv(ContainerMetadataEnvVar)
	if metadataEndpoint != "" {
		return metadataEndpoint, nil
	}
	return "", fmt.Errorf("Could not fetch v3 metadata endpoint")
}
