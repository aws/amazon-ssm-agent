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
	// ComponentNameHolder represents Place holder for component name
	ComponentNameHolder = "{ComponentName}"

	// ManifestNameFormat represents the manifest name format
	ManifestNameFormat = "{ComponentName}.json"

	// PackageNameFormat represents the package name format based
	PackageNameFormat = "{ComponentName}.{Compressed}"

	// ComponentUrl represents the s3 location where all components live
	// the url to a specific package is this plus /{ComponentName}/{Platform}/{Arch}/{PackageVersion}/{FileName}
	//ComponentUrl = "https://amazon-ssm-{Region}.s3.amazonaws.com/Components"
	ComponentUrl = "https://s3-us-west-2.amazonaws.com/amazon.mattfo" // TODO:MF:testing

	// ComponentUrlBjs is the s3 location for BJS region where all components live
	ComponentUrlBjs = "https://s3.{Region}.amazonaws.com.cn/amazon-ssm-{Region}/Components"

	// RegionBjs represents the BJS region
	RegionBjs = "cn-north-1"

	// InstallAction represents the json command to install component
	InstallAction = "Install"

	// UninstallAction represents the json command to uninstall component
	UninstallAction = "Uninstall"

	// PatternVersion represents the regular expression for validating version
	PatternVersion = "^(?:(\\d+)\\.)(?:(\\d+)\\.)(\\d+)$"
)

type Util interface {
	CreateComponentFolder(name string, version string) (folder string, err error)
	HasValidPackage(name string, version string) bool
	GetCurrentVersion(name string) (installedVersion string)
	GetLatestVersion(log log.T, name string, source string, context *updateutil.InstanceContext) (latestVersion string, err error)
}

type Utility struct{}

// getManifestName constructs the manifest name to locate in the s3 bucket
func getManifestName(componentName string) (manifestName string) {
	manifestName = ManifestNameFormat
	manifestName = strings.Replace(manifestName, ComponentNameHolder, componentName, -1)

	return manifestName
}

// getPackageName constructs the package name to locate in the s3 bucket
func getPackageName(componentName string, context *updateutil.InstanceContext) (packageName string) {
	packageName = PackageNameFormat

	packageName = strings.Replace(packageName, ComponentNameHolder, componentName, -1)
	packageName = strings.Replace(packageName, updateutil.CompressedHolder, context.CompressFormat, -1)

	return packageName
}

// TODO:MF: Should we change this to URL instead of string?  Can we use URI instead of URL?
// getS3ComponentLocation returns the s3 location containing all versions of a component
func getS3ComponentLocation(componentName string, context *updateutil.InstanceContext) (s3Location string) {
	s3Url := getS3Url(componentName, context)
	s3Location = s3Url.String()
	return s3Location
}

// TODO:MF: Should we change this to URL instead of string?
// getS3Location constructs the s3 url to locate the package for downloading
func getS3Location(componentName string, version string, context *updateutil.InstanceContext, fileName string) (s3Location string) {
	s3Url := getS3Url(componentName, context)
	s3Url.Path = path.Join(s3Url.Path, version, fileName)

	s3Location = s3Url.String()
	return s3Location
}

// getS3Url returns the s3 location containing all versions of a component
func getS3Url(componentName string, context *updateutil.InstanceContext) (s3Url *url.URL) {
	// s3 uri format based on agreed convention
	// TO DO: Implement region/endpoint map (or integrate with aws sdk endpoints package) to handle cases better
	var s3Location string
	if context.Region == RegionBjs {
		s3Location = ComponentUrlBjs
	} else {
		s3Location = ComponentUrl
	}

	s3Url, _ = url.Parse(strings.Replace(s3Location, updateutil.RegionHolder, context.Region, -1))
	s3Url.Path = path.Join(s3Url.Path, componentName, context.Platform, context.Arch)
	return
}

// componentFolder returns the name of the component folder for a given version
func getComponentFolder(name string, version string) (folder string) {
	return filepath.Join(appconfig.ComponentRoot, name, version)
}

// CreateComponentFolder constructs the local directory to place component
func (util *Utility) CreateComponentFolder(name string, version string) (folder string, err error) {
	folder = getComponentFolder(name, version)

	if err = filesysdep.MakeDirExecute(folder); err != nil {
		return "", err
	}

	return folder, nil
}

// HasValidPackage determines if a given version of a component has a folder on disk that contains a valid package
func (util *Utility) HasValidPackage(name string, version string) bool {
	// folder exists, contains manifest, manifest is valid, and folder contains at least 1 other directory or file (assumed to be the unpacked package)
	componentFolder := getComponentFolder(name, version)
	manifestPath := filepath.Join(componentFolder, getManifestName(name))
	if !filesysdep.Exists(manifestPath) {
		return false
	}
	if _, err := parseComponentManifest(nil, manifestPath); err != nil {
		return false
	}
	files, _ := filesysdep.GetFileNames(componentFolder)
	directories, _ := filesysdep.GetDirectoryNames(componentFolder)
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
			// TODO:MF: can we clean this logic up just a bit?
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

// getLatestS3Version finds the most recent version of a component in S3
func getLatestS3Version(log log.T, name string, context *updateutil.InstanceContext) (latestVersion string, err error) {
	amazonS3URL := s3util.ParseAmazonS3URL(log, getS3Url(name, context))
	folders, err := networkdep.ListS3Folders(log, amazonS3URL)
	if err != nil {
		return
	}
	return getLatestVersion(folders[:], ""), nil
}

// GetCurrentVersion finds the most recent installed version of a component
func (util *Utility) GetCurrentVersion(name string) (installedVersion string) {
	directories, err := filesysdep.GetDirectoryNames(filepath.Join(appconfig.ComponentRoot, name))
	if err != nil {
		return ""
	}
	// TODO:MF determine the installing version (if there is one) and pass as second parameter
	return getLatestVersion(directories[:], "")
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
// GetLatestVersion looks up the latest version of a given component for this platform/arch in S3 or manifest at source location
func (util *Utility) GetLatestVersion(log log.T, name string, source string, context *updateutil.InstanceContext) (latestVersion string, err error) {
	if source != "" {
		// TODO:MF: Copy manifest from source location, parse, and return version
		return "1.0.0", nil
	} else {
		return getLatestS3Version(log, name, context)
	}
}
