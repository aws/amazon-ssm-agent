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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

// metadataResponse returns metadata response with retries
func metadataResponse(endpoint string, respType string) ([]byte, error) {
	var resp []byte
	var err error
	client := &http.Client{
		Timeout: 2 * time.Second,
	}
	for i := 0; i < maxRetries; i++ {
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
		return nil, fmt.Errorf("%s: incorrect status code %d", respType, resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: unable to read response body: %v", respType, err)
	}

	return body, nil
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

func fetchAvailabilityZone() (string, error) {
	taskMetadata, err := taskMetadataResponse()
	if err != nil {
		return "", err
	}

	return taskMetadata.AvailabilityZone, nil
}

// fetchClusterNameAndTaskID returns the clusterName and taskId of the target
func fetchClusterNameAndTaskID() (string, string, error) {
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

// fetchContainerID returns the containerId of the target
func fetchContainerID() (string, error) {
	containerMetadata, err := containerMetadataResponse()
	if err != nil {
		return "", err
	}
	return containerMetadata.ID, nil
}

// fetchCIDRBlock returns the CIDR block of the target
func fetchCIDRBlock() (map[string][]string, error) {
	containerMetadata, err := containerMetadataResponse()
	if err != nil {
		return map[string][]string{}, err
	}
	// only V4 metadata endpoint contains network information
	if len(containerMetadata.Networks) <= 0 {
		return map[string][]string{}, nil
	}
	return map[string][]string{"ipv4": {containerMetadata.Networks[0].IPv4SubnetCIDRBlock}, "ipv6": {containerMetadata.Networks[0].IPv6SubnetCIDRBlock}}, nil
}

// taskMetadataResponse returns taskMetadataResponse
func taskMetadataResponse() (taskMetadata *taskResponse, err error) {
	lock.RLock()
	defer lock.RUnlock()
	if cachedTaskResponse != nil {
		return cachedTaskResponse, nil
	}

	cachedTaskResponse, err = getTaskMetadataResponse()
	return cachedTaskResponse, err
}

// containerMetadataResponse returns containerMetadataResponse
func containerMetadataResponse() (containerMetadata *containerResponse, err error) {
	lock.RLock()
	defer lock.RUnlock()
	if cachedContainerResponse != nil {
		return cachedContainerResponse, nil
	}

	cachedContainerResponse, err = getContainerMetadataResponse()
	return cachedContainerResponse, err
}

// getTaskMetadataResponse returns task metadata response
func getTaskMetadataResponse() (taskMetadata *taskResponse, err error) {
	var taskResp []byte
	v3MetadataEndpoint, err := getMetadataEndpoint()
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
func getContainerMetadataResponse() (containerMetadata *containerResponse, err error) {
	var containerResp []byte
	v3MetadataEndpoint, err := getMetadataEndpoint()
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

// getMetadataEndpoint returns ECS metadata endpoint
var getMetadataEndpoint = func() (string, error) {
	// looks for the environment variables which contains the metadata endpoint V4, if not found fall back to V3
	// Please refer more info about ECS metadata via the links below
	// https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-metadata-endpoint-v4.html
	// https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-metadata-endpoint-v3.html
	metadataEndpoint := os.Getenv(containerMetadataEnvVarV4)
	if metadataEndpoint != "" {
		return metadataEndpoint, nil
	}
	metadataEndpoint = os.Getenv(containerMetadataEnvVarV3)
	if metadataEndpoint != "" {
		return metadataEndpoint, nil
	}
	return "", fmt.Errorf("Could not fetch metadata endpoint")
}
