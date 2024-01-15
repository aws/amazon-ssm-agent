// Copyright 2023 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

//go:build windows
// +build windows

// Package downloadmanager helps us with file download related functions in ssm-setup-cli
package downloadmanager

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateinfo"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updatemanifest"
	updatemanifestmocks "github.com/aws/amazon-ssm-agent/agent/updateutil/updatemanifest/mocks"
	"github.com/stretchr/testify/assert"
)

func (suite *DownloadManagerTestSuite) TestDownloadManager_DownloadSignatureFile_Success() {
	path := "path1"
	utilHttpDownload = func(log log.T, fileURL string, destinationPath string) (string, error) {
		return destinationPath, nil
	}
	updateManifestNew = func(context context.T, info updateinfo.T, region string) updatemanifest.T {
		updateManifestMock := &updatemanifestmocks.T{}
		updateManifestMock.On("LoadManifest", path).Return(nil).Once()
		return updateManifestMock
	}
	downloadMgr := New(suite.logMock, "us-east-1", "https://s3.amazonaws.com/"+updateconstants.ManifestFile, nil, path, true)
	path, err := downloadMgr.DownloadSignatureFile("", "", "")

	assert.Nil(suite.T(), err, "unexpected error")
	assert.Equal(suite.T(), "", path, "mismatched path")
}
