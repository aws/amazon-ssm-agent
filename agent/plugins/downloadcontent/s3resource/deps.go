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
)

// dependency on S3 and downloaded artifacts
type s3deps interface {
	ListS3Directory(log log.T, amazonS3URL s3util.AmazonS3URL) (folderNames []string, err error)
	Download(log log.T, input artifact.DownloadInput) (artifact.DownloadOutput, error)
}

type s3DepImpl struct{}

var dep s3deps = &s3DepImpl{}

//TODO: Refactor the code to merge the s3 capabilities to one package
func (s3DepImpl) ListS3Directory(log log.T, amazonS3URL s3util.AmazonS3URL) (folderNames []string, err error) {
	return artifact.ListS3Directory(log, amazonS3URL)
}

func (s3DepImpl) Download(log log.T, input artifact.DownloadInput) (artifact.DownloadOutput, error) {
	return artifact.Download(log, input)
}
