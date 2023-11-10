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

// Package downloadmanager helps us with file download related functions in ssm-setup-cli
package downloadmanager

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateinfo"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updatemanifest"
	updatemanifestmocks "github.com/aws/amazon-ssm-agent/agent/updateutil/updatemanifest/mocks"

	"github.com/aws/amazon-ssm-agent/agent/log"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	updateinfomocks "github.com/aws/amazon-ssm-agent/agent/updateutil/updateinfo/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// Define ConfigManager TestSuite struct
type DownloadManagerTestSuite struct {
	suite.Suite
	logMock *logmocks.Mock
}

// Initialize the ConfigManagerTestSuite test suite struct
func (suite *DownloadManagerTestSuite) SetupTest() {
	hasLowerKernelVersionFunc = func() bool {
		return false
	}
	logMock := logmocks.NewMockLog()
	suite.logMock = logMock
}

func (suite *DownloadManagerTestSuite) TestDownloadManager_GetStableVersion_Success() {
	path := "path1"
	utilHttpDownload = func(log log.T, fileURL string, destinationPath string) (string, error) {
		return destinationPath, nil
	}
	updateManifestNew = func(context context.T, info updateinfo.T, region string) updatemanifest.T {
		updateManifestMock := &updatemanifestmocks.T{}
		updateManifestMock.On("LoadManifest", path).Return(nil).Once()
		return updateManifestMock
	}
	downloadMgr := New(suite.logMock, "us-east-1", "https://s3.amazonaws.com/"+updateconstants.ManifestFile, nil, "path1", true)
	versionUrl := ""
	expectedVersionNumber := "3.2.1377.0"
	expectedStableVersionURL := "https://s3.amazonaws.com/stable/VERSION"
	fileUtilityReadContent = func(stableVersionUrl string, client *http.Client) ([]byte, error) {
		versionUrl = stableVersionUrl
		return []byte(expectedVersionNumber), nil
	}
	versionNum, err := downloadMgr.GetStableVersion()
	assert.Equal(suite.T(), expectedVersionNumber, versionNum, "mismatched version number")
	assert.Nil(suite.T(), err, "unexpected error")
	assert.Equal(suite.T(), expectedStableVersionURL, versionUrl, "mismatched version URL")
	downloadMgr = New(suite.logMock, "us-east-1", "https://s3.amazonaws.com/"+updateconstants.ManifestFile+" ", nil, path, true)

	versionUrl = ""
	expectedVersionNumber = "3.2.1377.0"
	expectedStableVersionURL = "https://s3.amazonaws.com/stable/VERSION"
	fileUtilityReadContent = func(stableVersionUrl string, client *http.Client) ([]byte, error) {
		versionUrl = stableVersionUrl
		return []byte(expectedVersionNumber), nil
	}
	versionNum, err = downloadMgr.GetStableVersion()
	assert.Equal(suite.T(), expectedVersionNumber, versionNum, "mismatched version number")
	assert.Nil(suite.T(), err, "unexpected error")
	assert.Equal(suite.T(), expectedStableVersionURL, versionUrl, "mismatched version URL")

	downloadMgr = New(suite.logMock, "us-east-1", "", nil, "path1", true)
	versionUrl = ""
	expectedVersionNumber = "3.2.1377.0"
	expectedStableVersionURL = "https://s3.us-east-1.amazonaws.com/amazon-ssm-us-east-1/stable/VERSION"
	fileUtilityReadContent = func(stableVersionUrl string, client *http.Client) ([]byte, error) {
		versionUrl = stableVersionUrl
		return []byte(expectedVersionNumber), nil
	}
	versionNum, err = downloadMgr.GetStableVersion()
	assert.Equal(suite.T(), expectedVersionNumber, versionNum, "mismatched version number")
	assert.Nil(suite.T(), err, "unexpected error")
	assert.Equal(suite.T(), expectedStableVersionURL, versionUrl, "mismatched version URL")
}

func (suite *DownloadManagerTestSuite) TestDownloadManager_GetStableVersion_Failure() {
	path := "path1"
	utilHttpDownload = func(log log.T, fileURL string, destinationPath string) (string, error) {
		return destinationPath, nil
	}
	updateManifestNew = func(context context.T, info updateinfo.T, region string) updatemanifest.T {
		updateManifestMock := &updatemanifestmocks.T{}
		updateManifestMock.On("LoadManifest", path).Return(nil).Once()
		return updateManifestMock
	}
	downloadMgr := New(suite.logMock, "us-east-1", "https://s3.amazonaws.com/"+updateconstants.ManifestFile, nil, "path1", true)
	versionUrl := ""
	expectedStableVersionURL := "https://s3.amazonaws.com/stable/VERSION"
	fileUtilityReadContent = func(stableVersionUrl string, client *http.Client) ([]byte, error) {
		versionUrl = stableVersionUrl
		return []byte("3.d.32.2" + " "), nil
	}
	versionNum, err := downloadMgr.GetStableVersion()
	assert.Equal(suite.T(), "", versionNum, "mismatched version number")
	assert.NotNil(suite.T(), err, "should throw error")
	assert.Equal(suite.T(), expectedStableVersionURL, versionUrl, "mismatched version URL")

	fileUtilityReadContent = func(stableVersionUrl string, client *http.Client) ([]byte, error) {
		return nil, nil
	}
	_, err = downloadMgr.GetStableVersion()
	assert.NotNil(suite.T(), err, "should throw error")
}

func (suite *DownloadManagerTestSuite) TestDownloadManager_GetLatestVersion_Success() {
	path := "path1"
	expectedVersionNumber := "3.2.1377.0"
	utilHttpDownload = func(log log.T, fileURL string, destinationPath string) (string, error) {
		return destinationPath, nil
	}

	updateManifestNew = func(context context.T, info updateinfo.T, region string) updatemanifest.T {
		updateManifestMock := &updatemanifestmocks.T{}
		updateManifestMock.On("LoadManifest", path).Return(nil).Once()
		updateManifestMock.On("GetLatestActiveVersion", appconfig.DefaultAgentName).Return(expectedVersionNumber, nil).Once()
		return updateManifestMock
	}
	downloadMgr := New(suite.logMock, "us-east-1", "https://s3.amazonaws.com/"+updateconstants.ManifestFile, nil, path, true)
	versionNum, err := downloadMgr.GetLatestVersion()
	assert.Equal(suite.T(), expectedVersionNumber, versionNum, "mismatched version number")
	assert.Nil(suite.T(), err, "unexpected error")
}

func (suite *DownloadManagerTestSuite) TestDownloadManager_GetLatestVersion_Failure() {
	path := "path1"
	utilHttpDownload = func(log log.T, fileURL string, destinationPath string) (string, error) {
		return destinationPath, nil
	}
	updateManifestNew = func(context context.T, info updateinfo.T, region string) updatemanifest.T {
		updateManifestMock := &updatemanifestmocks.T{}
		updateManifestMock.On("LoadManifest", path).Return(nil).Once()
		updateManifestMock.On("GetLatestActiveVersion", appconfig.DefaultAgentName).Return("", fmt.Errorf("err1")).Once()
		return updateManifestMock
	}
	downloadMgr := New(suite.logMock, "us-east-1", "https://s3.amazonaws.com/"+updateconstants.ManifestFile, nil, "path1", true)
	versionNum, err := downloadMgr.GetLatestVersion()
	assert.Equal(suite.T(), "", versionNum, "mismatched version number")
	assert.NotNil(suite.T(), err, "should throw error")

	fileUtilityReadContent = func(stableVersionUrl string, client *http.Client) ([]byte, error) {
		return nil, nil
	}
	_, err = downloadMgr.GetStableVersion()
	assert.NotNil(suite.T(), err, "should throw error")
}

func (suite *DownloadManagerTestSuite) TestDownloadManager_DownloadLatestSSMSetupCLI_Success() {
	info := &updateinfomocks.T{}
	info.On("GeneratePlatformBasedFolderName").Return("linux_amd64")
	path := "path1"
	utilHttpDownload = func(log log.T, fileURL string, destinationPath string) (string, error) {
		return destinationPath, nil
	}
	updateManifestNew = func(context context.T, info updateinfo.T, region string) updatemanifest.T {
		updateManifestMock := &updatemanifestmocks.T{}
		updateManifestMock.On("LoadManifest", path).Return(nil).Once()
		return updateManifestMock
	}
	downloadMgr := New(suite.logMock, "us-east-1", "", info, "path1", true)
	actualSSMSetupCLIURL := ""
	utilHttpDownload = func(log log.T, fileURL string, destinationPath string) (string, error) {
		actualSSMSetupCLIURL = fileURL
		return "temp2", nil
	}
	checkSum := "23232"
	computeAgentChecksumFunc = func(agentFilePath string) (hash string, err error) {
		return checkSum, nil
	}
	expectedLatestSSMSetupCLIURL := "https://s3.us-east-1.amazonaws.com/amazon-ssm-us-east-1/latest/linux_amd64/ssm-setup-cli"
	err := downloadMgr.DownloadLatestSSMSetupCLI("temp1", checkSum)

	assert.Nil(suite.T(), err, "should not throw error")
	assert.Contains(suite.T(), actualSSMSetupCLIURL, expectedLatestSSMSetupCLIURL, "mismatched version URL")
}

func (suite *DownloadManagerTestSuite) TestDownloadManager_DownloadLatestSSMSetupCLI_HttpDownloadFailure() {
	info := &updateinfomocks.T{}
	info.On("GeneratePlatformBasedFolderName").Return("linux_amd64")
	path := "path1"
	utilHttpDownload = func(log log.T, fileURL string, destinationPath string) (string, error) {
		return destinationPath, nil
	}
	updateManifestNew = func(context context.T, info updateinfo.T, region string) updatemanifest.T {
		updateManifestMock := &updatemanifestmocks.T{}
		updateManifestMock.On("LoadManifest", path).Return(nil).Once()
		return updateManifestMock
	}
	downloadMgr := New(suite.logMock, "us-east-1", "", info, "path1", true)
	actualSSMSetupCLIURL := ""
	utilHttpDownload = func(log log.T, fileURL string, destinationPath string) (string, error) {
		actualSSMSetupCLIURL = fileURL
		return "temp2", fmt.Errorf("test")
	}
	checkSum := "23232"
	notVisited := true
	computeAgentChecksumFunc = func(agentFilePath string) (hash string, err error) {
		notVisited = false
		return checkSum, nil
	}
	expectedLatestSSMSetupCLIURL := "https://s3.us-east-1.amazonaws.com/amazon-ssm-us-east-1/latest/linux_amd64/ssm-setup-cli"
	err := downloadMgr.DownloadLatestSSMSetupCLI("temp1", checkSum)

	assert.Contains(suite.T(), err.Error(), "error while downloading SSM Setup CLI", "should throw error")
	assert.Contains(suite.T(), actualSSMSetupCLIURL, expectedLatestSSMSetupCLIURL, "mismatched version URL")
	assert.True(suite.T(), notVisited)
}

func (suite *DownloadManagerTestSuite) TestDownloadManager_DownloadLatestSSMSetupCLI_CheckSumFailure() {
	info := &updateinfomocks.T{}
	info.On("GeneratePlatformBasedFolderName").Return("linux_amd64")
	path := "path1"
	utilHttpDownload = func(log log.T, fileURL string, destinationPath string) (string, error) {
		return destinationPath, nil
	}
	updateManifestNew = func(context context.T, info updateinfo.T, region string) updatemanifest.T {
		updateManifestMock := &updatemanifestmocks.T{}
		updateManifestMock.On("LoadManifest", path).Return(nil).Once()
		return updateManifestMock
	}
	downloadMgr := New(suite.logMock, "us-east-1", "", info, "path1", true)
	actualSSMSetupCLIURL := ""
	utilHttpDownload = func(log log.T, fileURL string, destinationPath string) (string, error) {
		actualSSMSetupCLIURL = fileURL
		return "temp2", nil
	}
	checkSum := "23232"
	computeAgentChecksumFunc = func(agentFilePath string) (hash string, err error) {
		return "sdsds", nil
	}
	expectedLatestSSMSetupCLIURL := "https://s3.us-east-1.amazonaws.com/amazon-ssm-us-east-1/latest/linux_amd64/ssm-setup-cli"
	err := downloadMgr.DownloadLatestSSMSetupCLI("temp1", checkSum)

	assert.Contains(suite.T(), err.Error(), "checksum validation for ssm-setup-cli fail", "should throw error")
	assert.Contains(suite.T(), actualSSMSetupCLIURL, expectedLatestSSMSetupCLIURL, "mismatched version URL")
}

func (suite *DownloadManagerTestSuite) TestDownloadManager_DownloadArtifacts_Success() {
	info := &updateinfomocks.T{}
	info.On("GeneratePlatformBasedFolderName").Return("linux_amd64")
	path := "path1"
	tempPath := "temp2"
	version := "3.2.3.5"
	checkSum := "1234"
	utilHttpDownload = func(log log.T, fileURL string, destinationPath string) (string, error) {
		return destinationPath, nil
	}
	expectedLatestSSMSetupCLIURL := "https://s3.us-east-1.amazonaws.com/amazon-ssm-us-east-1/ssm-agent-manifest.json"

	updateManifestNew = func(context context.T, info updateinfo.T, region string) updatemanifest.T {
		updateManifestMock := &updatemanifestmocks.T{}
		updateManifestMock.On("LoadManifest", path).Return(nil).Once()
		updateManifestMock.On("GetDownloadURLAndHash", appconfig.DefaultAgentName, version).Return(expectedLatestSSMSetupCLIURL, checkSum, nil).Once()
		return updateManifestMock
	}
	downloadMgr := New(suite.logMock, "us-east-1", "", info, path, true)
	actualSSMSetupCLIURL := ""

	utilHttpDownload = func(log log.T, fileURL string, destinationPath string) (string, error) {
		if actualSSMSetupCLIURL == "" {
			actualSSMSetupCLIURL = fileURL
		}
		return tempPath, nil
	}

	computeAgentChecksumFunc = func(agentFilePath string) (hash string, err error) {
		return checkSum, nil
	}
	fileUtilUnCompress = func(log log.T, src, dest string) error {
		return nil
	}
	err := downloadMgr.DownloadArtifacts(version, "manifestURL1", "temp1")
	assert.Nil(suite.T(), err, "should not throw error")
	assert.Equal(suite.T(), expectedLatestSSMSetupCLIURL, actualSSMSetupCLIURL, "mismatched version URL")
}

func TestDownloadManagerTestSuite(t *testing.T) {
	suite.Run(t, new(DownloadManagerTestSuite))
}
