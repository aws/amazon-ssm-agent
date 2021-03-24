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

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	identityMocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	context "github.com/aws/amazon-ssm-agent/core/app/context/mocks"
	lock "github.com/nightlyone/lockfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type SelfUpdateTestSuite struct {
	suite.Suite
	contextMock     *context.ICoreAgentContext
	identityMock    *identityMocks.IAgentIdentity
	logMock         *log.Mock
	appconfigMock   *appconfig.SsmagentConfig
	selfUpdater     *SelfUpdate
	platformNameMap map[string]string
}

// SetupTest will initialized the object for each test case before test function execution
func (suite *SelfUpdateTestSuite) SetupTest() {
	suite.logMock = log.NewMockLog()
	suite.appconfigMock = &appconfig.SsmagentConfig{}
	suite.contextMock = &context.ICoreAgentContext{}
	suite.identityMock = &identityMocks.IAgentIdentity{}
	suite.contextMock.On("With", "[SelfUpdate]").Return(suite.contextMock)
	suite.contextMock.On("Log").Return(suite.logMock)
	suite.contextMock.On("Identity").Return(suite.identityMock)
	suite.contextMock.On("AppConfig").Return(suite.appconfigMock)
	suite.selfUpdater = NewSelfUpdater(suite.contextMock)
	suite.platformNameMap = map[string]string{
		PlatformAmazonLinux: PlatformLinux,
		PlatformRedHat:      PlatformLinux,
		PlatformCentOS:      PlatformLinux,
		PlatformSuseOS:      PlatformLinux,
		PlatformRaspbian:    PlatformUbuntu,
	}
	updateInitialize = func(instanceId string) (err error) {
		return nil
	}
	updateDownloadResource = func(region string) (err error) {
		return nil
	}
	updateExecuteSelfUpdate = func(log log.T, region string) (pid int, err error) {
		return os.Getppid(), nil
	}
}

func (suite *SelfUpdateTestSuite) TestLoadScheduleDaysWithinLimit() {

	assert.EqualValues(suite.T(), suite.selfUpdater.updateFrequencyHrs, 0,
		"Update frequency should 0 before initialization")

	config := suite.contextMock.AppConfig()

	config.Agent.SelfUpdate = true
	config.Agent.SelfUpdateScheduleDay = 4
	suite.selfUpdater.loadScheduledFrequency(*config)

	assert.Equal(suite.T(), 4, suite.selfUpdater.updateFrequencyHrs/24)
}

func (suite *SelfUpdateTestSuite) TestGetPlatformNameFailed() {
	platformNameGetter = func(log log.T) (name string, err error) {
		return "", fmt.Errorf("Failed to return platform name")
	}

	platformName, err := suite.selfUpdater.getPlatformName(suite.logMock)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), "", platformName)
}

func (suite *SelfUpdateTestSuite) TestGetPlatformNameForWindows() {
	platformNameGetter = func(log log.T) (name string, err error) {
		return "windows", nil
	}
	nanoChecker = func(log log.T) (bool, error) {
		return true, nil
	}
	platformName, err := suite.selfUpdater.getPlatformName(suite.logMock)
	assert.NotNil(suite.T(), err)

	nanoChecker = func(log log.T) (bool, error) {
		return false, nil
	}
	platformName, err = suite.selfUpdater.getPlatformName(suite.logMock)
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), PlatformWindows, platformName)
}

func (suite *SelfUpdateTestSuite) TestGetPlatformNameSucceed() {
	for platform, installer := range suite.platformNameMap {
		platformNameGetter = func(log log.T) (name string, err error) {
			return platform, nil
		}

		platformName, err := suite.selfUpdater.getPlatformName(suite.logMock)
		assert.Nil(suite.T(), err)
		assert.Equal(suite.T(), installer, platformName)
	}
}

func (suite *SelfUpdateTestSuite) TestGetDownloadManifestURL() {
	var manifestUrl, chinaRegion, commonRegion string
	chinaRegion = "cn-north-1"
	chinaManifestUrl := "https://s3.cn-north-1.amazonaws.com.cn/amazon-ssm-cn-north-1/ssm-agent-manifest.json"
	suite.identityMock.On("GetDefaultEndpoint", "s3").Return("s3.cn-north-1.amazonaws.com.cn").Once()

	manifestUrl = suite.selfUpdater.generateDownloadManifestURL(suite.logMock, chinaRegion)
	assert.Equal(suite.T(), manifestUrl, chinaManifestUrl)

	commonRegion = "us-east-1"
	commonManifestUrl := "https://s3.us-east-1.amazonaws.com/amazon-ssm-us-east-1/ssm-agent-manifest.json"
	suite.identityMock.On("GetDefaultEndpoint", "s3").Return("s3.us-east-1.amazonaws.com").Once()

	manifestUrl = suite.selfUpdater.generateDownloadManifestURL(suite.logMock, commonRegion)
	assert.Equal(suite.T(), manifestUrl, commonManifestUrl)
}

func (suite *SelfUpdateTestSuite) TestGetDownloadUpdaterChina() {
	var updaterUrl, chinaRegion, commonRegion string
	fileName := "amazon-ssm-agent-updater-linux-amd64.tar.gz"

	chinaRegion = "cn-north-1"
	chinaUpdaterUrl := "https://s3.cn-north-1.amazonaws.com.cn/amazon-ssm-cn-north-1/amazon-ssm-agent-updater/latest/amazon-ssm-agent-updater-linux-amd64.tar.gz"
	suite.identityMock.On("GetDefaultEndpoint", "s3").Return("s3.cn-north-1.amazonaws.com.cn").Once()

	updaterUrl = suite.selfUpdater.generateDownloadUpdaterURL(suite.logMock, chinaRegion, fileName)
	assert.Equal(suite.T(), chinaUpdaterUrl, updaterUrl)

	commonRegion = "eu-west-1"
	commonUpdaterUrl := "https://s3.eu-west-1.amazonaws.com/amazon-ssm-eu-west-1/amazon-ssm-agent-updater/latest/amazon-ssm-agent-updater-linux-amd64.tar.gz"
	suite.identityMock.On("GetDefaultEndpoint", "s3").Return("s3.eu-west-1.amazonaws.com").Once()

	updaterUrl = suite.selfUpdater.generateDownloadUpdaterURL(suite.logMock, commonRegion, fileName)
	assert.Equal(suite.T(), commonUpdaterUrl, updaterUrl)
}

func (suite *SelfUpdateTestSuite) TestFileName() {
	platformNameGetter = func(log log.T) (name string, err error) {
		return PlatformRedHat, nil
	}

	fileName, err := suite.selfUpdater.getUpdaterFileName(suite.logMock, "amd64", updateconstants.CompressFormat)
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), "amazon-ssm-agent-updater-linux-amd64.tar.gz", fileName)
}

func (suite *SelfUpdateTestSuite) TestLockFileBasic() {
	workingDir, _ := os.Getwd()
	lockfilePath := filepath.Join(workingDir, "lockDir")
	lockFileName = filepath.Join(lockfilePath, "test.lock")
	err := os.MkdirAll(lockfilePath, 0777)
	defer func() {
		os.RemoveAll(lockfilePath)
	}()

	mockSelfUpdateObj := SelfUpdate{context: suite.contextMock}

	err = mockSelfUpdateObj.updateFromS3()
	assert.Nil(suite.T(), err)
}

func (suite *SelfUpdateTestSuite) TestLockFileWithError() {
	suite.identityMock.On("InstanceID").Return("i-123", nil).Once()
	suite.identityMock.On("Region").Return("us-west-2", nil).Once()
	workingDir, _ := os.Getwd()
	lockfilePath := filepath.Join(workingDir, "lockDir")
	lockFileName = filepath.Join(lockfilePath, "test.lock")
	err := os.MkdirAll(lockfilePath, 0777)
	defer func() {
		os.RemoveAll(lockfilePath)
	}()

	mockSelfUpdateObj := SelfUpdate{context: suite.contextMock}

	updateExecuteSelfUpdate = func(log log.T, region string) (pid int, err error) {
		return -1, fmt.Errorf("test")
	}

	err = mockSelfUpdateObj.updateFromS3()
	assert.NotNil(suite.T(), err)

	updateExecuteSelfUpdate = func(log log.T, region string) (pid int, err error) {
		return os.Getppid(), nil
	}
	err = mockSelfUpdateObj.updateFromS3()
	assert.Nil(suite.T(), err)
}

func (suite *SelfUpdateTestSuite) TestLockWithMultipleUpdates() {
	suite.identityMock.On("InstanceID").Return("i-123", nil).Once()
	suite.identityMock.On("Region").Return("us-west-2", nil).Once()

	workingDir, _ := os.Getwd()
	lockfilePath := filepath.Join(workingDir, "lockDir")
	lockFileName = filepath.Join(lockfilePath, "test.lock")
	err := os.MkdirAll(lockfilePath, 0777)
	defer func() {
		os.RemoveAll(lockfilePath)
	}()

	mockSelfUpdateObj := SelfUpdate{context: suite.contextMock}

	err = mockSelfUpdateObj.updateFromS3()
	assert.Nil(suite.T(), err)

	err = mockSelfUpdateObj.updateFromS3()
	assert.NotNil(suite.T(), err)
	if err != nil {
		assert.NotNil(suite.T(), err.Error(), lock.ErrBusy.Error())
	}
}

//Execute the test suite
func TestSelfUpdateTestSuite(t *testing.T) {
	suite.Run(t, new(SelfUpdateTestSuite))
}
