// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package configurecomponent implements the ConfigureComponent plugin.
package configurecomponent

import (
	"path/filepath"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
)

const (
	// ComponentNameHolder represents Place holder for component name
	ComponentNameHolder = "{ComponentName}"

	// ManifestNameFormat represents the manifest name format
	ManifestNameFormat = "{ComponentName}.json"

	// PackageNameFormat represents the package name format based
	PackageNameFormat = "{ComponentName}-{Arch}.{Compressed}"

	// PackageLocationFormat represents the package's s3 location
	SourceFormat = "https://amazon-ssm-{Region}.s3.amazonaws.com/{ComponentName}/{Platform}/{PackageVersion}/{FileName}"

	// PlatformNano represents Nano Server
	PlatformNano = "nano"
)

type Util interface {
	CreateComponentFolder(name string, version string) (folder string, err error)
}

type Utility struct{}

func createManifestName(componentName string) (manifestName string) {
	manifestName = ManifestNameFormat
	manifestName = strings.Replace(manifestName, ComponentNameHolder, componentName, -1)

	return manifestName
}

// createPackageName constructs the package name to locate in the s3 bucket
func createPackageName(componentName string, context *updateutil.InstanceContext) (packageName string) {
	packageName = PackageNameFormat

	packageName = strings.Replace(packageName, ComponentNameHolder, componentName, -1)
	packageName = strings.Replace(packageName, updateutil.ArchHolder, context.Arch, -1)
	packageName = strings.Replace(packageName, updateutil.CompressedHolder, context.CompressFormat, -1)

	return packageName
}

// createPackageLocation constructs the s3 url to locate the package for downloading
func createS3Location(componentName string, version string, context *updateutil.InstanceContext, fileName string) (s3Location string) {
	// s3 uri format based on agreed convention
	s3Location = SourceFormat

	s3Location = strings.Replace(s3Location, updateutil.RegionHolder, context.Region, -1)
	s3Location = strings.Replace(s3Location, ComponentNameHolder, componentName, -1)
	s3Location = strings.Replace(s3Location, updateutil.PlatformHolder, context.Platform, -1)
	s3Location = strings.Replace(s3Location, updateutil.PackageVersionHolder, version, -1)
	s3Location = strings.Replace(s3Location, updateutil.FileNameHolder, fileName, -1)

	return s3Location
}

var mkDirAll = fileutil.MakeDirsWithExecuteAccess

// CreateComponentFolder constructs the local directory to place component
func (util *Utility) CreateComponentFolder(name string, version string) (folder string, err error) {
	folder = filepath.Join(appconfig.ComponentRoot, name, version)
	if err = mkDirAll(folder); err != nil {
		return "", err
	}

	return folder, nil
}
