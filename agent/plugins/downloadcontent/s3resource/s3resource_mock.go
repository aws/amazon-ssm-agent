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

package s3resource

import (
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/s3util"
	"github.com/stretchr/testify/mock"
)

// s3Mock
type s3DepMock struct {
	mock.Mock
}

func (s3 s3DepMock) ListS3Directory(log log.T, amazonS3URL s3util.AmazonS3URL) (folderNames []string, err error) {
	args := s3.Called(log, amazonS3URL)
	return args.Get(0).([]string), args.Error(1)
}

func (s3 s3DepMock) Download(log log.T, input artifact.DownloadInput) (artifact.DownloadOutput, error) {
	args := s3.Called(log, input)
	return args.Get(0).(artifact.DownloadOutput), args.Error(1)
}
