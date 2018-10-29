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

	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/archive"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/facade"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/packageservice"
	"github.com/aws/aws-sdk-go/service/ssm"
)

type PackageArchive struct {
	facadeClient facade.BirdwatcherFacade
	archiveType  string
}

// New is a constructor for PackageArchive struct
func New(facadeClientSession facade.BirdwatcherFacade) archive.IPackageArchive {
	return &PackageArchive{
		facadeClient: facadeClientSession,
		archiveType:  archive.PackageArchiveDocument,
	}
}

// Name of archive type
func (ba *PackageArchive) Name() string {
	return ba.archiveType
}

func (ba *PackageArchive) GetResourceVersion(packageName string, packageVersion string) (name string, version string) {
	version = packageVersion
	if packageservice.IsLatest(packageVersion) {
		version = packageservice.Latest
	}

	return packageName, version
}

// DownloadArtifactInfo downloads the manifest for the original birwatcher service
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

// GetFileDownloadLocation obtains the location of the file in the archive
func (ba *PackageArchive) GetFileDownloadLocation(file *archive.File, packageName string, version string) (string, error) {
	if file == nil {
		return "", fmt.Errorf("file is empty")
	}
	return file.Info.DownloadLocation, nil
}

// GetResourceArn returns the packageArn that is found i nthe manifest file
func (ba *PackageArchive) GetResourceArn(manifest *birdwatcher.Manifest) string {
	return manifest.PackageArn
}
