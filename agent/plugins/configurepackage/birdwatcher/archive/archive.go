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
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher"
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
	GetResourceVersion(packageName string, packageVersion string) (name string, version string)
	DownloadArchiveInfo(packageName string, version string) (string, error)
	GetFileDownloadLocation(file *File, packageName string, version string) (string, error)
}
