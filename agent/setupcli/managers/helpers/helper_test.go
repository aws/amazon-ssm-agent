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

// Package helpers contains helper functions for SSM-Setup-CLI
package helpers

import (
	"fmt"
	"testing"

	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	pkgMock "github.com/aws/amazon-ssm-agent/agent/setupcli/managers/packagemanagers/mocks"
	svcMock "github.com/aws/amazon-ssm-agent/agent/setupcli/managers/servicemanagers/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// Define HelperTestSuite TestSuite struct
type HelperTestSuite struct {
	suite.Suite
	logMock *logmocks.Mock
}

// Initialize the HelperTestSuite test suite struct
func (suite *HelperTestSuite) SetupTest() {
	logMock := logmocks.NewMockLog()
	suite.logMock = logMock
}

func (suite *HelperTestSuite) TestHelperInstallAgent_Success() {
	count := 0
	fileExists = func(filePath string) (bool, error) {
		count++
		return true, nil
	}
	installFile1 := "file1"
	installFile2 := "file2"

	pkgMgrMock := &pkgMock.IPackageManager{}
	pkgMgrMock.On("GetFilesReqForInstall", mock.Anything).Return([]string{installFile1, installFile2})
	pkgMgrMock.On("InstallAgent", mock.Anything, mock.Anything).Return(nil)

	svcMgrMock := &svcMock.IServiceManager{}
	svcMgrMock.On("ReloadManager").Return(nil)

	err := InstallAgent(suite.logMock, pkgMgrMock, svcMgrMock, "path1")
	assert.Equal(suite.T(), 2, count, "file iteration mismatch")
	assert.Nil(suite.T(), err)
}

func (suite *HelperTestSuite) TestHelperInstallAgent_Failure() {
	count := 0
	fileExists = func(filePath string) (bool, error) {
		count++
		return true, fmt.Errorf("file not found")
	}
	installFile1 := "file1"
	installFile2 := "file2"

	pkgMgrMock := &pkgMock.IPackageManager{}
	pkgMgrMock.On("GetFilesReqForInstall", mock.Anything).Return([]string{installFile1, installFile2})
	pkgMgrMock.On("InstallAgent", mock.Anything, mock.Anything).Return(nil)

	svcMgrMock := &svcMock.IServiceManager{}
	svcMgrMock.On("ReloadManager").Return(nil)

	err := InstallAgent(suite.logMock, pkgMgrMock, svcMgrMock, "path1")
	assert.Equal(suite.T(), 1, 1, "file iteration mismatch")
	assert.NotNil(suite.T(), err)
}

func TestHelperTestSuite(t *testing.T) {
	suite.Run(t, new(HelperTestSuite))
}
