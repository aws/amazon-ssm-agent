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

// Package birdwatcherarchive contains the struct that is called when the package information is stored in birdwatcher
package birdwatcherarchive

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/archive"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/facade"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/packageservice"
	"github.com/aws/aws-sdk-go/service/ssm"
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

func (ba *PackageArchive) GetResourceVersion(packageName string, version string) (names []string, versions []string) {
	packageVersion := version
	if packageservice.IsLatest(version) {
		packageVersion = packageservice.Latest
	}
	names = make([]string, 1)
	versions = make([]string, 1)
	names[0] = packageName
	versions[0] = packageVersion

	return
}

// // DownloadArtifactInfo downloads the manifest for the original birwatcher service
func (ba *PackageArchive) DownloadArchiveInfo(packageName string, version string) (string, error) {

	resp, err := ba.facadeClient.GetManifest(
		&ssm.GetManifestInput{
			PackageName:    &packageName,
			PackageVersion: &version,
		},
	)

	if err != nil {
		return "", fmt.Errorf("failed to retrieve manifest: %v", err)
	}
	return *resp.Manifest, nil
}
