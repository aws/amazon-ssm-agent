// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package service is a wrapper for the new Service
package service

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/gorilla/websocket"
)

// Service is an interface to the HM service operation v1.
type Service interface {
	CreateChannel(log log.T, instanceID string) (channelId string, err error)
	GetChannel(log log.T, channelId string) (*websocket.Conn, error)
	DeleteChannel(log log.T, channelId string) error
}

// sdkService is an service wrapper that delegates to the hm service sdk.
type sdkService struct{}

// NewService creates a new service instance.
func NewService(region string, endpoint string, creds *credentials.Credentials) Service {

	config := sdkutil.AwsConfig()

	if region != "" {
		config.Region = aws.String(region)
	}

	if endpoint != "" {
		config.Endpoint = aws.String(endpoint)
	}

	if creds != nil {
		config.Credentials = creds
	}

	return &sdkService{}
}

// CreateChannel makes POST request to service to get channel id
func (hmService *sdkService) CreateChannel(log log.T, instanceID string) (channelId string, err error) {
	return "", nil
}

// GetChannel makes GET request to service to open web socket connection
func (hmService *sdkService) GetChannel(log log.T, channelId string) (*websocket.Conn, error) {
	return nil, nil
}

// DeleteChannel closes the web socket
func (hmService *sdkService) DeleteChannel(log log.T, channelId string) error {
	return nil
}
