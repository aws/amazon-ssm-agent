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
	"runtime"

	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
)

// dependency on S3 and downloaded artifacts
type networkDep interface {
	Download(log log.T, input artifact.DownloadInput) (artifact.DownloadOutput, error)
}

var networkdep networkDep = &networkDepImp{}

type networkDepImp struct{}

func (networkDepImp) Download(log log.T, input artifact.DownloadInput) (artifact.DownloadOutput, error) {
	return artifact.Download(log, input)
}

// dependency on platform detection
type platformProviderDep interface {
	Name(log log.T) (string, error)
	Version(log log.T) (string, error)
	Architecture(log log.T) (string, error)
}

var platformProviderdep platformProviderDep = &platformProviderDepImp{}

type platformProviderDepImp struct{}

func (*platformProviderDepImp) Name(log log.T) (string, error) {
	return platform.PlatformName(log)
}

func (*platformProviderDepImp) Version(log log.T) (string, error) {
	return platform.PlatformVersion(log)
}

func (*platformProviderDepImp) Architecture(log log.T) (string, error) {
	return runtime.GOARCH, nil
}
