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

package ssms3

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/s3util"
	"github.com/stretchr/testify/mock"
)

// networkMock
type SSMS3Mock struct {
	mock.Mock
}

func (ds *SSMS3Mock) ListS3Folders(context context.T, amazonS3URL s3util.AmazonS3URL) (folderNames []string, err error) {
	args := ds.Called(context, amazonS3URL)
	return args.Get(0).([]string), args.Error(1)
}

func (ds *SSMS3Mock) CanGetS3Object(context context.T, amazonS3URL s3util.AmazonS3URL) bool {
	args := ds.Called(context, amazonS3URL)
	return args.Bool(0)
}

func (ds *SSMS3Mock) Download(context context.T, input artifact.DownloadInput) (artifact.DownloadOutput, error) {
	args := ds.Called(context, input)
	return args.Get(0).(artifact.DownloadOutput), args.Error(1)
}
