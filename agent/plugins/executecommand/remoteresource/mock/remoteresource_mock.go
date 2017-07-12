// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package remoteresource_mock has mock functions for remoteresource package
package remoteresource_mock

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/filemanager"
	"github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/remoteresource"
	"github.com/stretchr/testify/mock"
)

type RemoteResourceMock struct {
	mock.Mock
}

func (resourceMock RemoteResourceMock) Download(log log.T, filesys filemanager.FileSystem, entireDir bool, destinationDir string) error {
	args := resourceMock.Called(log, filesys, entireDir, destinationDir)
	return args.Error(0)
}

func (resourceMock RemoteResourceMock) PopulateResourceInfo(log log.T, destinationDir string, entireDir bool) (resourceInfo remoteresource.ResourceInfo, err error) {
	args := resourceMock.Called(log, destinationDir, entireDir)
	return args.Get(0).(remoteresource.ResourceInfo), args.Error(1)
}

func (resourceMock RemoteResourceMock) ValidateLocationInfo() (bool, error) {
	args := resourceMock.Called()
	return args.Bool(0), args.Error(1)
}
