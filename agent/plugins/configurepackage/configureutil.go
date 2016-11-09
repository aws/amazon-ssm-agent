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

// Package configurepackage implements the ConfigurePackage plugin.
package configurepackage

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/s3util"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
)

const (
	// PackageNameHolder represents Place holder for package name
	PackageNameHolder = "{PackageName}"

	// ManifestNameFormat represents the manifest name format
	ManifestNameFormat = "{PackageName}.json"

	// PackageNameFormat represents the package name format based
	PackageNameFormat = "{PackageName}.{Compressed}"

	// PackageUrl represents the s3 location where all packages live
	// the url to a specific package is this plus /{PackageName}/{Platform}/{Arch}/{PackageVersion}/{FileName}
	PackageUrl = "https://s3.{Region}.amazonaws.com/amazon-ssm-packages-{Region}/Packages"

	// PackageUrlBjs is the s3 location for BJS region where all packages live
	PackageUrlBjs = "https://s3.{Region}.amazonaws.com.cn/amazon-ssm-packages-{Region}/Packages"

	// RegionBjs represents the BJS region
	RegionBjs = "cn-north-1"

	// InstallAction represents the json command to install package
	InstallAction = "Install"

	// UninstallAction represents the json command to uninstall package
	UninstallAction = "Uninstall"

	// PatternVersion represents the regular expression for validating version
	PatternVersion = "^(?:(\\d+)\\.)(?:(\\d+)\\.)(\\d+)$"
)

type configureUtil interface {
	CreatePackageFolder(name string, version string) (folder string, err error)
	HasValidPackage(name string, version string) bool
	GetCurrentVersion(name string) (installedVersion string)
	GetLatestVersion(log log.T, name string, context *updateutil.InstanceContext) (latestVersion string, err error)
}

type configureUtilImp struct{}

// getManifestName constructs the manifest name to locate in the s3 bucket
func getManifestName(packageName string) (manifestName string) {
	manifestName = ManifestNameFormat
	manifestName = strings.Replace(manifestName, PackageNameHolder, packageName, -1)

	return manifestName
}

// getPackageFilename constructs the package name to locate in the s3 bucket
func getPackageFilename(packageName string, context *updateutil.InstanceContext) (packageFilename string) {
	packageFilename = PackageNameFormat

	packageFilename = strings.Replace(packageFilename, PackageNameHolder, packageName, -1)
	packageFilename = strings.Replace(packageFilename, updateutil.CompressedHolder, context.CompressFormat, -1)

	return packageFilename
}

// TODO:MF: Should we change this to URL instead of string?  Can we use URI instead of URL?
// getS3PackageLocation returns the s3 location containing all versions of a package
func getS3PackageLocation(packageName string, context *updateutil.InstanceContext) (s3Location string) {
	s3Url := getS3Url(packageName, context)
	s3Location = s3Url.String()
	return s3Location
}

// TODO:MF: Should we change this to URL instead of string?
// getS3Location constructs the s3 url to locate the package for downloading
func getS3Location(packageName string, version string, context *updateutil.InstanceContext, fileName string) (s3Location string) {
	s3Url := getS3Url(packageName, context)
	s3Url.Path = path.Join(s3Url.Path, version, fileName)

	s3Location = s3Url.String()
	return s3Location
}

// getS3Url returns the s3 location containing all versions of a package
func getS3Url(packageName string, context *updateutil.InstanceContext) (s3Url *url.URL) {
	// s3 uri format based on agreed convention
	// TO DO: Implement region/endpoint map (or integrate with aws sdk endpoints package) to handle cases better
	var s3Location string
	if context.Region == RegionBjs {
		s3Location = PackageUrlBjs
	} else {
		s3Location = PackageUrl
	}

	s3Url, _ = url.Parse(strings.Replace(s3Location, updateutil.RegionHolder, context.Region, -1))
	s3Url.Path = path.Join(s3Url.Path, packageName, appconfig.PackagePlatform, context.Arch)
	return
}

// getPackageRoot returns the name of the folder containing all versions of a package
func getPackageRoot(name string) (folder string) {
	return filepath.Join(appconfig.PackageRoot, name)
}

// getPackageFolder returns the name of the package folder for a given version
func getPackageFolder(name string, version string) (folder string) {
	return filepath.Join(getPackageRoot(name), version)
}

// CreatePackageFolder constructs the local directory to place package
func (util *configureUtilImp) CreatePackageFolder(name string, version string) (folder string, err error) {
	folder = getPackageFolder(name, version)

	if err = filesysdep.MakeDirExecute(folder); err != nil {
		return "", err
	}

	return folder, nil
}

// HasValidPackage determines if a given version of a package has a folder on disk that contains a valid package
func (util *configureUtilImp) HasValidPackage(name string, version string) bool {
	// folder exists, contains manifest, manifest is valid, and folder contains at least 1 other directory or file (assumed to be the unpacked package)
	packageFolder := getPackageFolder(name, version)
	manifestPath := filepath.Join(packageFolder, getManifestName(name))
	if !filesysdep.Exists(manifestPath) {
		return false
	}
	if _, err := parsePackageManifest(nil, manifestPath); err != nil {
		return false
	}
	files, _ := filesysdep.GetFileNames(packageFolder)
	directories, _ := filesysdep.GetDirectoryNames(packageFolder)
	if len(files) <= 1 && len(directories) == 0 {
		return false
	}
	return true
}

// getLatestVersion returns the latest version given a list of version strings (that match PatternVersion)
func getLatestVersion(versions []string, except string) (version string) {
	var latestVersion string = ""
	var latestMajor, latestMinor, latestBuild = -1, -1, -1
	for _, version := range versions {
		if version == except {
			continue
		}
		if major, minor, build, err := parseVersion(version); err == nil {
			if major < latestMajor {
				continue
			} else if major == latestMajor && minor < latestMinor {
				continue
			} else if major == latestMajor && minor == latestMinor && build <= latestBuild {
				continue
			}
			latestMajor = major
			latestMinor = minor
			latestBuild = build
			latestVersion = version
		}
	}
	return latestVersion
}

// getLatestS3Version finds the most recent version of a package in S3
func getLatestS3Version(log log.T, name string, context *updateutil.InstanceContext) (latestVersion string, err error) {
	amazonS3URL := s3util.ParseAmazonS3URL(log, getS3Url(name, context))
	log.Debugf("looking up latest version of %v from %v", name, amazonS3URL.String())
	folders, err := networkdep.ListS3Folders(log, amazonS3URL)
	if err != nil {
		return
	}
	return getLatestVersion(folders[:], ""), nil
}

// GetCurrentVersion finds the most recent installed version of a package
func (util *configureUtilImp) GetCurrentVersion(name string) (installedVersion string) {
	directories, err := filesysdep.GetDirectoryNames(filepath.Join(appconfig.PackageRoot, name))
	if err != nil {
		return ""
	}
	return getLatestVersion(directories[:], getInstallingPackageVersion(name))
}

// parseVersion returns the major, minor, and build parts of a valid version string and an error if the string is not valid
func parseVersion(version string) (major int, minor int, build int, err error) {
	if matched, err := regexp.MatchString(PatternVersion, version); matched == false || err != nil {
		return 0, 0, 0, fmt.Errorf("invalid version string %v", version)
	}
	versionParts := strings.Split(version, ".")
	if major, err = strconv.Atoi(versionParts[0]); err != nil {
		return
	}
	if minor, err = strconv.Atoi(versionParts[1]); err != nil {
		return
	}
	if build, err = strconv.Atoi(versionParts[2]); err != nil {
		return
	}
	return
}

// TODO:MF: This is the first utility function that calls out to S3 or some URI - perhaps this is part of a different set of utilities
// GetLatestVersion looks up the latest version of a given package for this platform/arch in S3 or manifest at source location
func (util *configureUtilImp) GetLatestVersion(log log.T, name string, context *updateutil.InstanceContext) (latestVersion string, err error) {
	// TODO:OFFLINE: Copy manifest from source location, parse, and return version
	latestVersion, err = getLatestS3Version(log, name, context)
	// handle case where we couldn't figure out which version to install but not because of an error in the S3 call
	if latestVersion == "" {
		return "", fmt.Errorf("no latest version found for package %v on platform %v[%v]", name, appconfig.PackagePlatform, context.Arch)
	}
	return latestVersion, err
}
