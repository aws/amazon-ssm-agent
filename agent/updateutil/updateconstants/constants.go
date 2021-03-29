// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package updateconstants contains constants related to update
package updateconstants

const (
	// UpdaterPackageNamePrefix represents the name of Updater Package
	UpdaterPackageNamePrefix = "-updater"

	// HashType represents the default hash type
	HashType = "sha256"

	// Updater represents Updater name
	Updater = "updater"

	// Directory containing older versions of agent during update
	UpdateAmazonSSMAgentDir = "amazon-ssm-agent/"

	// UpdateContextFileName represents Update context json file
	UpdateContextFileName = "updatecontext.json"

	// UpdatePluginResultFileName represents Update plugin result file name
	UpdatePluginResultFileName = "updatepluginresult.json"

	// DefaultOutputFolder represents default location for storing output files
	DefaultOutputFolder = "awsupdateSsmAgent"

	// DefaultStandOut represents the default file name for update stand output
	DefaultStandOut = "stdout"

	// DefaultStandErr represents the default file name for update stand error
	DefaultStandErr = "stderr"

	// RegionHolder represents Place holder for Region
	RegionHolder = "{Region}"

	// PackageNameHolder represents Place holder for package name
	PackageNameHolder = "{PackageName}"

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

	// PlatformLinux represents linux
	PlatformLinux = "linux"

	// PlatformAmazonLinux represents amazon linux
	PlatformAmazonLinux = "amazon"

	// PlatformRedHat represents RedHat
	PlatformRedHat = "red hat"

	// PlatformOracleLinux represents oracle linux
	PlatformOracleLinux = "oracle"

	// PlatformUbuntu represents Ubuntu
	PlatformUbuntu = "ubuntu"

	// PlatformUbuntuSnap represents Snap
	PlatformUbuntuSnap = "snap"

	//PlatformDarwin represents darwin
	PlatformDarwin = "darwin"

	// PlatformCentOS represents CentOS
	PlatformCentOS = "centos"

	// PlatformSuse represents SLES(SUSe)
	PlatformSuseOS = "sles"

	// PlatformRaspbian represents Raspbian
	PlatformRaspbian = "raspbian"

	// PlatformDebian represents Debian
	PlatformDebian = "debian"

	// PlatformWindows represents windows
	PlatformWindows = "windows"

	//PlatformWindowsNano represents windows nano
	PlatformWindowsNano = "windows-nano"

	//PlatformMacOsX represents mac os
	PlatformMacOsX = "mac os x"

	// DefaultUpdateExecutionTimeoutInSeconds represents default timeout time for execution update related scripts in seconds
	DefaultUpdateExecutionTimeoutInSeconds = 150

	// PipelineTestVersion represents fake version for pipeline tests
	PipelineTestVersion = "255.0.0.0"

	SSMAgentWorkerMinVersion = "3.0.0.0"

	MinimumVersion = "0"

	// Lock file expiry minutes
	UpdateLockFileMinutes = int64(60)

	SnapServiceFile = "/etc/systemd/system/snap.amazon-ssm-agent.amazon-ssm-agent.service"

	// ManifestFile is the manifest file name
	ManifestFile = "ssm-agent-manifest.json"

	//ManifestPath is path of manifest in the s3 bucket
	ManifestPath = "/amazon-ssm-{Region}/" + ManifestFile

	// CommonManifestURL is the Manifest URL for regular regions
	CommonManifestURL = "https://s3.{Region}.amazonaws.com" + ManifestPath

	// ChinaManifestURL is the manifest URL for regions in China
	ChinaManifestURL = "https://s3.{Region}.amazonaws.com.cn" + ManifestPath

	// DarwinBinaryPath is the default path of the amazon-ssm-agent binary on darwin
	DarwinBinaryPath = "/opt/aws/ssm/bin/amazon-ssm-agent"
)

// error status codes returned from the update scripts
type UpdateScriptExitCode int

const (
	// exit code represents exit code when there is no service manager
	// TODO: Move error to a update precondition
	ExitCodeUnsupportedPlatform UpdateScriptExitCode = 124

	// exit code represents exit code from agent update install script
	ExitCodeUpdateUsingPkgMgr UpdateScriptExitCode = 125
)

// SUb status values
const (
	// installRollback represents rollback code flow occurring during installation
	InstallRollback = "InstallRollback_"

	// verificationRollback represents rollback code flow occurring during verification
	VerificationRollback = "VerificationRollback_"

	// downgrade represents that the respective error code was logged during agent downgrade
	Downgrade = "downgrade_"
)

//ErrorCode is types of Error Codes
type ErrorCode string

const (
	// ErrorInvalidSourceVersion represents Source version is not supported
	ErrorInvalidSourceVersion ErrorCode = "ErrorInvalidSourceVersion"

	// ErrorInvalidTargetVersion represents Target version is not supported
	ErrorInvalidTargetVersion ErrorCode = "ErrorInvalidTargetVersion"

	// ErrorSourcePkgDownload represents source version not able to download
	ErrorSourcePkgDownload ErrorCode = "ErrorSourcePkgDownload"

	// ErrorCreateInstanceContext represents the error code while loading the initial context
	ErrorCreateInstanceContext ErrorCode = "ErrorCreateInstanceContext"

	// ErrorTargetPkgDownload represents target version not able to download
	ErrorTargetPkgDownload ErrorCode = "ErrorTargetPkgDownload"

	// ErrorUnexpected represents Unexpected Error from panic
	ErrorUnexpectedThroughPanic ErrorCode = "ErrorUnexpectedThroughPanic"

	// ErrorManifestURLParse represents manifest url parse error
	ErrorManifestURLParse ErrorCode = "ErrorManifestURLParse"

	// ErrorDownloadManifest represents download manifest error
	ErrorDownloadManifest ErrorCode = "ErrorDownloadManifest"

	// ErrorCreateUpdateFolder represents error when creating the download directory
	ErrorCreateUpdateFolder ErrorCode = "ErrorCreateUpdateFolder"

	// ErrorDownloadUpdater represents error when download and unzip the updater
	ErrorDownloadUpdater ErrorCode = "ErrorDownloadUpdater"

	// ErrorExecuteUpdater represents error when execute the updater
	ErrorExecuteUpdater ErrorCode = "ErrorExecuteUpdater"

	// ErrorUnsupportedVersion represents version less than minimum supported version by OS
	ErrorUnsupportedVersion ErrorCode = "ErrorUnsupportedVersion"

	// ErrorUpdateFailRollbackSuccess represents rollback succeeded but update process failed
	ErrorUpdateFailRollbackSuccess ErrorCode = "ErrorUpdateFailRollbackSuccess"

	// ErrorAttemptToDowngrade represents An update is attempting to downgrade Ec2Config to a lower version
	ErrorAttemptToDowngrade ErrorCode = "ErrorAttempToDowngrade"

	// ErrorFailedPrecondition represents An non fulfilled precondition
	ErrorFailedPrecondition ErrorCode = "ErrorFailedPrecondition"

	// ErrorInitializationFailed represents An update is failed to initialize
	ErrorInitializationFailed ErrorCode = "ErrorInitializationFailed"

	// ErrorInvalidPackage represents Installation package file is invalid
	ErrorInvalidPackage ErrorCode = "ErrorInvalidPackage"

	// ErrorPackageNotAccessible represents Installation package file is not accessible
	ErrorPackageNotAccessible ErrorCode = "ErrorPackageNotAccessible"

	// ErrorInvalidCertificate represents Installation package file doesn't contain valid certificate
	ErrorInvalidCertificate ErrorCode = "ErrorInvalidCertificate"

	// ErrorVersionNotFoundInManifest represents version is not found in the manifest
	ErrorVersionNotFoundInManifest ErrorCode = "ErrorVersionNotFoundInManifest"

	// ErrorGetLatestActiveVersionManifest represents failure to get latest active version from manifest
	ErrorGetLatestActiveVersionManifest ErrorCode = "ErrorGetLatestActiveVersionManifest"

	// ErrorInvalidManifest represents Invalid manifest file
	ErrorInvalidManifest ErrorCode = "ErrorInvalidManifest"

	// ErrorInvalidManifestLocation represents Invalid manifest file location
	ErrorInvalidManifestLocation ErrorCode = "ErrorInvalidManifestLocation"

	// ErrorUninstallFailed represents Uninstall failed
	ErrorUninstallFailed ErrorCode = "ErrorUninstallFailed"

	// ErrorUnsupportedServiceManager represents unsupported service manager
	ErrorUnsupportedServiceManager ErrorCode = "ErrorUnsupportedServiceManager"

	// ErrorInstallFailed represents Install failed
	ErrorInstallFailed ErrorCode = "ErrorInstallFailed"

	// ErrorCannotStartService represents Cannot start Ec2Config service
	ErrorCannotStartService ErrorCode = "ErrorCannotStartService"

	// ErrorCannotStopService represents Cannot stop Ec2Config service
	ErrorCannotStopService ErrorCode = "ErrorCannotStopService"

	// ErrorTimeout represents Installation time-out
	ErrorTimeout ErrorCode = "ErrorTimeout"

	// ErrorVersionCompare represents version compare error
	ErrorVersionCompare ErrorCode = "ErrorVersionCompare"

	// ErrorUnexpected represents Unexpected Error
	ErrorUnexpected ErrorCode = "ErrorUnexpected"

	// ErrorUpdaterLockBusy represents message when updater lock is acquired by someone else
	ErrorUpdaterLockBusy ErrorCode = "ErrorUpdaterLockBusy"

	// ErrorEnvironmentIssue represents Unexpected Error
	ErrorEnvironmentIssue ErrorCode = "ErrorEnvironmentIssue"

	// ErrorLoadingAgentVersion represents failed for loading agent version
	ErrorLoadingAgentVersion ErrorCode = "ErrorLoadingAgentVersion"

	SelfUpdatePrefix = "SelfUpdate"

	// we have same below fields in processor package without underscore
	UpdateFailed    = "UpdateFailed_"
	UpdateSucceeded = "UpdateSucceeded_"
)

type TargetVersionResolver int

// target version resolver options
const (
	TargetVersionCustomerDefined = iota
	TargetVersionLatest
	TargetVersionSelfUpdate
)

// NonAlarmingErrors contains error codes which are not important.
var NonAlarmingErrors = map[ErrorCode]struct{}{
	ErrorUnsupportedServiceManager: {},
	ErrorAttemptToDowngrade:        {},
	ErrorFailedPrecondition:        {},
}

type SelfUpdateState string

const (
	Stage SelfUpdateState = "Stage"
)

const (
	// WarnInactiveVersion represents the warning message when inactive version is used for update
	WarnInactiveVersion string = "InactiveAgentVersion"

	// WarnUpdaterLockFail represents warning message that the lock could not be acquired because of system issues
	WarnUpdaterLockFail string = "WarnUpdaterLockFail"
)

const (
	// installer script for snap
	SnapInstaller = "snap-install.sh"
	// uninstaller script for snap
	SnapUnInstaller = "snap-uninstall.sh"
)

// MinimumDiskSpaceForUpdate represents 100 Mb in bytes
const MinimumDiskSpaceForUpdate int64 = 104857600
