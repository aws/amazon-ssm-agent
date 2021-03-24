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

// Package updatemanifest implements the logic for the ssm agent s3 manifest..
package updatemanifest

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	updateinfomocks "github.com/aws/amazon-ssm-agent/agent/updateutil/updateinfo/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

//Valid manifest files
var sampleManifests = "testdata/sampleManifest.json"

//Invalid manifest files
var errorManifests = "testdata/errorManifest.json"

//Valid manifest file with Status field
var sampleManifestWithStatus = "testdata/sampleManifestWithStatus.json"

func TestParseSimpleManifest_HasVersion(t *testing.T) {
	context := context.NewMockDefault()
	updateInfo := &updateinfomocks.T{}

	// parse manifest
	packageName := "amazon-ssm-agent"
	updateInfo.On("GenerateCompressedFileName", packageName).Return(packageName + "-linux-amd64.tar.gz").Once()

	manifest := New(context, updateInfo)
	err := manifest.LoadManifest(sampleManifests)

	// check results
	assert.Nil(t, err)
	assert.NotNil(t, manifest)

	updateInfo.On("GenerateCompressedFileName", packageName).Return(packageName + "-linux-amd64.tar.gz").Times(2)

	// version exists
	hasVersion := manifest.HasVersion(packageName, "1.1.0.0")
	assert.True(t, hasVersion)

	// version not exists
	hasVersion = manifest.HasVersion(packageName, "1.1.0.3")
	assert.False(t, hasVersion)

	// package not exist
	packageName = "amazon-ssm-agent"
	updateInfo.On("GenerateCompressedFileName", packageName).Return(packageName + "-archlinux-amd64.tar.gz").Twice()
	hasVersion = manifest.HasVersion(packageName, "1.1.0.0")
	assert.False(t, hasVersion)
}

func TestParseSimpleManifest_GetDownloadURLHash(t *testing.T) {
	context := context.NewMockDefault()
	updateInfo := &updateinfomocks.T{}

	// parse manifest
	packageName := "amazon-ssm-agent"
	updateInfo.On("GenerateCompressedFileName", packageName).Return(packageName + "-linux-amd64.tar.gz").Once()
	manifest := New(context, updateInfo)
	err := manifest.LoadManifest(sampleManifests)

	// check results
	assert.Nil(t, err)
	assert.NotNil(t, manifest)

	updateInfo.On("GenerateCompressedFileName", packageName).Return(packageName + "-linux-amd64.tar.gz").Twice()

	// version exists
	url, hash, err := manifest.GetDownloadURLAndHash(packageName, "1.1.43.0")
	assert.Nil(t, err)
	assert.Equal(t, "https://s3.amazonaws.com/some-agent-bucket/amazon-ssm-agent/1.1.43.0/amazon-ssm-agent-linux-amd64.tar.gz", url)
	assert.Equal(t, "bc477b4ea68756e3a83b93445cc1bbdd5f9465b3334f7ecf58c69771956d5673", hash)

	// version not exists
	url, hash, err = manifest.GetDownloadURLAndHash(packageName, "1.3.3.7")
	assert.NotNil(t, err)
	assert.Equal(t, "", url)
	assert.Equal(t, "", hash)

	// package not exist
	packageName = "amazon-ssm-agent"
	updateInfo.On("GenerateCompressedFileName", packageName).Return(packageName + "-archlinux-amd64.tar.gz").Twice()
	url, hash, err = manifest.GetDownloadURLAndHash(packageName, "7.3.3.1")
	assert.NotNil(t, err)
	assert.Equal(t, "", url)
	assert.Equal(t, "", hash)
}

//Test ParseManifest with invalid manifest files
func TestParseManifestWithError(t *testing.T) {
	context := context.NewMockDefault()
	updateInfo := &updateinfomocks.T{}

	// parse manifest
	manifest := New(context, updateInfo)
	err := manifest.LoadManifest(errorManifests)

	// check results
	assert.NotNil(t, err)
	assert.NotNil(t, manifest)
}

//Test parsing manifest file with version Status field
func TestParseManifestWithStatusUpdater(t *testing.T) {
	packageName := "amazon-ssm-agent-updater"

	context := context.NewMockDefault()
	updateInfo := &updateinfomocks.T{}

	updateInfo.On("GenerateCompressedFileName", mock.Anything).Return(func(arg string) string { return arg + "-linux-amd64.tar.gz" })

	// parse manifest
	manifest := New(context, updateInfo)
	err := manifest.LoadManifest(sampleManifestWithStatus)

	// check results
	assert.Nil(t, err)
	assert.NotNil(t, manifest)

	// Invalid status
	isActiveStatus, err := manifest.IsVersionActive(packageName, "1.1.0.0")
	assert.False(t, isActiveStatus)
	assert.NotNil(t, err)

	// Empty status -> active
	isActiveStatus, err = manifest.IsVersionActive(packageName, "1.2.251.0")
	assert.True(t, isActiveStatus)
	assert.Nil(t, err)

	// Active status
	isActiveStatus, err = manifest.IsVersionActive(packageName, "2.2.45.0")
	assert.True(t, isActiveStatus)
	assert.Nil(t, err)

	// Inactive Status
	isActiveStatus, err = manifest.IsVersionActive(packageName, "2.2.58.0")
	assert.False(t, isActiveStatus)
	assert.Nil(t, err)

	// Deprecated Status
	isActiveStatus, err = manifest.IsVersionActive(packageName, "2.2.64.0")
	assert.False(t, isActiveStatus)
	assert.Nil(t, err)

	// Active Status
	isActiveStatus, err = manifest.IsVersionActive(packageName, "2.2.69.0")
	assert.True(t, isActiveStatus)
	assert.Nil(t, err)

	url, hash, err := manifest.GetDownloadURLAndHash(packageName, "2.2.69.0")
	assert.Equal(t, "https://s3.amazonaws.com/some-agent-bucket/amazon-ssm-agent-updater/2.2.69.0/amazon-ssm-agent-updater-linux-amd64.tar.gz", url)
	assert.Equal(t, "3127c6c149711c3a3ea53aed7d2167bfc9049d9388e36d19f2f400a6fdeac213", hash)
}

//Test parsing manifest file with version Status field
func TestParseManifestWithStatusAgent(t *testing.T) {
	packageName := "amazon-ssm-agent"
	arch := "amd64"

	context := context.NewMockDefault()
	updateInfo := &updateinfomocks.T{}

	updateInfo.On("GenerateCompressedFileName", mock.Anything).Return(func(arg string) string { return arg + "-linux-" + arch + ".tar.gz" })

	// parse manifest
	manifest := New(context, updateInfo)
	err := manifest.LoadManifest(sampleManifestWithStatus)

	// check results
	assert.Nil(t, err)
	assert.NotNil(t, manifest)

	// Invalid status
	isActiveStatus, err := manifest.IsVersionActive(packageName, "1.1.0.0")
	assert.False(t, isActiveStatus)
	assert.NotNil(t, err)

	// Empty status -> active
	isActiveStatus, err = manifest.IsVersionActive(packageName, "1.2.251.0")
	assert.True(t, isActiveStatus)
	assert.Nil(t, err)

	// Active status
	isActiveStatus, err = manifest.IsVersionActive(packageName, "2.0.796.0")
	assert.True(t, isActiveStatus)
	assert.Nil(t, err)

	// Inactive Status
	isActiveStatus, err = manifest.IsVersionActive(packageName, "2.0.805.0")
	assert.False(t, isActiveStatus)
	assert.Nil(t, err)

	// Deprecated Status
	isActiveStatus, err = manifest.IsVersionActive(packageName, "2.0.822.0")
	assert.False(t, isActiveStatus)
	assert.Nil(t, err)

	// Deprecated Status
	isDeprecatedStatus, err := manifest.IsVersionDeprecated(packageName, "2.0.822.0")
	assert.True(t, isDeprecatedStatus)
	assert.Nil(t, err)

	// Active Status
	isActiveStatus, err = manifest.IsVersionActive(packageName, "2.2.96.0")
	assert.True(t, isActiveStatus)
	assert.Nil(t, err)

	url, hash, err := manifest.GetDownloadURLAndHash(packageName, "2.2.96.0")
	assert.Nil(t, err)
	assert.Equal(t, "https://s3.amazonaws.com/some-agent-bucket/amazon-ssm-agent/2.2.96.0/amazon-ssm-agent-linux-amd64.tar.gz", url)
	assert.Equal(t, "2137c6c149711c3a3ea53aed7d2167bfc9049d9388e36d19f2f400a6fdeac312", hash)

	version, err := manifest.GetLatestActiveVersion(packageName)
	assert.Nil(t, err)
	assert.Equal(t, "2.2.96.0", version)

	// Change arch where latest active is 2.2.96.0 is Deprecated
	arch = "arm64"
	isDeprecatedStatus, err = manifest.IsVersionDeprecated(packageName, "2.2.96.0")
	assert.True(t, isDeprecatedStatus)
	assert.Nil(t, err)

	version, err = manifest.GetLatestActiveVersion(packageName)
	assert.Nil(t, err)
	assert.Equal(t, "2.0.796.0", version)

	version, err = manifest.GetLatestVersion(packageName)
	assert.Nil(t, err)
	assert.Equal(t, "2.2.96.0", version)

	// Package does not exist
	version, err = manifest.GetLatestActiveVersion("randomPackage")
	assert.NotNil(t, err)
	assert.Equal(t, "0", version)

	// Package does not exist
	version, err = manifest.GetLatestVersion("randomPackage")
	assert.NotNil(t, err)
	assert.Equal(t, "0", version)
}
