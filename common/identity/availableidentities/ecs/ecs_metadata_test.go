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

package ecs

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	fakeV3Endpoint = "endpoint"
)

func TestFetchClusterNameAndTaskIdSuccess(t *testing.T) {
	metadataResponseOnce = func(client *http.Client, endpoint string, respType string) ([]byte, error) {
		return []byte("{\"Cluster\": \"clusterName\"," +
			"\"TaskARN\": \"arn:aws:ecs:us-east-2:012345678910:task/9781c248-0edd-4cdb-9a93-f63cb662a5d3\"}"), nil
	}
	getMetadataEndpoint = func() (string, error) {
		return fakeV3Endpoint, nil
	}

	clusterName, taskId, err := fetchClusterNameAndTaskID()
	assert.Nil(t, err)
	assert.Equal(t, "clusterName", clusterName)
	assert.Equal(t, "9781c248-0edd-4cdb-9a93-f63cb662a5d3", taskId)
}

func TestFetchClusterNameAndTaskIdSuccessForFargate(t *testing.T) {
	metadataResponseOnce = func(client *http.Client, endpoint string, respType string) ([]byte, error) {
		return []byte("{\"Cluster\": \"arn:aws:ecs:us-east-2:012345678910:cluster/clusterName\"," +
			"\"TaskARN\": \"arn:aws:ecs:us-east-2:012345678910:task/9781c248-0edd-4cdb-9a93-f63cb662a5d3\"}"), nil
	}
	getMetadataEndpoint = func() (string, error) {
		return fakeV3Endpoint, nil
	}

	clusterName, taskId, err := fetchClusterNameAndTaskID()
	assert.Nil(t, err)
	assert.Equal(t, "clusterName", clusterName)
	assert.Equal(t, "9781c248-0edd-4cdb-9a93-f63cb662a5d3", taskId)
}

func TestFetchContainerIdSuccess(t *testing.T) {
	metadataResponseOnce = func(client *http.Client, endpoint string, respType string) ([]byte, error) {
		return []byte("{\"DockerId\": \"1234567890\"," +
			"\"Name\": \"dockerName\"}"), nil
	}
	getMetadataEndpoint = func() (string, error) {
		return fakeV3Endpoint, nil
	}

	containerID, err := fetchContainerID()
	assert.Nil(t, err)
	assert.Equal(t, "1234567890", containerID)
}

func TestFetchRegionSuccess(t *testing.T) {
	metadataResponseOnce = func(client *http.Client, endpoint string, respType string) ([]byte, error) {
		return []byte("{\"Cluster\": \"clusterName\"," +
			"\"TaskARN\": \"arn:aws:ecs:us-east-2:012345678910:task/9781c248-0edd-4cdb-9a93-f63cb662a5d3\"}"), nil
	}
	getMetadataEndpoint = func() (string, error) {
		return fakeV3Endpoint, nil
	}

	region, err := fetchRegion()
	assert.Nil(t, err)
	assert.Equal(t, "us-east-2", region)
}

func TestMetadataResponseWithRetries(t *testing.T) {
	tmp := 0
	metadataResponseOnce = func(client *http.Client, endpoint string, respType string) ([]byte, error) {
		tmp = tmp + 1
		return nil, fmt.Errorf("unable to make http call")
	}
	_, err := metadataResponse(fakeV3Endpoint, "task")

	assert.NotNil(t, err)
	assert.Equal(t, 4, tmp)
}
