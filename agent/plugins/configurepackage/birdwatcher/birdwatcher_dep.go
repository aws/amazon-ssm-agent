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

package birdwatcher

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
)

// dependency on S3 and downloaded artifacts
type networkDep interface {
	Download(context context.T, input artifact.DownloadInput) (artifact.DownloadOutput, error)
}

var Networkdep networkDep = &networkDepImp{}

type networkDepImp struct{}

func (networkDepImp) Download(context context.T, input artifact.DownloadInput) (artifact.DownloadOutput, error) {
	return artifact.Download(context, input)
}
