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
	"github.com/aws/amazon-ssm-agent/agent/fileutil/filemanager"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/remoteresource"
	"github.com/stretchr/testify/mock"
)

type RemoteResourceMock struct {
	mock.Mock
}

func (resourceMock RemoteResourceMock) DownloadRemoteResource(log log.T, filesys filemanager.FileSystem, destinationDir string) (err error, result *remoteresource.DownloadResult) {
	args := resourceMock.Called(log, filesys, destinationDir)
	return args.Error(0), args.Get(1).(*remoteresource.DownloadResult)
}

func (resourceMock RemoteResourceMock) ValidateLocationInfo() (bool, error) {
	args := resourceMock.Called()
	return args.Bool(0), args.Error(1)
}

func NewEmptyDownloadResult() *remoteresource.DownloadResult {
	return &remoteresource.DownloadResult{Files: []string{}}
}

func NewDownloadResult(files []string) *remoteresource.DownloadResult {
	return &remoteresource.DownloadResult{Files: files}
}
