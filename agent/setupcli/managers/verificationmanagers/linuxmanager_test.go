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
	"bytes"
	"errors"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
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
	// Override sleepFunc to avoid real delays in tests
	sleepFunc = func(d time.Duration) {}
}

// Test function for Verification Manager - Success scenario
func (suite *VerificationManagerLinuxTestSuite) TestVerificationManager_Success() {
	artifactsPath := "temp2"
	signaturePath := "sig1"
	tempKeyRing := "keyring"
	gpgExtension := ".gpg"
	keyringFile := filepath.Join(artifactsPath, tempKeyRing+gpgExtension)

	mgrHelper := &mhMock.IManagerHelper{}
	ioWriteUtil = func(filename string, data []byte, perm fs.FileMode) error {
		return nil
	}
	amazonSSMAgentGPGKey := filepath.Join(artifactsPath, appconfig.DefaultAgentName+gpgExtension)
	fileExtension := ".deb"
	binaryPath := filepath.Join(artifactsPath, appconfig.DefaultAgentName+fileExtension)
	mgrHelper.On("IsCommandAvailable", "gpg").Return(true)
	mgrHelper.On("RunCommand", "gpg", "--no-tty", "--batch", "--no-default-keyring", "--keyring", keyringFile, "--import", amazonSSMAgentGPGKey).Return("status: accepted sample output", nil).Once()
	mgrHelper.On("RunCommand", "gpg", "--no-tty", "--batch", "--no-default-keyring", "--keyring", keyringFile, "--verify", signaturePath, binaryPath).Return("Good signature from \"SSM Agent <ssm-agent-signer@amazon.com>\"", nil).Once()

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
	gpgExtension := ".gpg"
	keyringFile := filepath.Join(artifactsPath, tempKeyRing+gpgExtension)

	mgrHelper := &mhMock.IManagerHelper{}
	ioWriteUtil = func(filename string, data []byte, perm fs.FileMode) error {
		return nil
	}
	amazonSSMAgentGPGKey := filepath.Join(artifactsPath, appconfig.DefaultAgentName+gpgExtension)
	fileExtension := ".deb"
	gpgErr := errors.New("exit status 1")
	binaryPath := filepath.Join(artifactsPath, appconfig.DefaultAgentName+fileExtension)
	mgrHelper.On("IsCommandAvailable", "gpg").Return(true)
	mgrHelper.On("RunCommand", "gpg", "--no-tty", "--batch", "--no-default-keyring", "--keyring", keyringFile, "--import", amazonSSMAgentGPGKey).Return("status: accepted sample output", nil).Once()
	mgrHelper.On("IsTimeoutError", gpgErr).Return(false)
	mgrHelper.On("RunCommand", "gpg", "--no-tty", "--batch", "--no-default-keyring", "--keyring", keyringFile, "--verify", signaturePath, binaryPath).Return("Bad signature from \"SSM Agent <ssm-agent-signer@amazon.com>\"", gpgErr).Once()

	pkgManagerRef := linuxManager{managerHelper: mgrHelper}
	err := pkgManagerRef.VerifySignature(suite.logMock, signaturePath, artifactsPath, fileExtension)

	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to verify signature using gpg with output")
	mgrHelper.AssertExpectations(suite.T())
}

func TestVerificationManagerLinuxTestSuite(t *testing.T) {
	suite.Run(t, new(VerificationManagerLinuxTestSuite))
}

func TestGpgvFallback_Success(t *testing.T) {
	artifactsPath := "artifacts"
	signaturePath := "sig/signature.sig"
	fileExtension := ".deb"
	binaryPath := filepath.Join(artifactsPath, appconfig.DefaultAgentName+fileExtension)
	dearmoredKeyPath := filepath.Join(artifactsPath, appconfig.DefaultAgentName+".gpg.bin")

	logMock := logmocks.NewMockLog()
	mgrHelper := &mhMock.IManagerHelper{}
	ioWriteUtil = func(filename string, data []byte, perm fs.FileMode) error {
		return nil
	}

	mgrHelper.On("IsCommandAvailable", "gpg").Return(false)
	mgrHelper.On("IsCommandAvailable", "gpgv").Return(true)
	mgrHelper.On("RunCommand", "gpgv", "--keyring", dearmoredKeyPath, signaturePath, binaryPath).
		Return("Good signature", nil)

	pkgManagerRef := linuxManager{managerHelper: mgrHelper}
	err := pkgManagerRef.VerifySignature(logMock, signaturePath, artifactsPath, fileExtension)

	assert.Nil(t, err)
	mgrHelper.AssertExpectations(t)
}

func TestSqvFallback_Success(t *testing.T) {
	artifactsPath := "artifacts"
	signaturePath := "sig/signature.sig"
	fileExtension := ".deb"
	binaryPath := filepath.Join(artifactsPath, appconfig.DefaultAgentName+fileExtension)
	sqvKeyPath := filepath.Join(artifactsPath, appconfig.DefaultAgentName+".asc")

	logMock := logmocks.NewMockLog()
	mgrHelper := &mhMock.IManagerHelper{}
	ioWriteUtil = func(filename string, data []byte, perm fs.FileMode) error {
		return nil
	}

	mgrHelper.On("IsCommandAvailable", "gpg").Return(false)
	mgrHelper.On("IsCommandAvailable", "gpgv").Return(false)
	mgrHelper.On("IsCommandAvailable", "sqv").Return(true)
	mgrHelper.On("RunCommand", "sqv", "--keyring", sqvKeyPath, signaturePath, binaryPath).
		Return("1C5B395CBE5E74E2F1CA21592784DBF388D19D46", nil)

	pkgManagerRef := linuxManager{managerHelper: mgrHelper}
	err := pkgManagerRef.VerifySignature(logMock, signaturePath, artifactsPath, fileExtension)

	assert.Nil(t, err)
	mgrHelper.AssertExpectations(t)
}

func TestSqvFallback_Failure(t *testing.T) {
	artifactsPath := "artifacts"
	signaturePath := "sig/signature.sig"
	fileExtension := ".deb"
	binaryPath := filepath.Join(artifactsPath, appconfig.DefaultAgentName+fileExtension)
	sqvKeyPath := filepath.Join(artifactsPath, appconfig.DefaultAgentName+".asc")

	logMock := logmocks.NewMockLog()
	mgrHelper := &mhMock.IManagerHelper{}
	ioWriteUtil = func(filename string, data []byte, perm fs.FileMode) error {
		return nil
	}

	sqvErr := errors.New("exit status 1")
	mgrHelper.On("IsCommandAvailable", "gpg").Return(false)
	mgrHelper.On("IsCommandAvailable", "gpgv").Return(false)
	mgrHelper.On("IsCommandAvailable", "sqv").Return(true)
	mgrHelper.On("IsTimeoutError", sqvErr).Return(false)
	mgrHelper.On("RunCommand", "sqv", "--keyring", sqvKeyPath, signaturePath, binaryPath).
		Return("signature verification failed", sqvErr)

	pkgManagerRef := linuxManager{managerHelper: mgrHelper}
	err := pkgManagerRef.VerifySignature(logMock, signaturePath, artifactsPath, fileExtension)

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "failed to verify signature using sqv")
	mgrHelper.AssertExpectations(t)
}

func TestNeitherGpgGpgvNorSqvAvailable_FallsBackToInProcess(t *testing.T) {
	// When none of gpg, gpgv, or sqv is present, VerifySignature routes to the
	// in-process verifier instead of returning an error.
	osOpen = os.Open
	ioReadFile = ioutil.ReadFile
	defer func() { inProcessKeyringSource = buildInProcessKeyring }()

	logMock := logmocks.NewMockLog()
	mgrHelper := &mhMock.IManagerHelper{}
	ioWriteUtil = func(filename string, data []byte, perm fs.FileMode) error {
		return nil
	}

	mgrHelper.On("IsCommandAvailable", "gpg").Return(false)
	mgrHelper.On("IsCommandAvailable", "gpgv").Return(false)
	mgrHelper.On("IsCommandAvailable", "sqv").Return(false)

	// Missing signature file proves the in-process path was taken (its error prefix).
	pkgManagerRef := linuxManager{managerHelper: mgrHelper}
	err := pkgManagerRef.VerifySignature(logMock, "nonexistent.sig", "artifacts", ".deb")

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "in-process verify")
	mgrHelper.AssertExpectations(t)
}

func TestBuildInProcessKeyring_ParsesEmbeddedKeys(t *testing.T) {
	keyring, err := buildInProcessKeyring()

	assert.Nil(t, err)
	assert.Equal(t, 3, len(keyring), "expected all three embedded signing keys to parse under the default crypto policy")
}

func TestVerifySignatureInProcess_Success(t *testing.T) {
	osOpen = os.Open
	ioReadFile = ioutil.ReadFile
	defer func() { inProcessKeyringSource = buildInProcessKeyring }()

	tempDir := t.TempDir()
	fileExtension := ".deb"
	binaryPath := filepath.Join(tempDir, appconfig.DefaultAgentName+fileExtension)
	signaturePath := filepath.Join(tempDir, appconfig.DefaultAgentName+fileExtension+".sig")

	payload := []byte("test agent binary contents")
	assert.Nil(t, os.WriteFile(binaryPath, payload, 0644))

	entity, err := openpgp.NewEntity("SSM Test", "", "ssm-test@amazon.com", nil)
	assert.Nil(t, err)

	var sigBuf bytes.Buffer
	assert.Nil(t, openpgp.DetachSign(&sigBuf, entity, bytes.NewReader(payload), nil))
	assert.Nil(t, os.WriteFile(signaturePath, sigBuf.Bytes(), 0644))

	inProcessKeyringSource = func() (openpgp.EntityList, error) {
		return openpgp.EntityList{entity}, nil
	}

	logMock := logmocks.NewMockLog()
	mgrHelper := &mhMock.IManagerHelper{}
	mgrHelper.On("IsCommandAvailable", "gpg").Return(false)
	mgrHelper.On("IsCommandAvailable", "gpgv").Return(false)
	mgrHelper.On("IsCommandAvailable", "sqv").Return(false)

	pkgManagerRef := linuxManager{managerHelper: mgrHelper}
	err = pkgManagerRef.VerifySignature(logMock, signaturePath, tempDir, fileExtension)

	assert.Nil(t, err)
	mgrHelper.AssertExpectations(t)
}

func TestVerifySignatureInProcess_BadSignature(t *testing.T) {
	osOpen = os.Open
	ioReadFile = ioutil.ReadFile
	defer func() { inProcessKeyringSource = buildInProcessKeyring }()

	tempDir := t.TempDir()
	fileExtension := ".deb"
	binaryPath := filepath.Join(tempDir, appconfig.DefaultAgentName+fileExtension)
	signaturePath := filepath.Join(tempDir, appconfig.DefaultAgentName+fileExtension+".sig")

	assert.Nil(t, os.WriteFile(binaryPath, []byte("real agent contents"), 0644))

	entity, err := openpgp.NewEntity("SSM Test", "", "ssm-test@amazon.com", nil)
	assert.Nil(t, err)

	// Sign different bytes than the binary so verification must fail.
	var sigBuf bytes.Buffer
	assert.Nil(t, openpgp.DetachSign(&sigBuf, entity, bytes.NewReader([]byte("tampered contents")), nil))
	assert.Nil(t, os.WriteFile(signaturePath, sigBuf.Bytes(), 0644))

	inProcessKeyringSource = func() (openpgp.EntityList, error) {
		return openpgp.EntityList{entity}, nil
	}

	logMock := logmocks.NewMockLog()
	mgrHelper := &mhMock.IManagerHelper{}
	mgrHelper.On("IsCommandAvailable", "gpg").Return(false)
	mgrHelper.On("IsCommandAvailable", "gpgv").Return(false)
	mgrHelper.On("IsCommandAvailable", "sqv").Return(false)

	pkgManagerRef := linuxManager{managerHelper: mgrHelper}
	err = pkgManagerRef.VerifySignature(logMock, signaturePath, tempDir, fileExtension)

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "in-process verify: failed to verify signature")
	mgrHelper.AssertExpectations(t)
}

func TestImportTimeout(t *testing.T) {
	sleepFunc = func(d time.Duration) {}
	artifactsPath := "artifacts"
	gpgExtension := ".gpg"
	amazonSSMAgentGPGKey := filepath.Join(artifactsPath, appconfig.DefaultAgentName+gpgExtension)
	keyringFile := filepath.Join(artifactsPath, "keyring"+gpgExtension)

	logMock := logmocks.NewMockLog()
	mgrHelper := &mhMock.IManagerHelper{}
	ioWriteUtil = func(filename string, data []byte, perm fs.FileMode) error {
		return nil
	}

	timeoutErr := errors.New("command timed out")
	mgrHelper.On("IsCommandAvailable", "gpg").Return(true)
	mgrHelper.On("RunCommand", "gpg", "--no-tty", "--batch", "--no-default-keyring", "--keyring", keyringFile, "--import", amazonSSMAgentGPGKey).
		Return("", timeoutErr)
	mgrHelper.On("IsTimeoutError", timeoutErr).Return(true)

	pkgManagerRef := linuxManager{managerHelper: mgrHelper}
	err := pkgManagerRef.VerifySignature(logMock, "sig", artifactsPath, ".deb")

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "gpg command timed out")
}

func TestVerifyTimeout(t *testing.T) {
	sleepFunc = func(d time.Duration) {}
	artifactsPath := "artifacts"
	signaturePath := "sig"
	fileExtension := ".deb"
	gpgExtension := ".gpg"
	amazonSSMAgentGPGKey := filepath.Join(artifactsPath, appconfig.DefaultAgentName+gpgExtension)
	keyringFile := filepath.Join(artifactsPath, "keyring"+gpgExtension)
	binaryPath := filepath.Join(artifactsPath, appconfig.DefaultAgentName+fileExtension)

	logMock := logmocks.NewMockLog()
	mgrHelper := &mhMock.IManagerHelper{}
	ioWriteUtil = func(filename string, data []byte, perm fs.FileMode) error {
		return nil
	}

	timeoutErr := errors.New("command timed out")
	mgrHelper.On("IsCommandAvailable", "gpg").Return(true)
	mgrHelper.On("RunCommand", "gpg", "--no-tty", "--batch", "--no-default-keyring", "--keyring", keyringFile, "--import", amazonSSMAgentGPGKey).
		Return("ok", nil)
	mgrHelper.On("RunCommand", "gpg", "--no-tty", "--batch", "--no-default-keyring", "--keyring", keyringFile, "--verify", signaturePath, binaryPath).
		Return("", timeoutErr)
	mgrHelper.On("IsTimeoutError", timeoutErr).Return(true)

	pkgManagerRef := linuxManager{managerHelper: mgrHelper}
	err := pkgManagerRef.VerifySignature(logMock, signaturePath, artifactsPath, fileExtension)

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "gpg verify: command timed out")
}

func TestFileCreationFailure(t *testing.T) {
	logMock := logmocks.NewMockLog()
	mgrHelper := &mhMock.IManagerHelper{}
	ioWriteUtil = func(filename string, data []byte, perm fs.FileMode) error {
		return errors.New("permission denied")
	}

	mgrHelper.On("IsCommandAvailable", "gpg").Return(true)

	pkgManagerRef := linuxManager{managerHelper: mgrHelper}
	err := pkgManagerRef.VerifySignature(logMock, "sig", "artifacts", ".deb")

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "failed to create amazon-ssm-agent.gpg file")
}

func TestDearmorPublicKeys_BothKeysDecoded(t *testing.T) {
	dearmored, err := GetLinuxPublicKeyDearmored()

	assert.Nil(t, err)
	// Both keys combined should be >2000 bytes (key1 ~1178 + key2 ~1690)
	assert.Greater(t, len(dearmored), 2000, "expected both keys to be decoded, got %d bytes", len(dearmored))
}
