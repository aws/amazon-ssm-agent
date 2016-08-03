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

	// PackageNameFormat represents the package name format based on agreed convention
	PackageNameFormat = "{ComponentName}-{Arch}.{Compressed}"

	// PackageLocationFormat represents the package's s3 location
	PackageLocationFormat = "https://amazon-ssm-{Region}.s3.amazonaws.com/{ComponentName}/{Platform}/{PackageVersion}/{PackageName}"

	// PlatformNano represents Nano Server
	PlatformNano = "nano"
)

type Util interface {
	CreateComponentFolder(name string, version string) (folder string, err error)
}

type Utility struct{}

// createPackageName constructs the package name to locate in the s3 bucket
func createPackageName(componentName string, context *updateutil.InstanceContext) (packageName string) {
	packageName = PackageNameFormat

	packageName = strings.Replace(packageName, ComponentNameHolder, componentName, -1)
	packageName = strings.Replace(packageName, updateutil.ArchHolder, context.Arch, -1)
	packageName = strings.Replace(packageName, updateutil.CompressedHolder, context.CompressFormat, -1)

	return packageName
}

// createPackageLocation constructs the s3 url to locate the package for downloading
func createPackageLocation(componentName string, version string, context *updateutil.InstanceContext, packageName string) (packageLocation string) {
	// s3 uri format based on agreed convention
	packageLocation = PackageLocationFormat

	packageLocation = strings.Replace(packageLocation, updateutil.RegionHolder, context.Region, -1)
	packageLocation = strings.Replace(packageLocation, ComponentNameHolder, componentName, -1)
	packageLocation = strings.Replace(packageLocation, updateutil.PlatformHolder, context.Platform, -1)
	packageLocation = strings.Replace(packageLocation, updateutil.PackageVersionHolder, version, -1)
	packageLocation = strings.Replace(packageLocation, updateutil.PackageNameHolder, packageName, -1)

	return packageLocation
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
