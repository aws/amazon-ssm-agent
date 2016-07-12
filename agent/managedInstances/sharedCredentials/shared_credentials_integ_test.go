// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the AWS Customer Agreement (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/agreement/

// +build integration

// package sharedCredentials tests
package sharedCredentials

import (
	"os"
	"testing"

	"github.com/aws/aws-sdk-go/vendor/github.com/go-ini/ini"
	"github.com/stretchr/testify/assert"
)

const (
	accessKey       = "DummyAccessKey"
	accessSecretKey = "DummyAccessSecretKey"
	token           = "DummyToken"
	profile         = "DummyProfile"
)

func TestSharedCredentialsStore(t *testing.T) {
	os.Clearenv()

	os.Setenv("AWS_SHARED_CREDENTIALS_FILE", "./example.ini")

	err1 := Store(accessKey, accessSecretKey, token, profile)
	assert.Nil(t, err1, "Expect no error saving profile")

	config, err2 := ini.Load("./example.ini")
	assert.Nil(t, err2, "Expect no error loading file")

	iniProfile := config.Section(profile)

	assert.Equal(t, accessKey, iniProfile.Key(awsAccessKeyID).Value(), "Expect access key ID to match")
	assert.Equal(t, accessSecretKey, iniProfile.Key(awsSecretAccessKey).Value(), "Expect secret access key to match")
	assert.Equal(t, token, iniProfile.Key(awsSessionToken).Value(), "Expect session token to match")
}

func TestSharedCredentialsStoreDefaultProfile(t *testing.T) {
	os.Clearenv()

	os.Setenv("AWS_SHARED_CREDENTIALS_FILE", "./example.ini")

	err1 := Store(accessKey, accessSecretKey, token, "")
	assert.Nil(t, err1, "Expect no error ssaving profile")

	config, err2 := ini.Load("./example.ini")
	assert.Nil(t, err2, "Expect no error loading file")

	iniProfile := config.Section(defaultProfile)

	assert.Equal(t, accessKey, iniProfile.Key(awsAccessKeyID).Value(), "Expect access key ID to match")
	assert.Equal(t, accessSecretKey, iniProfile.Key(awsSecretAccessKey).Value(), "Expect secret access key to match")
	assert.Equal(t, token, iniProfile.Key(awsSessionToken).Value(), "Expect session token to match")
}

func TestSharedCredentialsFilenameFromUserProfile(t *testing.T) {
	os.Clearenv()

	os.Setenv("HOME", "")
	os.Setenv("USERPROFILE", "")

	_, err1 := filename()

	assert.Error(t, err1, "Expect error when no HOME or USERPROFILE set")

	os.Clearenv()
	os.Setenv("USERPROFILE", "")
	os.Setenv("HOME", "hometest")

	file2, err2 := filename()

	assert.Nil(t, err2, "Expect no error when HOME is set")
	assert.Equal(t, "hometest/.aws/credentials", file2, "HOME dir does not match")

	os.Clearenv()
	os.Setenv("USERPROFILE", "usertest")
	os.Setenv("HOME", "")

	file3, err3 := filename()

	assert.Nil(t, err3, "Expect no error when USERPROFILE is set")
	assert.Equal(t, "usertest/.aws/credentials", file3, "HOME dir does not match")
}
