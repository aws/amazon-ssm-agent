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

//go:build freebsd || linux || netbsd || openbsd
// +build freebsd linux netbsd openbsd

// Package verificationmanagers is used to verify the agent packages
package verificationmanagers

import (
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	mhMock "github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// Define VerificationManagerLinux TestSuite struct
type VerificationManagerLinuxTestSuite struct {
	suite.Suite
	logMock *logmocks.Mock
}

// Initialize the VerificationManagerLinux test suite struct
func (suite *VerificationManagerLinuxTestSuite) SetupTest() {
	logMock := logmocks.NewMockLog()
	suite.logMock = logMock
}

// Test function for Verification Manager - Success scenario
func (suite *VerificationManagerLinuxTestSuite) TestVerificationManager_Success() {
	artifactsPath := "temp2"
	signaturePath := "sig1"
	tempKeyRing := "keyring"
	keyringPath := filepath.Join(artifactsPath, tempKeyRing)
	gpgExtension := ".gpg"

	mgrHelper := &mhMock.IManagerHelper{}
	fileUtilMakeDirs = func(destinationDir string) (err error) {
		return nil
	}
	ioWriteUtil = func(filename string, data []byte, perm fs.FileMode) error {
		return nil
	}
	amazonSSMAgentGPGKey := filepath.Join(artifactsPath, appconfig.DefaultAgentName+gpgExtension)
	fileExtension := ".deb"
	binaryPath := filepath.Join(artifactsPath, appconfig.DefaultAgentName+fileExtension)
	mgrHelper.On("IsCommandAvailable", "gpg").Return(true)
	mgrHelper.On("RunCommand", "gpg", "--no-default-keyring", "--keyring", keyringPath, "--import", amazonSSMAgentGPGKey).Return("status: accepted sample output", nil).Once()
	mgrHelper.On("RunCommand", "gpg", "--no-default-keyring", "--keyring", keyringPath, "--verify", signaturePath, binaryPath).Return("Good signature from \"SSM Agent <ssm-agent-signer@amazon.com>\"", nil).Once()

	pkgManagerRef := linuxManager{managerHelper: mgrHelper}
	err := pkgManagerRef.VerifySignature(suite.logMock, signaturePath, artifactsPath, fileExtension)

	assert.Nil(suite.T(), err)
	mgrHelper.AssertExpectations(suite.T())
}

// Test function for Verification Manager - Failure scenario
func (suite *VerificationManagerLinuxTestSuite) TestVerificationManager_Failure() {
	artifactsPath := "temp2"
	signaturePath := "sig1"
	tempKeyRing := "keyring"
	keyringPath := filepath.Join(artifactsPath, tempKeyRing)
	gpgExtension := ".gpg"

	mgrHelper := &mhMock.IManagerHelper{}
	fileUtilMakeDirs = func(destinationDir string) (err error) {
		return nil
	}
	ioWriteUtil = func(filename string, data []byte, perm fs.FileMode) error {
		return nil
	}
	amazonSSMAgentGPGKey := filepath.Join(artifactsPath, appconfig.DefaultAgentName+gpgExtension)
	fileExtension := ".deb"
	binaryPath := filepath.Join(artifactsPath, appconfig.DefaultAgentName+fileExtension)
	mgrHelper.On("IsCommandAvailable", "gpg").Return(true)
	mgrHelper.On("RunCommand", "gpg", "--no-default-keyring", "--keyring", keyringPath, "--import", amazonSSMAgentGPGKey).Return("status: accepted sample output", nil).Once()
	mgrHelper.On("RunCommand", "gpg", "--no-default-keyring", "--keyring", keyringPath, "--verify", signaturePath, binaryPath).Return("Bad signature from \"SSM Agent <ssm-agent-signer@amazon.com>\"", nil).Once()

	pkgManagerRef := linuxManager{managerHelper: mgrHelper}
	err := pkgManagerRef.VerifySignature(suite.logMock, signaturePath, artifactsPath, fileExtension)

	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "signature verification failed")
	mgrHelper.AssertExpectations(suite.T())
}

func TestVerificationManagerLinuxTestSuite(t *testing.T) {
	suite.Run(t, new(VerificationManagerLinuxTestSuite))
}
