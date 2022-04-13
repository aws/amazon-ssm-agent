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

// Package updates3util implements the logic for s3 update download
package updates3util

import (
	"io/ioutil"
	"os"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updatemanifest"
	"github.com/aws/amazon-ssm-agent/agent/version"
)

var fileDownload = artifact.Download
var fileDecompress = fileutil.Uncompress

var createTempDir = ioutil.TempDir
var removeDir = os.RemoveAll

var currentVersion = version.Version

type T interface {
	DownloadManifest(manifest updatemanifest.T, manifestUrl string) *UpdateErrorStruct

	DownloadUpdater(
		manifest updatemanifest.T,
		updaterPackageName string,
		downloadPath string,
	) (version string, err error)
}

type updateS3UtilImpl struct {
	context context.T
}

// DownloadManifestErrorCodes represents error codes while downloading manifest
type DownloadManifestErrorCodes string

var (
	// ResolveManifestURLErrorCode represents error code for manifest URL resolution error
	ResolveManifestURLErrorCode DownloadManifestErrorCodes = "d1"
	// TmpDownloadDirCreationErrorCode represents error code for directory download error
	TmpDownloadDirCreationErrorCode DownloadManifestErrorCodes = "d2"
	// NetworkFileDownloadErrorCode represents error code for manifest file download error
	NetworkFileDownloadErrorCode DownloadManifestErrorCodes = "d3"
	// HashMismatchErrorCode represents error code for file hash mismatch failure
	HashMismatchErrorCode DownloadManifestErrorCodes = "d4"
	// LocalFilePathEmptyErrorCode represents error code for empty local download file path
	LocalFilePathEmptyErrorCode DownloadManifestErrorCodes = "d5"
	// LoadManifestErrorCode represents error code for load manifest error
	LoadManifestErrorCode DownloadManifestErrorCodes = "d6"
)

// UpdateErrorStruct represents Update error struct
type UpdateErrorStruct struct {
	// Error denotes update error
	Error error
	// Error denotes update error code
	ErrorCode string
}
