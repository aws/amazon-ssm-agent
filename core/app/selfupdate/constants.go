// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package selfupdate provides an interface to force update with Message Gateway Service and S3

package selfupdate

const (
	name = "SelfUpdate"

	//TODO: Implement the Jitter logic for selfupdate
	JitterRatio = 1

	// MinimumDiskSpaceForUpdate represents 100 Mb in bytes
	MinimumDiskSpaceForUpdate int64 = 104857600

	// DefaultSelfUpdateFolder represent the orchestration sub folder for self update
	DefaultSelfUpdateFolder = "self-update-agent"

	// DefaultOutputFolder represents default location for storing output files
	DefaultOutputFolder = "awsupdateSsmAgent"

	ForceUpdatePullIntervalMinutes = 60 * 24 // 1 day = 1440 minutes

	// PackageVersionHolder represents Place holder for package version
	PackageVersionHolder = "{PackageVersion}"

	// FileNameHolder represents Place holder for file name
	FileNameHolder = "{FileName}"

	// PlatformHolder represents Place holder for platform
	PlatformHolder = "{Platform}"

	// ArchHolder represents Place holder for Arch
	ArchHolder = "{Arch}"

	// CompressedHolder represents Place holder for compress format
	CompressedHolder = "{Compressed}"

	// RegionHolder represents Place holder for region
	RegionHolder = "{Region}"

	// PlatformLinux represents linux
	PlatformLinux = "linux"

	// PlatformAmazonLinux represents amazon linux
	PlatformAmazonLinux = "amazon"

	// PlatformRedHat represents RedHat
	PlatformRedHat = "red hat"

	// PlatformUbuntu represents Ubuntu
	PlatformUbuntu = "ubuntu"

	//PlatformDarwin represents darwin
	PlatformDarwin = "darwin"

	//PlatformMacOsX represents mac os
	PlatformMacOsX = "mac os x"

	// PlatformOracleLinux represents oracle linux
	PlatformOracleLinux = "oracle"

	// PlatformDebian represents Debian
	PlatformDebian = "debian"

	// PlatformUbuntuSnap represents Snap
	PlatformUbuntuSnap = "snap"

	// PlatformCentOS represents CentOS
	PlatformCentOS = "centos"

	// PlatformSuse represents SLES(SUSe)
	PlatformSuseOS = "sles"

	// PlatformSuse represents Raspbian
	PlatformRaspbian = "raspbian"

	// PlatformWindows represents windows
	PlatformWindows = "windows"

	//PlatformWindowsNano represents windows nano
	PlatformWindowsNano = "windows-nano"

	// Default value for package name
	PackageName = "amazon-ssm-agent-updater"

	// Always download the updater from latest folder
	PackageVersion = "latest"

	// Updater represents Updater name
	Updater = "updater"

	// DefaultStandOut represents the default file name for update stand output
	DefaultStandOut = "stdout"

	// DefaultStandErr represents the default file name for update stand error
	DefaultStandErr = "stderr"

	// Default value for package name for ssm agent
	DefaultSSMAgentName = "amazon-ssm-agent"

	// url format to download updater for ssm agent
	UrlPath = "/amazon-ssm-{Region}/amazon-ssm-agent-updater/latest/{FileName}"

	// CommonUrlPath is the url format for download package in common region
	CommonUrlPath = "https://s3.{Region}.amazonaws.com" + UrlPath

	//ManifestPath is path of manifest in the s3 bucket
	ManifestPath = "/amazon-ssm-{Region}/ssm-agent-manifest.json"

	// CommonManifestURL is the Manifest URL for regular regions
	CommonManifestURL = "https://s3.{Region}.amazonaws.com" + ManifestPath

	// cn- is a prefix for China region
	ChinaRegionPrefix = "cn-"
)
