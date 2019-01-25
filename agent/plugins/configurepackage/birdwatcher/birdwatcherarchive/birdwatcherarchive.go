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
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/trace"
	"github.com/aws/aws-sdk-go/service/ssm"
)

type PackageArchive struct {
	facadeClient   facade.BirdwatcherFacade
	archiveType    string
	cache          packageservice.ManifestCache
	localManifests map[string]*localManifest
}

type localManifest struct {
	manifestString  string
	packageArn      string
	manifestVersion string
}

// New is a constructor for PackageArchive struct
func New(facadeClientSession facade.BirdwatcherFacade, context map[string]string) archive.IPackageArchive {
	// TODO: Add a SetManifest method for PackageArchive to avoid the birdwatcherManifest in the constructor.
	manifests := make(map[string]*localManifest)
	setLocalManifestString(manifests, context)
	return &PackageArchive{
		facadeClient:   facadeClientSession,
		archiveType:    archive.PackageArchiveBirdwatcher,
		localManifests: manifests,
	}
}

// Name of archive type
func (ba *PackageArchive) Name() string {
	return ba.archiveType
}

// SetManifestCache sets the manifest cache
func (ba *PackageArchive) SetManifestCache(manifestCache packageservice.ManifestCache) {
	ba.cache = manifestCache
}

// SetResource sets the package name and the manifest version
func (ba *PackageArchive) SetResource(packageName string, version string, manifest *birdwatcher.Manifest) {
	key := archive.FormKey(packageName, version)
	if _, ok := ba.localManifests[key]; !ok {
		ba.localManifests[key] = &localManifest{}
	}

	ba.localManifests[key].packageArn = manifest.PackageArn
	ba.localManifests[key].manifestVersion = manifest.Version
}

// GetResourceArn returns the packageArn that is found i nthe manifest file
func (ba *PackageArchive) GetResourceArn(packageName string, version string) string {
	key := archive.FormKey(packageName, version)
	if _, ok := ba.localManifests[key]; !ok {
		return ""
	}

	return ba.localManifests[key].packageArn
}

// GetResourceVersion returns the version
func (ba *PackageArchive) GetResourceVersion(packageName string, packageVersion string) (name string, version string) {
	version = packageVersion
	if packageservice.IsLatest(packageVersion) {
		version = packageservice.Latest
	}

	return packageName, version
}

// GetFileDownloadLocation obtains the location of the file in the archive
func (ba *PackageArchive) GetFileDownloadLocation(file *archive.File, packageName string, version string) (string, error) {
	if file == nil {
		return "", fmt.Errorf("file is empty")
	}
	return file.Info.DownloadLocation, nil
}

// DownloadArtifactInfo downloads the manifest for the original birwatcher service
func (ba *PackageArchive) DownloadArchiveInfo(tracer trace.Tracer, packageName string, version string) (string, error) {
	trace := tracer.BeginSection("Downloading birdwatcher archive info")
	defer trace.End()

	key := archive.FormKey(packageName, version)
	if _, ok := ba.localManifests[key]; !ok {
		ba.localManifests[key] = &localManifest{}
	}

	if ba.localManifests[key].manifestString == "" {
		trace.AppendDebugf("Cannot find manifest with key: %v in localManifests, downloading from remote.", key)
		resp, err := ba.facadeClient.GetManifest(
			&ssm.GetManifestInput{
				PackageName:    &packageName,
				PackageVersion: &version,
			},
		)

		if err != nil {
			return "", fmt.Errorf("failed to retrieve manifest: %v", err)
		}

		ba.localManifests[key].manifestString = *resp.Manifest
	} else {
		trace.AppendDebugf("Found manifest with key: %v in localManifests", key)
	}

	return ba.localManifests[key].manifestString, nil
}

// ReadManifestFromCache to read the manifest from cache
// Birdwatcher packages store the manifest with the package version
func (ba *PackageArchive) ReadManifestFromCache(packageArn string, version string) (*birdwatcher.Manifest, error) {
	data, err := ba.cache.ReadManifest(packageArn, version)
	if err != nil {
		return nil, err
	}

	return archive.ParseManifest(&data)
}

// WriteManifestToCache stores the manifest in cache
func (ba *PackageArchive) WriteManifestToCache(packageArn string, version string, manifest []byte) error {
	return ba.cache.WriteManifest(packageArn, version, manifest)
}

// Sets the manifest string in the localManifest
func setLocalManifestString(localManifests map[string]*localManifest, context map[string]string) {
	if name, nOk := context["packageName"]; nOk {
		if version, vOk := context["packageVersion"]; vOk {
			if manifest, mOk := context["manifest"]; mOk {
				key := archive.FormKey(name, version)
				if _, ok := localManifests[key]; !ok {
					localManifests[key] = &localManifest{}
				}

				localManifests[key].manifestString = manifest
			}
		}
	}
}
