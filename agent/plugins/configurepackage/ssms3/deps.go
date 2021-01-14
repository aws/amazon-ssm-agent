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
)

// dependency on S3 and downloaded artifacts
type networkDep interface {
	ListS3Folders(context context.T, amazonS3URL s3util.AmazonS3URL) (folderNames []string, err error)
	CanGetS3Object(context context.T, amazonS3URL s3util.AmazonS3URL) bool
	Download(context context.T, input artifact.DownloadInput) (artifact.DownloadOutput, error)
}

type networkDepImp struct{}

var networkdep networkDep = &networkDepImp{}

func (networkDepImp) ListS3Folders(context context.T, amazonS3URL s3util.AmazonS3URL) (folderNames []string, err error) {
	return artifact.ListS3Folders(context, amazonS3URL)
}

func (networkDepImp) CanGetS3Object(context context.T, amazonS3URL s3util.AmazonS3URL) bool {
	return artifact.CanGetS3Object(context, amazonS3URL)
}

func (networkDepImp) Download(context context.T, input artifact.DownloadInput) (artifact.DownloadOutput, error) {
	return artifact.Download(context, input)
}
