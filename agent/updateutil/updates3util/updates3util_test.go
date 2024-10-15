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
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
	contextmocks "github.com/aws/amazon-ssm-agent/agent/mocks/context"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	updatemanifestmocks "github.com/aws/amazon-ssm-agent/agent/updateutil/updatemanifest/mocks"
	identityMocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/stretchr/testify/assert"
)

func TestResolveManifestUrl_RegionError(t *testing.T) {
	identity := &identityMocks.IAgentIdentity{}
	context := &contextmocks.Mock{}

	context.On("Identity").Return(identity)
	context.On("Log").Return(logmocks.NewMockLog())
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
	context := &contextmocks.Mock{}

	context.On("Identity").Return(identity)
	context.On("Log").Return(logmocks.NewMockLog())
	identity.On("GetServiceEndpoint", "s3").Return("SomeRandom_{Region}_URL")
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
	context := &contextmocks.Mock{}

	context.On("Identity").Return(identity)
	context.On("Log").Return(logmocks.NewMockLog())
	identity.On("GetServiceEndpoint", "s3").Return("")
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
	context := &contextmocks.Mock{}

	manifest := &updatemanifestmocks.T{}

	context.On("Identity").Return(identity)
	context.On("Log").Return(logmocks.NewMockLog())
	util := &updateS3UtilImpl{
		context,
	}

	identity.On("Region").Return("", fmt.Errorf("SomeResolveError")).Once()

	err := util.DownloadManifest(manifest, "RandomManifestURL")

	assert.NotNil(t, err)
	assert.Equal(t, "SomeResolveError", err.Error.Error())
	assert.Equal(t, string(ResolveManifestURLErrorCode), err.ErrorCode)
}

func TestDownloadManifest_FailedCreateTempDir(t *testing.T) {
	createTempDir = func(string, string) (string, error) { return "", fmt.Errorf("SomeTmpError") }
	identity := &identityMocks.IAgentIdentity{}
	context := &contextmocks.Mock{}

	manifest := &updatemanifestmocks.T{}

	context.On("Identity").Return(identity)
	context.On("Log").Return(logmocks.NewMockLog())
	util := &updateS3UtilImpl{
		context,
	}

	identity.On("Region").Return("SomeRegion", nil).Once()

	err := util.DownloadManifest(manifest, "RandomManifestURL")
	assert.NotNil(t, err)
	assert.Equal(t, "SomeTmpError", err.Error.Error())
}

func TestDownloadManifest_FailedDownloadFile(t *testing.T) {
	createTempDir = func(string, string) (string, error) { return "SomePath", nil }
	removeDir = func(string) error { return nil }

	identity := &identityMocks.IAgentIdentity{}
	mockContext := &contextmocks.Mock{}

	manifest := &updatemanifestmocks.T{}

	mockContext.On("Identity").Return(identity)
	mockContext.On("Log").Return(logmocks.NewMockLog())
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
	assert.Equal(t, "failed to download file reliably, RandomManifestURL, SomeDownloadError", err.Error.Error())
	assert.Equal(t, string(NetworkFileDownloadErrorCode), err.ErrorCode)

	// filedownload hash not matched
	fileDownload = func(context.T, artifact.DownloadInput) (artifact.DownloadOutput, error) {
		return artifact.DownloadOutput{IsHashMatched: false, LocalFilePath: "SomeLocalPath"}, nil
	}

	err = util.DownloadManifest(manifest, "RandomManifestURL")
	assert.NotNil(t, err)
	assert.Equal(t, "failed to download file reliably, RandomManifestURL", err.Error.Error())
	assert.Equal(t, string(HashMismatchErrorCode), err.ErrorCode)

	// filedownload local path empty
	fileDownload = func(context.T, artifact.DownloadInput) (artifact.DownloadOutput, error) {
		return artifact.DownloadOutput{IsHashMatched: true, LocalFilePath: ""}, nil
	}

	err = util.DownloadManifest(manifest, "RandomManifestURL")
	assert.NotNil(t, err)
	assert.Equal(t, "failed to download file reliably, RandomManifestURL", err.Error.Error())
	assert.Equal(t, string(LocalFilePathEmptyErrorCode), err.ErrorCode)
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
	mockContext := &contextmocks.Mock{}

	manifest := &updatemanifestmocks.T{}

	mockContext.On("Identity").Return(identity)
	mockContext.On("Log").Return(logmocks.NewMockLog())
	util := &updateS3UtilImpl{
		mockContext,
	}

	identity.On("Region").Return("SomeRegion", nil).Once()
	manifest.On("LoadManifest", "SomeLocalPath").Return(fmt.Errorf("SomeLoadManifestError"))

	err := util.DownloadManifest(manifest, "RandomManifestURL")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error.Error(), "RandomManifestURL")
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
	mockContext := &contextmocks.Mock{}

	manifest := &updatemanifestmocks.T{}

	mockContext.On("Identity").Return(identity)
	mockContext.On("Log").Return(logmocks.NewMockLog())
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
	mockContext := &contextmocks.Mock{}

	manifest := &updatemanifestmocks.T{}

	mockContext.On("Identity").Return(identity)
	mockContext.On("Log").Return(logmocks.NewMockLog())
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
	mockContext := &contextmocks.Mock{}

	manifest := &updatemanifestmocks.T{}

	mockContext.On("Identity").Return(identity)
	mockContext.On("Log").Return(logmocks.NewMockLog())
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
	mockContext := &contextmocks.Mock{}

	manifest := &updatemanifestmocks.T{}

	mockContext.On("Identity").Return(identity)
	mockContext.On("Log").Return(logmocks.NewMockLog())
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
	mockContext := &contextmocks.Mock{}

	manifest := &updatemanifestmocks.T{}

	mockContext.On("Identity").Return(identity)
	mockContext.On("Log").Return(logmocks.NewMockLog())
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
	mockContext := &contextmocks.Mock{}

	manifest := &updatemanifestmocks.T{}

	mockContext.On("Identity").Return(identity)
	mockContext.On("Log").Return(logmocks.NewMockLog())
	util := &updateS3UtilImpl{
		mockContext,
	}

	manifest.On("GetLatestVersion", updaterPackageName).Return(expectedVersion, nil).Once()
	manifest.On("GetDownloadURLAndHash", updaterPackageName, expectedVersion).Return("someurl", "somehash", nil).Once()

	version, err := util.DownloadUpdater(manifest, updaterPackageName, "SomeDownloadPath")

	assert.Nil(t, err)
	assert.Equal(t, expectedVersion, version)
}

func TestGetStableVersion_s3httpDownload_Success(t *testing.T) {
	mockContext := contextmocks.NewMockDefault()
	util := &updateS3UtilImpl{
		mockContext,
	}
	// s3 success
	s3FileRead = func(context context.T, stableVersionUrl string) (output []byte, err error) {
		return []byte("3.1.1.1"), nil
	}
	https3Download = func(stableVersionUrl string, client *http.Client) ([]byte, error) {
		return []byte("3.2.1.1"), nil
	}
	version, err := util.GetStableVersion("")
	assert.Equal(t, "3.1.1.1", version)
	assert.Nil(t, err)

	// s3 fail & http success
	s3FileRead = func(context context.T, stableVersionUrl string) (output []byte, err error) {
		return []byte("3.1.1.1"), fmt.Errorf("s3 download failed")
	}
	https3Download = func(stableVersionUrl string, client *http.Client) ([]byte, error) {
		return []byte("3.2.1.1"), nil
	}
	version, err = util.GetStableVersion("stableUrl")
	assert.Equal(t, "3.2.1.1", version)
	assert.Nil(t, err)

	// s3 fail & http success
	s3FileRead = func(context context.T, stableVersionUrl string) (output []byte, err error) {
		return nil, nil
	}
	https3Download = func(stableVersionUrl string, client *http.Client) ([]byte, error) {
		return []byte("3.2.1.1"), nil
	}
	version, err = util.GetStableVersion("stableUrl")
	assert.Equal(t, "3.2.1.1", version)
	assert.Nil(t, err)
}

func TestGetStableVersion_s3httpDownload_Failed(t *testing.T) {
	mockContext := contextmocks.NewMockDefault()
	util := &updateS3UtilImpl{
		mockContext,
	}
	s3FileRead = func(context context.T, stableVersionUrl string) (output []byte, err error) {
		return []byte("3.1.1.1"), fmt.Errorf("s3 download failed")
	}
	https3Download = func(stableVersionUrl string, client *http.Client) ([]byte, error) {
		return []byte("3.2.1.1"), fmt.Errorf("http download failed")
	}
	version, err := util.GetStableVersion("stableUrl")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "http download failed")
	assert.Empty(t, version)
}

func TestGetStableVersion_InvalidVersion(t *testing.T) {
	mockContext := contextmocks.NewMockDefault()
	util := &updateS3UtilImpl{
		mockContext,
	}

	versionsToTest := []string{
		"3.1.1.1.1",
		"3.1.1.a",
		"3.1.a.1",
		"3.a.1.1",
		"a.1.1.1",
		"3.1.1.1a",
		"3.1.1",
	}
	for _, versionResponse := range versionsToTest {

		s3FileRead = func(context context.T, stableVersionUrl string) (output []byte, err error) {
			return []byte(versionResponse), nil
		}
		https3Download = func(stableVersionUrl string, client *http.Client) ([]byte, error) {
			return []byte(versionResponse), nil
		}
		version, err := util.GetStableVersion("stableUrl")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid version format returned from")
		if !strings.HasSuffix(err.Error(), versionResponse) {
			assert.Fail(t, fmt.Sprintf("expected error to end with version %s: %s", versionResponse, err.Error()))
		}
		assert.Empty(t, version)
	}
}

func TestHttpDownload_Success(t *testing.T) {
	httpClient := &http.Client{}
	httpClient.Transport = roundTripFunc(func(req *http.Request) *http.Response {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(bytes.NewReader([]byte("3.1.1188.0"))),
			Header:     http.Header{},
		}
	})
	version, err := httpDownload("stableUrl", httpClient)
	assert.NoError(t, err)
	assert.Equal(t, "3.1.1188.0", string(version))
}

func TestHttpDownload_Failure(t *testing.T) {
	httpClient := &http.Client{}
	httpClient.Transport = roundTripFunc(func(req *http.Request) *http.Response {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       ioutil.NopCloser(bytes.NewReader([]byte("3.1.1188.0"))),
			Header:     http.Header{},
		}
	})
	version, err := httpDownload("stableUrl", httpClient)
	assert.Error(t, err)
	assert.Equal(t, "", string(version))
}

type roundTripFunc func(req *http.Request) *http.Response

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}
