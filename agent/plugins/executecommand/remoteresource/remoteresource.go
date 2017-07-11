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
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/filemanager"
)

const (
	JSONExtension = ".json"

	Unknown = iota
	Script
	Document
)

// RemoteResource is an interface for accessing remote resources. Every type of remote resource is expected to implement RemoteResource interface
type RemoteResource interface {
	Download(log log.T, filesys filemanager.FileSystem, entireDir bool, destinationDir string) error
	PopulateResourceInfo(log log.T, destinationDir string, entireDir bool) (resourceInfo ResourceInfo, err error)
	ValidateLocationInfo() (bool, error)
}

// ResourceInfo represents the required information after downloading the remote resource
type ResourceInfo struct {
	LocalDestinationPath string //Local path to the file to be executed
	EntireDir            bool
	TypeOfResource       int    //Valid values are either Unknown, Script or Document
	StarterFile          string //Name of the file
}
