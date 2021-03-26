// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package updatemanifest implements the logic for the ssm agent s3 manifest.
package updatemanifest

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateinfo"
)

const (
	// version status of SSM agent
	VersionStatusActive     = "Active"
	VersionStatusInactive   = "Inactive"
	VersionStatusDeprecated = "Deprecated"
)

type T interface {
	LoadManifest(manifestPath string) error
	HasVersion(packageName string, version string) bool
	GetLatestVersion(packageName string) (string, error)
	GetLatestActiveVersion(packageName string) (string, error)
	GetDownloadURLAndHash(packageName string, version string) (string, string, error)
	IsVersionDeprecated(packageName string, version string) (bool, error)
	IsVersionActive(packageName string, version string) (bool, error)
}

type manifestImpl struct {
	context  context.T
	info     updateinfo.T
	manifest *jsonManifest
}

// jsonManifest represents the json structure of online manifest file.
type jsonManifest struct {
	SchemaVersion string            `json:"SchemaVersion"`
	URIFormat     string            `json:"UriFormat"`
	Packages      []*packageContent `json:"Packages"`
}

// packageContent section in the Manifest json.
type packageContent struct {
	Name  string         `json:"Name"`
	Files []*fileContent `json:"Files"`
}

// fileContent holds the file name and available versions
type fileContent struct {
	Name              string            `json:"Name"`
	AvailableVersions []*packageVersion `json:"AvailableVersions"`
}

// packageVersion section in the PackageContent
type packageVersion struct {
	Version  string `json:"Version"`
	Checksum string `json:"Checksum"`
	Status   string `json:"Status"`
}
