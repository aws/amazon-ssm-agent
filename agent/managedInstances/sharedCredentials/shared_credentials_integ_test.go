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

//go:build integration
// +build integration

// package sharedCredentials tests
package sharedCredentials

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"

	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/stretchr/testify/assert"
	"gopkg.in/ini.v1"
)

const (
	accessKey       = "DummyAccessKey"
	accessSecretKey = "DummyAccessSecretKey"
	token           = "DummyToken"
	profile         = "DummyProfile"
	testFilePath    = "example.ini"
)

func setupTest(credPath string) {
	// check if file exists, if not create it
	if !fileutil.Exists(credPath) {
		fileutil.WriteAllText(credPath, "")
	}
}

func exampleCredFilePath() string {
	pwd, _ := os.Getwd()
	credFilePath := filepath.Join(pwd, testFilePath)
	return credFilePath
}

func TestSharedCredentialsStore(t *testing.T) {
	// Test setup
	os.Clearenv()
	credFilePath := exampleCredFilePath()
	setupTest(credFilePath)

	// Test
	os.Setenv("AWS_SHARED_CREDENTIALS_FILE", credFilePath)

	shareFilePath, _ := GetSharedCredsFilePath("")
	err1 := Store(log.NewMockLog(), accessKey, accessSecretKey, token, shareFilePath, profile, false)
	assert.Nil(t, err1, "Expect no error saving profile")

	config, err2 := ini.Load(credFilePath)
	assert.Nil(t, err2, "Expect no error loading file")

	iniProfile := config.Section(profile)
	os.Setenv("AWS_SHARED_CREDENTIALS_FILE", "")

	assert.Equal(t, accessKey, iniProfile.Key(awsAccessKeyID).Value(), "Expect access key ID to match")
	assert.Equal(t, accessSecretKey, iniProfile.Key(awsSecretAccessKey).Value(), "Expect secret access key to match")
	assert.Equal(t, token, iniProfile.Key(awsSessionToken).Value(), "Expect session token to match")
}

func TestSharedCredentialsStoreDefaultProfile(t *testing.T) {
	// Test setup
	os.Clearenv()
	credFilePath := exampleCredFilePath()
	setupTest(credFilePath)

	// Test
	os.Setenv("AWS_SHARED_CREDENTIALS_FILE", credFilePath)

	shareFilePath, _ := GetSharedCredsFilePath("")
	err1 := Store(log.NewMockLog(), accessKey, accessSecretKey, token, shareFilePath, "", false)
	assert.Nil(t, err1, "Expect no error saving profile")

	config, err2 := ini.Load(credFilePath)
	os.Setenv("AWS_SHARED_CREDENTIALS_FILE", "")
	assert.Nil(t, err2, "Expect no error loading file")

	iniProfile := config.Section(defaultProfile)

	assert.Equal(t, accessKey, iniProfile.Key(awsAccessKeyID).Value(), "Expect access key ID to match")
	assert.Equal(t, accessSecretKey, iniProfile.Key(awsSecretAccessKey).Value(), "Expect secret access key to match")
	assert.Equal(t, token, iniProfile.Key(awsSessionToken).Value(), "Expect session token to match")
}

func TestSharedCredentialsStore_ParseError_NoForceUpdate_LeavesFileUnchanged(t *testing.T) {
	// Test setup.  Shared credentials file is corrupt.
	os.Clearenv()
	credFilePath := exampleCredFilePath()
	os.Setenv("AWS_SHARED_CREDENTIALS_FILE", credFilePath)
	fileutil.WriteAllText(credFilePath, "junk")

	// Test
	shareFilePath, _ := GetSharedCredsFilePath("")
	err := Store(log.NewMockLog(), accessKey, accessSecretKey, token, shareFilePath, profile, false)
	assert.NotNil(t, err, "Expect error when saving profile")

	// Verify that the file is unchanged
	contents, err := fileutil.ReadAllText(credFilePath)
	os.Setenv("AWS_SHARED_CREDENTIALS_FILE", "")

	assert.Equal(t, "junk", contents)
}

func TestSharedCredentialsStore_ParseError_ForceUpdate_OverwritesFile(t *testing.T) {
	// Test setup.  Shared credentials file is corrupt.
	os.Clearenv()
	credFilePath := exampleCredFilePath()
	os.Setenv("AWS_SHARED_CREDENTIALS_FILE", credFilePath)
	fileutil.WriteAllText(credFilePath, "junk")

	// Test
	shareFilePath, _ := GetSharedCredsFilePath("")
	err := Store(log.NewMockLog(), accessKey, accessSecretKey, token, shareFilePath, profile, true)
	assert.Nil(t, err, "Expect no error when saving profile")

	// Verify that the shared credentials file has been replaced with a new one
	config, err := ini.Load(credFilePath)
	os.Setenv("AWS_SHARED_CREDENTIALS_FILE", "")
	assert.Nil(t, err, "Expect no error loading file")

	iniProfile := config.Section(profile)
	assert.Equal(t, accessKey, iniProfile.Key(awsAccessKeyID).Value(), "Expect access key ID to match")
	assert.Equal(t, accessSecretKey, iniProfile.Key(awsSecretAccessKey).Value(), "Expect secret access key to match")
	assert.Equal(t, token, iniProfile.Key(awsSessionToken).Value(), "Expect session token to match")
}

func TestSharedCredentialsFilenameFromUserProfile(t *testing.T) {
	// Test setup
	os.Setenv("USERPROFILE", "hometest")
	os.Setenv("HOME", "hometest")

	file2, err2 := GetSharedCredsFilePath("")

	assert.Nil(t, err2, "Expect no error when HOME is set")
	assert.Equal(t, filepath.Join("hometest", ".aws", "credentials"), file2, "HOME dir does not match")
}
