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

// FileInfo contains data for one SSM package
type FileInfo struct {
	Checksums        map[string]string `json:"checksums"`
	DownloadLocation string            `json:"downloadLocation"`
	Size             int               `json:"size"`
}

// PackageInfo contains references to Files matching the current platform/version/arch
type PackageInfo struct {
	FileName string `json:"file"`
}

// Manifest contains references to all SSM packages for a given agent version
type Manifest struct {
	SchemaVersion string `json:"schemaVersion"`
	PackageArn    string `json:"packageArn"`
	Version       string `json:"version"`

	// platform -> version -> arch -> file
	Packages map[string]map[string]map[string]*PackageInfo `json:"packages"`
	Files    map[string]*FileInfo                          `json:"files"`
}
