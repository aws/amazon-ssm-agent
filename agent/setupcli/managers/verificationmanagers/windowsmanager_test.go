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

// Package verificationmanagers is used to verify the agent packages
package verificationmanagers

import (
	"path/filepath"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common"
	mhMock "github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// Define VerificationManagerWindows TestSuite struct
type VerificationManagerWindowsTestSuite struct {
	suite.Suite
	logMock *logmocks.Mock
}

// Initialize the VerificationManagerLinux test suite struct
func (suite *VerificationManagerWindowsTestSuite) SetupTest() {
	logMock := logmocks.NewMockLog()
	suite.logMock = logMock
}

// Test function for Verification Manager - Success scenario
func (suite *VerificationManagerWindowsTestSuite) TestVerificationManager_Success() {
	artifactsPath := "temp1"
	signaturePath := "sign1"

	isNanoPlatform = func(log log.T) (bool, error) {
		return false, nil
	}

	setupPath := filepath.Join(artifactsPath, common.AmazonWindowsSetupFile)

	mgrHelper := &mhMock.IManagerHelper{}
	subjectName := "CN=Amazon.com Services LLC, OU=AWS Systems Manager, O=Amazon.com Services LLC, L=Seattle, S=Washington, C=US, SERIALNUMBER=3482342, OID.2.5.4.15=Private Organization, OID.1.3.6.1.4.1.311.60.2.1.2=Delaware, OID.1.3.6.1.4.1.311.60.2.1.3=US"
	mgrHelper.On("RunCommand", mock.Anything, "(Get-AuthenticodeSignature", "-FilePath", "\""+setupPath+"\").SignerCertificate.SubjectName.Name").Return(subjectName, nil).Once()
	mgrHelper.On("RunCommand", mock.Anything, "(Get-AuthenticodeSignature", "-FilePath", "\""+setupPath+"\").Status").Return("Valid", nil).Once()

	pkgManagerRef := windowsManager{managerHelper: mgrHelper}
	err := pkgManagerRef.VerifySignature(suite.logMock, signaturePath, artifactsPath, "")

	assert.Nil(suite.T(), err)
	mgrHelper.AssertExpectations(suite.T())
}

// Test function for Verification Manager - Failure scenario
func (suite *VerificationManagerWindowsTestSuite) TestVerificationManager_Failure() {
	artifactsPath := "temp1"
	signaturePath := "sign1"

	isNanoPlatform = func(log log.T) (bool, error) {
		return false, nil
	}

	setupPath := filepath.Join(artifactsPath, common.AmazonWindowsSetupFile)

	mgrHelper := &mhMock.IManagerHelper{}

	subjectName := "CN=Amazon.com Services LLC, OU=AWS Systems Manager, O=Amazon.com Services LLC, L=Seattle, S=Washington, C=US, SERIALNUMBER=3482342, OID.2.5.4.15=Private Organization, OID.1.3.6.1.4.1.311.60.2.1.2=Delaware, OID.1.3.6.1.4.1.311.60.2.1.3=US"
	mgrHelper.On("RunCommand", mock.Anything, "(Get-AuthenticodeSignature", "-FilePath", "\""+setupPath+"\").SignerCertificate.SubjectName.Name").Return(subjectName, nil).Once()
	mgrHelper.On("RunCommand", mock.Anything, "(Get-AuthenticodeSignature", "-FilePath", "\""+setupPath+"\").Status").Return("Inalid", nil).Once()

	pkgManagerRef := windowsManager{managerHelper: mgrHelper}
	err := pkgManagerRef.VerifySignature(suite.logMock, signaturePath, artifactsPath, "")

	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "invalid signing:")
	mgrHelper.AssertExpectations(suite.T())
}

// Test function for Verification Manager - Failure scenario
func (suite *VerificationManagerWindowsTestSuite) TestVerificationManager_Failure_WrongCommonName() {
	artifactsPath := "temp1"
	signaturePath := "sign1"

	isNanoPlatform = func(log log.T) (bool, error) {
		return false, nil
	}

	setupPath := filepath.Join(artifactsPath, common.AmazonWindowsSetupFile)

	mgrHelper := &mhMock.IManagerHelper{}

	subjectName := "CN=sdfdsfsdfsd, OU=AWS Systems Manager, O=Amazon.com Services LLC, L=Seattle, S=Washington, C=US, SERIALNUMBER=3482342, OID.2.5.4.15=Private Organization, OID.1.3.6.1.4.1.311.60.2.1.2=Delaware, OID.1.3.6.1.4.1.311.60.2.1.3=US"
	mgrHelper.On("RunCommand", mock.Anything, "(Get-AuthenticodeSignature", "-FilePath", "\""+setupPath+"\").SignerCertificate.SubjectName.Name").Return(subjectName, nil).Once()

	pkgManagerRef := windowsManager{managerHelper: mgrHelper}
	err := pkgManagerRef.VerifySignature(suite.logMock, signaturePath, artifactsPath, "")

	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "cerificate identity is not valid")
	mgrHelper.AssertExpectations(suite.T())
}

func TestVerificationManagerWindowsTestSuite(t *testing.T) {
	suite.Run(t, new(VerificationManagerWindowsTestSuite))
}
