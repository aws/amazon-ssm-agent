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

// Package updates3util implements the logic for s3 update download
package updates3util

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
	updatemanifestmocks "github.com/aws/amazon-ssm-agent/agent/updateutil/updatemanifest/mocks"
	identityMocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/stretchr/testify/assert"
)

func TestResolveManifestUrl_RegionError(t *testing.T) {
	identity := &identityMocks.IAgentIdentity{}
	context := &context.Mock{}

	context.On("Identity").Return(identity)
	context.On("Log").Return(log.NewMockLog())
	util := &updateS3UtilImpl{
		context,
	}

	identity.On("Region").Return("", fmt.Errorf("SomeRegionError")).Once()
	url, err := util.resolveManifestUrl("")
	assert.NotNil(t, err)
	assert.Equal(t, "", url)
}

func TestResolveManifestUrl_EmptyURL_DynamicEndpointSuccess(t *testing.T) {
	identity := &identityMocks.IAgentIdentity{}
	context := &context.Mock{}

	context.On("Identity").Return(identity)
	context.On("Log").Return(log.NewMockLog())
	identity.On("GetDefaultEndpoint", "s3").Return("SomeRandom_{Region}_URL")
	util := &updateS3UtilImpl{
		context,
	}

	identity.On("Region").Return("BogusRegion", nil).Once()
	url, err := util.resolveManifestUrl("")
	assert.Nil(t, err)
	assert.Equal(t, "https://SomeRandom_BogusRegion_URL/amazon-ssm-BogusRegion/ssm-agent-manifest.json", url)
}

func TestResolveManifestUrl_EmptyURL_EmptyDynamicEndpoint(t *testing.T) {
	identity := &identityMocks.IAgentIdentity{}
	context := &context.Mock{}

	context.On("Identity").Return(identity)
	context.On("Log").Return(log.NewMockLog())
	identity.On("GetDefaultEndpoint", "s3").Return("")
	util := &updateS3UtilImpl{
		context,
	}

	// Assert us-east-1 url
	identity.On("Region").Return("us-east-1", nil).Once()
	url, err := util.resolveManifestUrl("")
	assert.Nil(t, err)
	assert.Equal(t, "https://s3.us-east-1.amazonaws.com/amazon-ssm-us-east-1/ssm-agent-manifest.json", url)

	// Assert cn-north-1 url
	identity.On("Region").Return("cn-north-1", nil).Once()
	url, err = util.resolveManifestUrl("")
	assert.Nil(t, err)
	assert.Equal(t, "https://s3.cn-north-1.amazonaws.com.cn/amazon-ssm-cn-north-1/ssm-agent-manifest.json", url)
}

func TestDownloadManifest_FailedResolveManifestUrl(t *testing.T) {
	identity := &identityMocks.IAgentIdentity{}
	context := &context.Mock{}

	manifest := &updatemanifestmocks.T{}

	context.On("Identity").Return(identity)
	context.On("Log").Return(log.NewMockLog())
	util := &updateS3UtilImpl{
		context,
	}

	identity.On("Region").Return("", fmt.Errorf("SomeResolveError")).Once()

	err := util.DownloadManifest(manifest, "RandomManifestURL")

	assert.NotNil(t, err)
	assert.Equal(t, "SomeResolveError", err.Error())
}

func TestDownloadManifest_FailedCreateTempDir(t *testing.T) {
	createTempDir = func(string, string) (string, error) { return "", fmt.Errorf("SomeTmpError") }
	identity := &identityMocks.IAgentIdentity{}
	context := &context.Mock{}

	manifest := &updatemanifestmocks.T{}

	context.On("Identity").Return(identity)
	context.On("Log").Return(log.NewMockLog())
	util := &updateS3UtilImpl{
		context,
	}

	identity.On("Region").Return("SomeRegion", nil).Once()

	err := util.DownloadManifest(manifest, "RandomManifestURL")
	assert.NotNil(t, err)
	assert.Equal(t, "SomeTmpError", err.Error())
}

func TestDownloadManifest_FailedDownloadFile(t *testing.T) {
	createTempDir = func(string, string) (string, error) { return "SomePath", nil }
	removeDir = func(string) error { return nil }

	identity := &identityMocks.IAgentIdentity{}
	mockContext := &context.Mock{}

	manifest := &updatemanifestmocks.T{}

	mockContext.On("Identity").Return(identity)
	mockContext.On("Log").Return(log.NewMockLog())
	util := &updateS3UtilImpl{
		mockContext,
	}

	identity.On("Region").Return("SomeRegion", nil).Times(3)

	// filedownload error
	fileDownload = func(context.T, artifact.DownloadInput) (artifact.DownloadOutput, error) {
		return artifact.DownloadOutput{}, fmt.Errorf("SomeDownloadError")
	}

	err := util.DownloadManifest(manifest, "RandomManifestURL")
	assert.NotNil(t, err)
	assert.Equal(t, "failed to download file reliably, RandomManifestURL, SomeDownloadError", err.Error())

	// filedownload hash not matched
	fileDownload = func(context.T, artifact.DownloadInput) (artifact.DownloadOutput, error) {
		return artifact.DownloadOutput{IsHashMatched: false, LocalFilePath: "SomeLocalPath"}, nil
	}

	err = util.DownloadManifest(manifest, "RandomManifestURL")
	assert.NotNil(t, err)
	assert.Equal(t, "failed to download file reliably, RandomManifestURL", err.Error())

	// filedownload local path empty
	fileDownload = func(context.T, artifact.DownloadInput) (artifact.DownloadOutput, error) {
		return artifact.DownloadOutput{IsHashMatched: true, LocalFilePath: ""}, nil
	}

	err = util.DownloadManifest(manifest, "RandomManifestURL")
	assert.NotNil(t, err)
	assert.Equal(t, "failed to download file reliably, RandomManifestURL", err.Error())
}

func TestDownloadManifest_FailedLoadManifest(t *testing.T) {
	createTempDir = func(string, string) (string, error) { return "SomePath", nil }
	removeDir = func(string) error { return nil }

	artifactOutput := artifact.DownloadOutput{
		IsHashMatched: true,
		LocalFilePath: "SomeLocalPath",
	}
	fileDownload = func(context.T, artifact.DownloadInput) (artifact.DownloadOutput, error) { return artifactOutput, nil }

	identity := &identityMocks.IAgentIdentity{}
	mockContext := &context.Mock{}

	manifest := &updatemanifestmocks.T{}

	mockContext.On("Identity").Return(identity)
	mockContext.On("Log").Return(log.NewMockLog())
	util := &updateS3UtilImpl{
		mockContext,
	}

	identity.On("Region").Return("SomeRegion", nil).Once()
	manifest.On("LoadManifest", "SomeLocalPath").Return(fmt.Errorf("SomeLoadManifestError"))

	err := util.DownloadManifest(manifest, "RandomManifestURL")
	assert.NotNil(t, err)
	assert.Equal(t, "SomeLoadManifestError", err.Error())
}

func TestDownloadManifest_Success(t *testing.T) {
	createTempDir = func(string, string) (string, error) { return "SomePath", nil }
	removeDir = func(string) error { return nil }

	artifactOutput := artifact.DownloadOutput{
		IsHashMatched: true,
		LocalFilePath: "SomeLocalPath",
	}
	fileDownload = func(context.T, artifact.DownloadInput) (artifact.DownloadOutput, error) { return artifactOutput, nil }

	identity := &identityMocks.IAgentIdentity{}
	mockContext := &context.Mock{}

	manifest := &updatemanifestmocks.T{}

	mockContext.On("Identity").Return(identity)
	mockContext.On("Log").Return(log.NewMockLog())
	util := &updateS3UtilImpl{
		mockContext,
	}

	identity.On("Region").Return("SomeRegion", nil).Once()
	manifest.On("LoadManifest", "SomeLocalPath").Return(nil).Once()

	err := util.DownloadManifest(manifest, "RandomManifestURL")
	assert.Nil(t, err)
}

func TestDownloadUpdater_FailedGetLatestVersion(t *testing.T) {
	updaterPackageName := "UpdaterPackageName"

	createTempDir = func(string, string) (string, error) { return "SomePath", nil }
	removeDir = func(string) error { return nil }

	artifactOutput := artifact.DownloadOutput{
		IsHashMatched: true,
		LocalFilePath: "SomeLocalPath",
	}
	fileDownload = func(context.T, artifact.DownloadInput) (artifact.DownloadOutput, error) { return artifactOutput, nil }

	identity := &identityMocks.IAgentIdentity{}
	mockContext := &context.Mock{}

	manifest := &updatemanifestmocks.T{}

	mockContext.On("Identity").Return(identity)
	mockContext.On("Log").Return(log.NewMockLog())
	util := &updateS3UtilImpl{
		mockContext,
	}

	manifest.On("GetLatestVersion", updaterPackageName).Return("", fmt.Errorf("SomeVersionError")).Once()

	version, err := util.DownloadUpdater(manifest, updaterPackageName, "SomeDownloadPath")

	assert.NotNil(t, err)
	assert.Equal(t, "SomeVersionError", err.Error())
	assert.Equal(t, "", version)
}

func TestDownloadUpdater_FailedGetDownloadURL(t *testing.T) {
	updaterPackageName := "UpdaterPackageName"
	expectedVersion := "-1.-1.-1.-1"

	createTempDir = func(string, string) (string, error) { return "SomePath", nil }
	removeDir = func(string) error { return nil }

	artifactOutput := artifact.DownloadOutput{
		IsHashMatched: true,
		LocalFilePath: "SomeLocalPath",
	}
	fileDownload = func(context.T, artifact.DownloadInput) (artifact.DownloadOutput, error) { return artifactOutput, nil }

	identity := &identityMocks.IAgentIdentity{}
	mockContext := &context.Mock{}

	manifest := &updatemanifestmocks.T{}

	mockContext.On("Identity").Return(identity)
	mockContext.On("Log").Return(log.NewMockLog())
	util := &updateS3UtilImpl{
		mockContext,
	}

	manifest.On("GetLatestVersion", updaterPackageName).Return(expectedVersion, nil).Once()
	manifest.On("GetDownloadURLAndHash", updaterPackageName, expectedVersion).Return("", "", fmt.Errorf("SomeURLError")).Once()

	version, err := util.DownloadUpdater(manifest, updaterPackageName, "SomeDownloadPath")

	assert.NotNil(t, err)
	assert.Equal(t, "SomeURLError", err.Error())
	assert.Equal(t, "", version)
}

func TestDownloadUpdater_FailedDownloadUpdater(t *testing.T) {
	updaterPackageName := "UpdaterPackageName"
	expectedVersion := "-1.-1.-1.-1"

	createTempDir = func(string, string) (string, error) { return "SomePath", nil }
	removeDir = func(string) error { return nil }

	identity := &identityMocks.IAgentIdentity{}
	mockContext := &context.Mock{}

	manifest := &updatemanifestmocks.T{}

	mockContext.On("Identity").Return(identity)
	mockContext.On("Log").Return(log.NewMockLog())
	util := &updateS3UtilImpl{
		mockContext,
	}

	manifest.On("GetLatestVersion", updaterPackageName).Return(expectedVersion, nil).Times(3)
	manifest.On("GetDownloadURLAndHash", updaterPackageName, expectedVersion).Return("someurl", "somehash", nil).Times(3)

	// filedownload error
	fileDownload = func(context.T, artifact.DownloadInput) (artifact.DownloadOutput, error) {
		return artifact.DownloadOutput{}, fmt.Errorf("SomeDownloadError")
	}

	version, err := util.DownloadUpdater(manifest, updaterPackageName, "SomeDownloadPath")
	assert.NotNil(t, err)
	assert.Equal(t, "", version)
	assert.Equal(t, "failed to download file reliably, someurl, SomeDownloadError", err.Error())

	// filedownload hash not matched
	fileDownload = func(context.T, artifact.DownloadInput) (artifact.DownloadOutput, error) {
		return artifact.DownloadOutput{IsHashMatched: false, LocalFilePath: "SomeLocalPath"}, nil
	}

	version, err = util.DownloadUpdater(manifest, updaterPackageName, "SomeDownloadPath")
	assert.NotNil(t, err)
	assert.Equal(t, "", version)
	assert.Equal(t, "failed to download file reliably, someurl", err.Error())

	// filedownload local path empty
	fileDownload = func(context.T, artifact.DownloadInput) (artifact.DownloadOutput, error) {
		return artifact.DownloadOutput{IsHashMatched: true, LocalFilePath: ""}, nil
	}

	version, err = util.DownloadUpdater(manifest, updaterPackageName, "SomeDownloadPath")
	assert.NotNil(t, err)
	assert.Equal(t, "", version)
	assert.Equal(t, "failed to download file reliably, someurl", err.Error())
}

func TestDownloadUpdater_FailedDecompress(t *testing.T) {
	updaterPackageName := "UpdaterPackageName"
	expectedVersion := "-1.-1.-1.-1"

	createTempDir = func(string, string) (string, error) { return "SomePath", nil }
	removeDir = func(string) error { return nil }

	artifactOutput := artifact.DownloadOutput{
		IsHashMatched: true,
		LocalFilePath: "SomeLocalPath",
	}
	fileDownload = func(context.T, artifact.DownloadInput) (artifact.DownloadOutput, error) { return artifactOutput, nil }

	identity := &identityMocks.IAgentIdentity{}
	mockContext := &context.Mock{}

	manifest := &updatemanifestmocks.T{}

	mockContext.On("Identity").Return(identity)
	mockContext.On("Log").Return(log.NewMockLog())
	util := &updateS3UtilImpl{
		mockContext,
	}

	manifest.On("GetLatestVersion", updaterPackageName).Return(expectedVersion, nil).Once()
	manifest.On("GetDownloadURLAndHash", updaterPackageName, expectedVersion).Return("someurl", "somehash", nil).Once()

	fileDecompress = func(log log.T, src, dest string) error { return fmt.Errorf("SomeDecompressError") }
	version, err := util.DownloadUpdater(manifest, updaterPackageName, "SomeDownloadPath")

	assert.NotNil(t, err)
	assert.Equal(t, "failed to decompress updater package, SomeLocalPath, SomeDecompressError", err.Error())
	assert.Equal(t, "", version)
}

func TestDownloadUpdater_Success(t *testing.T) {
	updaterPackageName := "UpdaterPackageName"
	expectedVersion := "-1.-1.-1.-1"

	createTempDir = func(string, string) (string, error) { return "SomePath", nil }
	removeDir = func(string) error { return nil }

	artifactOutput := artifact.DownloadOutput{
		IsHashMatched: true,
		LocalFilePath: "SomeLocalPath",
	}
	fileDownload = func(context.T, artifact.DownloadInput) (artifact.DownloadOutput, error) { return artifactOutput, nil }
	fileDecompress = func(log log.T, src, dest string) error { return nil }

	identity := &identityMocks.IAgentIdentity{}
	mockContext := &context.Mock{}

	manifest := &updatemanifestmocks.T{}

	mockContext.On("Identity").Return(identity)
	mockContext.On("Log").Return(log.NewMockLog())
	util := &updateS3UtilImpl{
		mockContext,
	}

	manifest.On("GetLatestVersion", updaterPackageName).Return(expectedVersion, nil).Once()
	manifest.On("GetDownloadURLAndHash", updaterPackageName, expectedVersion).Return("someurl", "somehash", nil).Once()

	version, err := util.DownloadUpdater(manifest, updaterPackageName, "SomeDownloadPath")

	assert.Nil(t, err)
	assert.Equal(t, expectedVersion, version)
}
