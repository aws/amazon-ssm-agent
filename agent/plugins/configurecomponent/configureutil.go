// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
	SourceFormat = "https://amazon-ssm-{Region}.s3.amazonaws.com/Components/{ComponentName}/{Platform}/{PackageVersion}/{FileName}"

	// SourceFormatBjs represents the package's s3 location for BJS region
	SourceFormatBjs = "https://s3.{Region}.amazonaws.com.cn/amazon-ssm-{Region}/Components/{ComponentName}/{Platform}/{PackageVersion}/{FileName}"

	// RegionBjs represents the BJS region
	RegionBjs = "cn-north-1"

	// InstallAction represents the json command to install component
	InstallAction = "Install"

	// UninstallAction represents the json command to uninstall component
	UninstallAction = "Uninstall"

	// PlatformNano represents Nano Server
	PlatformNano = "nano"
)

type Util interface {
	CreateComponentFolder(name string, version string) (folder string, err error)
}

type Utility struct{}

// createManifestName constructs the manifest name to locate in the s3 bucket
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
	// TO DO: Implement region/endpoint map (or integrate with aws sdk endpoints package) to handle cases better
	if context.Region == RegionBjs {
		s3Location = SourceFormatBjs
	} else {
		s3Location = SourceFormat
	}

	s3Location = strings.Replace(s3Location, updateutil.RegionHolder, context.Region, -1)
	s3Location = strings.Replace(s3Location, ComponentNameHolder, componentName, -1)
	s3Location = strings.Replace(s3Location, updateutil.PlatformHolder, context.Platform, -1)
	s3Location = strings.Replace(s3Location, updateutil.PackageVersionHolder, version, -1)
	s3Location = strings.Replace(s3Location, updateutil.FileNameHolder, fileName, -1)

	return s3Location
}

var mkDirAll = fileutil.MakeDirsWithExecuteAccess
var componentExists = fileutil.Exists
var versionExists = fileutil.Exists

// CreateComponentFolder constructs the local directory to place component
func (util *Utility) CreateComponentFolder(name string, version string) (folder string, err error) {
	folder = filepath.Join(appconfig.ComponentRoot, name, version)

	if err = mkDirAll(folder); err != nil {
		return "", err
	}

	return folder, nil
}

// needUpdate determines if installation needs to update an existing version of a component
func needUpdate(name string, requestedVersion string) (update bool) {
	// check that any version is already installed
	componentFolder := filepath.Join(appconfig.ComponentRoot, name)
	exist := componentExists(componentFolder)

	// install as normal when component is not yet installed
	if !exist {
		return false
	}

	// check that specific version is already installed
	versionFolder := filepath.Join(componentFolder, requestedVersion)
	exist = versionExists(versionFolder)

	// install as normal when component version is already installed
	if exist {
		return false
	}

	return true
}
