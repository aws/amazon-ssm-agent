// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

//+build windows

// Package updateec2config implements the UpdateEC2Config plugin.
package updateec2config

const (
	//minimum version for EC2 config service
	minimumVersion = "0"

	// PipelineTestVersion represents fake version for pipeline tests
	PipelineTestVersion = "9999.0.0.0"

	//EC2 config agent constants
	EC2UpdaterPackageName = "aws-ec2windows-ec2configupdater"
	EC2ConfigAgentName    = "aws-ec2windows-ec2config"
	EC2UpdaterFileName    = "EC2ConfigUpdater.zip"
	EC2SetupFileName      = "EC2ConfigSetup.zip"
	Updater               = "EC2ConfigUpdater"

	//redefined here because manifest file has a spelling error which will need to be continued
	PackageVersionHolder = "{PacakgeVersion}"

	//update command arguments
	SetupInstallCmd   = " --setup-installation"
	SourceVersionCmd  = "-current-version"
	SourceLocationCmd = "-current-source"
	SourceHashCmd     = "-current-hash"
	TargetVersionCmd  = "-target-version"
	TargetLocationCmd = "-target-source"
	TargetHashCmd     = "-target-hash"
	MessageIDCmd      = "-message-id"
	HistoryCmd        = "-history"
	InstanceID        = "-instance-id"
	DocumentIDCmd     = "-document-id"
	RegionIDCmd       = "-instance-region"
	UserAgentCmd      = "-user-agent"
	MdsEndpointCmd    = "-mds-endpoint"
	UpdateHealthCmd   = " --health-update"
	UpdateCmd         = " --update"

	//constant num histories
	numHistories = "10"

	//HTTP format for ssmagent
	HTTPFormat = "https://aws-ssm-{Region}.s3.amazonaws.com"

	//S3 format for updater
	S3Format = "https://s3.amazonaws.com/aws-ssm-{Region}"

	// CommonManifestURL is the URL for the manifest file in regular regions
	CommonManifestURL = "https://s3.{Region}.amazonaws.com/aws-ssm-{Region}/manifest.json"

	// ChinaManifestURL is the URL for the manifest in regions in China
	ChinaManifestURL = "https://s3.{Region}.amazonaws.com.cn/aws-ssm-{Region}/manifest.json"
)

// update context constant strings
const (
	notStarted  = "NotStarted"
	initialized = "Initialized"
	staged      = "Staged"
	installed   = "Installed"
	rollback    = "Rollback"
	rolledBack  = "Rolledback"
	completed   = "Completed"
)

// update state constant strings
const (
	inProgress = "InProgress"
	succeeded  = "Succeeded"
	failed     = "Failed"
)
