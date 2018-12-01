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

//package archive holds the resources for the archive
package archive

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/packageservice"
)

const (
	PackageArchiveBirdwatcher = "birdwatcher"
	PackageArchiveDocument    = "document"
)

type File struct {
	Name string
	Info birdwatcher.FileInfo
}

type IPackageArchive interface {
	Name() string
	//TODO: Send this by address or reference
	SetManifestCache(cache packageservice.ManifestCache)
	SetResource(*birdwatcher.Manifest)
	GetResourceVersion(packageName string, packageVersion string) (name string, version string)
	GetResourceArn() string
	GetFileDownloadLocation(file *File, packageName string, version string) (string, error)
	DownloadArchiveInfo(packageName string, version string) (string, error)
	ReadManifestFromCache() (*birdwatcher.Manifest, error)
	WriteManifestToCache(manifest []byte) error
}

func ParseManifest(data *[]byte) (*birdwatcher.Manifest, error) {
	var manifest birdwatcher.Manifest

	// TODO: additional validation
	if err := json.NewDecoder(bytes.NewReader(*data)).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("failed to decode manifest: %v", err)
	}

	return &manifest, nil
}
