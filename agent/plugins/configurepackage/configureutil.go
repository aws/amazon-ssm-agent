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
	"regexp"
	"strconv"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/s3util"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
)

const (
	// PackageNameFormat represents the package name format based
	PackageNameFormat = "{PackageName}.{Compressed}"

	// PackageUrlStandard represents the s3 folder where all versions of a package live
	// the url to a specific package has a format like https://s3.us-east-1.amazonaws.com/amazon-ssm-packages-us-east-1/Packages/Test/windows/amd64/1.0.0/Test.zip
	PackageUrlStandard = "https://s3.{Region}.amazonaws.com/amazon-ssm-packages-{Region}/Packages/{PackageName}/{Platform}/{Arch}"

	// PackageUrlBeta is the s3 location for ad-hoc testing by package developers
	PackageUrlBeta = "https://s3.us-east-1.amazonaws.com/amazon-ssm-packages-beta/Packages/{PackageName}/{Platform}/{Arch}"

	// PackageUrlGamma is the s3 location for internal pre-production testing
	PackageUrlGamma = "https://s3.us-east-1.amazonaws.com/amazon-ssm-packages-us-east-1-gamma/Packages/{PackageName}/{Platform}/{Arch}"

	// PackageUrlBjs is the s3 location for BJS region where all packages live
	PackageUrlBjs = "https://s3.{Region}.amazonaws.com.cn/amazon-ssm-packages-{Region}/Packages/{PackageName}/{Platform}/{Arch}"

	// PackageNameSuffix represents (when concatenated with the correct package url) the s3 location of a specific version of a package
	PackageNameSuffix = "/{PackageVersion}/" + PackageNameFormat

	// PatternVersion represents the regular expression for validating version
	PatternVersion = "^(?:(\\d+)\\.)(?:(\\d+)\\.)(\\d+)$"
)

type configureUtil interface {
	GetLatestVersion(log log.T, name string) (latestVersion string, err error)
	GetS3Location(packageName string, version string) (s3Location string)
}

type configureUtilImp struct {
	packageUrl     string
	compressFormat string
}

func NewUtil(instanceContext *updateutil.InstanceContext, repository string) configureUtil {
	var packageUrl string
	if repository == "beta" {
		packageUrl = PackageUrlBeta
	} else if repository == "gamma" {
		packageUrl = PackageUrlGamma
	} else if instanceContext.Region == s3util.RegionBJS {
		packageUrl = PackageUrlBjs
	} else {
		packageUrl = PackageUrlStandard
	}
	packageUrl = strings.Replace(packageUrl, updateutil.RegionHolder, instanceContext.Region, -1)
	packageUrl = strings.Replace(packageUrl, updateutil.PlatformHolder, appconfig.PackagePlatform, -1)
	packageUrl = strings.Replace(packageUrl, updateutil.ArchHolder, instanceContext.Arch, -1)
	return &configureUtilImp{packageUrl: packageUrl, compressFormat: instanceContext.CompressFormat}
}

// getS3Location constructs the s3 url to locate the package for downloading
func (util *configureUtilImp) GetS3Location(packageName string, version string) (s3Location string) {
	s3Location = util.packageUrl
	s3Location += PackageNameSuffix

	s3Location = strings.Replace(s3Location, updateutil.PackageNameHolder, packageName, -1)
	s3Location = strings.Replace(s3Location, updateutil.PackageVersionHolder, version, -1)
	s3Location = strings.Replace(s3Location, updateutil.CompressedHolder, util.compressFormat, -1)
	return s3Location
}

// getS3Url returns the s3 location containing all versions of a package
func getS3Url(packageUrl string, packageName string) (s3Url *url.URL) {
	// s3 uri format based on agreed convention
	s3Location := packageUrl
	s3Location = strings.Replace(s3Location, updateutil.PackageNameHolder, packageName, -1)

	s3Url, _ = url.Parse(s3Location)
	return s3Url
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
func getLatestS3Version(log log.T, packageUrl string, name string) (latestVersion string, err error) {
	amazonS3URL := s3util.ParseAmazonS3URL(log, getS3Url(packageUrl, name))
	log.Debugf("looking up latest version of %v from %v", name, amazonS3URL.String())
	folders, err := networkdep.ListS3Folders(log, amazonS3URL)
	if err != nil {
		return
	}
	return getLatestVersion(folders[:], ""), nil
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
func (util *configureUtilImp) GetLatestVersion(log log.T, name string) (latestVersion string, err error) {
	latestVersion, err = getLatestS3Version(log, util.packageUrl, name)
	// handle case where we couldn't figure out which version to install but not because of an error in the S3 call
	if latestVersion == "" {
		return "", fmt.Errorf("no latest version found for package %v on platform %v", name, appconfig.PackagePlatform)
	}
	return latestVersion, err
}
