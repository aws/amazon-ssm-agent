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
	DownloadManifest(manifest updatemanifest.T, manifestUrl string) (err error)

	DownloadUpdater(
		manifest updatemanifest.T,
		updaterPackageName string,
		downloadPath string,
	) (version string, err error)
}

type updateS3UtilImpl struct {
	context context.T
}
