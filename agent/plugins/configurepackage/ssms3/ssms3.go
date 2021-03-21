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

package ssms3

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/packageservice"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/trace"
	"github.com/aws/amazon-ssm-agent/agent/s3util"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
)

const (
	// EndpointHolder represent placeholder for S3 endpoint
	EndpointHolder = "{Endpoint}"

	// RegionHolder represents placeholder for region
	RegionHolder = "{Region}"

	// PlatformHolder represents placeholder for platform
	PlatformHolder = "{Platform}"

	// ArchHolder represents placeholder for Arch
	ArchHolder = "{Arch}"

	// PackageNameFormat represents the package name format based
	PackageNameFormat = "{PackageName}.zip"

	// PackageURLStandard represents the s3 folder where all versions of a package live
	// the url to a specific package has a format like https://s3.us-east-1.amazonaws.com/amazon-ssm-packages-us-east-1/Packages/Test/windows/amd64/1.0.0/Test.zip
	PackageURLStandard = "https://{Endpoint}/amazon-ssm-packages-{Region}/BirdwatcherPackages/{PackageName}/{Platform}/{Arch}"

	// PackageURLBeta is the s3 location for ad-hoc testing by package developers
	PackageURLBeta = "https://s3.amazonaws.com/amazon-ssm-packages-beta/BirdwatcherPackages/{PackageName}/{Platform}/{Arch}"

	// PackageURLGamma is the s3 location for internal pre-production testing
	PackageURLGamma = "https://s3.amazonaws.com/amazon-ssm-packages-us-east-1-gamma/BirdwatcherPackages/{PackageName}/{Platform}/{Arch}"

	// PackageNameSuffix represents (when concatenated with the correct package url) the s3 location of a specific version of a package
	PackageNameSuffix = "/{PackageVersion}/" + PackageNameFormat

	// PatternVersion represents the regular expression for validating version
	PatternVersion = "^(?:(\\d+)\\.)(?:(\\d+)\\.)(\\d+)$"

	// ActiveServiceURL is the s3 object whose presence indicates the SSMS3 service implementation should be used
	ActiveServiceURL      = "https://{Endpoint}/amazon-ssm-packages-{Region}/active-birdwatcher-fallback"
	ActiveServiceURLBeta  = "https://s3.amazonaws.com/amazon-ssm-packages-beta/active-birdwatcher-fallback"
	ActiveServiceURLGamma = "https://s3.amazonaws.com/amazon-ssm-packages-us-east-1-gamma/active-birdwatcher-fallback"
)

type PackageService struct {
	context    context.T
	packageURL string
}

// UseSSMS3Service checks for existence of the active service indicator file.  If the file has been removed, it indicates that the new package service should be used
func UseSSMS3Service(context context.T, tracer trace.Tracer, repository string, region string) bool {
	var checkURL string
	if repository == "beta" {
		checkURL = ActiveServiceURLBeta
	} else if repository == "gamma" {
		checkURL = ActiveServiceURLGamma
	} else {
		checkURL = ActiveServiceURL
	}
	checkURL = strings.Replace(checkURL, EndpointHolder, s3util.GetS3Endpoint(context, region), -1)
	checkURL = strings.Replace(checkURL, RegionHolder, region, -1)

	parsedURL, _ := url.Parse(checkURL)
	logger := tracer.CurrentTrace().Logger
	return networkdep.CanGetS3Object(context, s3util.ParseAmazonS3URL(logger, parsedURL))
}

func New(context context.T, repository string, region string) *PackageService {
	var packageURL string
	if repository == "beta" {
		packageURL = PackageURLBeta
	} else if repository == "gamma" {
		packageURL = PackageURLGamma
	} else {
		packageURL = PackageURLStandard
	}
	packageURL = strings.Replace(packageURL, EndpointHolder, s3util.GetS3Endpoint(context, region), -1)
	packageURL = strings.Replace(packageURL, RegionHolder, region, -1)
	packageURL = strings.Replace(packageURL, PlatformHolder, appconfig.PackagePlatform, -1)
	packageURL = strings.Replace(packageURL, ArchHolder, runtime.GOARCH, -1)
	return &PackageService{
		context:    context,
		packageURL: packageURL}
}

func (ds *PackageService) PackageServiceName() string {
	return packageservice.PackageServiceName_ssms3
}

func (ds *PackageService) GetPackageArnAndVersion(packageName string, packageVersion string) (name string, version string) {
	version = packageVersion
	if packageservice.IsLatest(packageVersion) {
		version = packageservice.Latest
	}
	return packageName, version
}

// DownloadManifest looks up the latest version of a given package for this platform/arch in S3 or manifest at source location
func (ds *PackageService) DownloadManifest(tracer trace.Tracer, packageName string, version string) (string, string, bool, error) {
	//TODO: Redesign the DownloadManifest in the packageService to return the manifest, once ssms3 gets deleted
	var targetVersion string
	var err error

	//return the isSameAsCache true for ssms3 case, to not have to reinstall the package every time this is called
	isSameAsCache := true

	if !packageservice.IsLatest(version) {
		targetVersion = version
	} else {
		targetVersion, err = getLatestS3Version(ds.context, tracer, ds.packageURL, packageName)
		tracer.CurrentTrace().AppendInfof("latest version: %v", targetVersion)
		if err != nil {
			return packageName, "", isSameAsCache, err
		}
		// handle case where we couldn't figure out which version to install but not because of an error in the S3 call
		if targetVersion == "" {
			return packageName, "", isSameAsCache, fmt.Errorf("no latest version found for package %v on platform %v", packageName, appconfig.PackagePlatform)
		}
	}

	return packageName, targetVersion, isSameAsCache, err
}

func (ds *PackageService) DownloadArtifact(tracer trace.Tracer, packageName string, version string) (string, error) {
	s3Location := getS3Location(packageName, version, ds.packageURL)
	return downloadPackageFromS3(ds.context, tracer, s3Location)
}

func (*PackageService) ReportResult(tracer trace.Tracer, result packageservice.PackageResult) error {
	// NOP
	return nil
}

// utils

// downloadPackageFromS3 downloads and uncompresses the installation package from s3 bucket
func downloadPackageFromS3(context context.T, tracer trace.Tracer, packageS3Source string) (string, error) {
	// TODO: deduplicate with birdwatcher download
	downloadInput := artifact.DownloadInput{
		SourceURL: packageS3Source,
	}

	downloadOutput, downloadErr := networkdep.Download(context, downloadInput)
	if downloadErr != nil || downloadOutput.LocalFilePath == "" {
		errMessage := fmt.Sprintf("failed to download installation package reliably, %v", downloadInput.SourceURL)
		if downloadErr != nil {
			errMessage = fmt.Sprintf("%v, %v", errMessage, downloadErr.Error())
		}
		// TODO: cleanup download artefacts

		// return download error
		return "", errors.New(errMessage)
	}

	return downloadOutput.LocalFilePath, nil
}

// getS3Location constructs the s3 url to locate the package for downloading
func getS3Location(packageName string, version string, url string) string {
	s3Location := url + PackageNameSuffix

	s3Location = strings.Replace(s3Location, updateconstants.PackageNameHolder, packageName, -1)
	s3Location = strings.Replace(s3Location, updateconstants.PackageVersionHolder, version, -1)
	return s3Location
}

// getS3Url returns the s3 location containing all versions of a package
func getS3Url(packageURL string, packageName string) *url.URL {
	// s3 uri format based on agreed convention
	s3Location := packageURL
	s3Location = strings.Replace(s3Location, updateconstants.PackageNameHolder, packageName, -1)

	s3Url, _ := url.Parse(s3Location)
	return s3Url
}

// getLatestVersion returns the latest version given a list of version strings (that match PatternVersion)
func getLatestVersion(versions []string, except string) string {
	var latestVersion string
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
func getLatestS3Version(context context.T, tracer trace.Tracer, packageURL string, name string) (string, error) {
	logger := tracer.CurrentTrace().Logger

	amazonS3URL := s3util.ParseAmazonS3URL(logger, getS3Url(packageURL, name))

	versiontrace := tracer.BeginSection(fmt.Sprintf("looking up latest version of %v from %v", name, amazonS3URL.String()))

	folders, err := networkdep.ListS3Folders(context, amazonS3URL)
	if err != nil {
		versiontrace.WithError(err).End()
		return "", err
	}

	latestVersion := getLatestVersion(folders[:], "")
	versiontrace.AppendInfof("latest version: %s", latestVersion).End()
	return latestVersion, nil
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
