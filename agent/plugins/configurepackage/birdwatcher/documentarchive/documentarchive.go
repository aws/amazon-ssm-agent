// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package documentarchive contains the struct that is called when the package information is stored in birdwatcher
package documentarchive

import (
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/archive"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/facade"
)

type PackageArchive struct {
	facadeClient facade.BirdwatcherFacade
}

// New is a constructor for PackageArchive struct
func New(facadeClientSession facade.BirdwatcherFacade) archive.IPackageArchive {
	return &PackageArchive{
		facadeClient: facadeClientSession,
	}
}

// GetResourceVersion makes a call to birdwatcher API to figure the right version of the resource that needs to be installed
func (da *PackageArchive) GetResourceVersion(packageName string, packageVersion string) (name string, version string) {
	// Return the packageVersion as "" if empty and return version if specified.
	return packageName, packageVersion
}

// DownloadArtifactInfo downloads the document using GetDocument and eventually gets the manifest from that
func (da *PackageArchive) DownloadArchiveInfo(packageName string, version string) (string, error) {

	// TODO: Add implementation
	return "", nil
}
