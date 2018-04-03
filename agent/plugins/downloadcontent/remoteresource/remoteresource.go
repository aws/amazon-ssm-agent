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

// Package remoteresource is the factory for creating and developing on multiple remote resources
package remoteresource

import (
	"github.com/aws/amazon-ssm-agent/agent/fileutil/filemanager"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

const (
	JSONExtension = ".json"
	YAMLExtension = ".yaml"
)

type DownloadResult struct {
	Files []string
}

// RemoteResource is an interface for accessing remote resources. Every type of remote resource is expected to implement RemoteResource interface
type RemoteResource interface {
	DownloadRemoteResource(log log.T, filesys filemanager.FileSystem, destinationDir string) (err error, result *DownloadResult)
	ValidateLocationInfo() (bool, error)
}
