// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package ssms3

import (
	"errors"
	"fmt"
	"runtime"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetLatestVersion_NumericSort(t *testing.T) {
	versions := [3]string{"1.0.0", "2.0.0", "10.0.0"}
	latest := getLatestVersion(versions[:], "")
	assert.Equal(t, "10.0.0", latest)
}

func TestGetLatestVersion_OnlyOneValid(t *testing.T) {
	versions := [3]string{"0.0.0", "1.0", "1.0.0.0"}
	latest := getLatestVersion(versions[:], "")
	assert.Equal(t, "0.0.0", latest)
}

func TestGetLatestVersion_NoneValid(t *testing.T) {
	versions := [3]string{"Foo", "1.0", "1.0.0.0"}
	latest := getLatestVersion(versions[:], "")
	assert.Equal(t, "", latest)
}

func TestGetLatestVersion_None(t *testing.T) {
	versions := make([]string, 0)
	latest := getLatestVersion(versions[:], "")
	assert.Equal(t, "", latest)
}

func TestEndpointEuCentral1(t *testing.T) {
	service := New("", "eu-central-1")
	assert.Equal(t, fmt.Sprintf("https://s3.eu-central-1.amazonaws.com/amazon-ssm-packages-eu-central-1/BirdwatcherPackages/{PackageName}/%v/%v", appconfig.PackagePlatform, runtime.GOARCH), service.packageURL)
}

func TestEndpointEuCentral1Beta(t *testing.T) {
	service := New("beta", "eu-central-1")
	assert.Equal(t, fmt.Sprintf("https://s3.amazonaws.com/amazon-ssm-packages-beta/BirdwatcherPackages/{PackageName}/%v/%v", appconfig.PackagePlatform, runtime.GOARCH), service.packageURL)
}

func TestEndpointEuCentral1Gamma(t *testing.T) {
	service := New("gamma", "eu-central-1")
	assert.Equal(t, fmt.Sprintf("https://s3.amazonaws.com/amazon-ssm-packages-us-east-1-gamma/BirdwatcherPackages/{PackageName}/%v/%v", appconfig.PackagePlatform, runtime.GOARCH), service.packageURL)
}

func TestEndpointCnNorth1(t *testing.T) {
	service := New("", "cn-north-1")
	assert.Equal(t, fmt.Sprintf("https://s3.cn-north-1.amazonaws.com.cn/amazon-ssm-packages-cn-north-1/BirdwatcherPackages/{PackageName}/%v/%v", appconfig.PackagePlatform, runtime.GOARCH), service.packageURL)
}

func TestEndpointCnNorth1Beta(t *testing.T) {
	service := New("beta", "cn-north-1")
	assert.Equal(t, fmt.Sprintf("https://s3.amazonaws.com/amazon-ssm-packages-beta/BirdwatcherPackages/{PackageName}/%v/%v", appconfig.PackagePlatform, runtime.GOARCH), service.packageURL)
}

func TestEndpointCnNorth1Gamma(t *testing.T) {
	service := New("gamma", "cn-north-1")
	assert.Equal(t, fmt.Sprintf("https://s3.amazonaws.com/amazon-ssm-packages-us-east-1-gamma/BirdwatcherPackages/{PackageName}/%v/%v", appconfig.PackagePlatform, runtime.GOARCH), service.packageURL)
}

func TestEndpointUsEast1(t *testing.T) {
	service := New("", "us-east-1")
	assert.Equal(t, fmt.Sprintf("https://s3.amazonaws.com/amazon-ssm-packages-us-east-1/BirdwatcherPackages/{PackageName}/%v/%v", appconfig.PackagePlatform, runtime.GOARCH), service.packageURL)
}

func TestEndpointUsEast1Beta(t *testing.T) {
	service := New("beta", "us-east-1")
	assert.Equal(t, fmt.Sprintf("https://s3.amazonaws.com/amazon-ssm-packages-beta/BirdwatcherPackages/{PackageName}/%v/%v", appconfig.PackagePlatform, runtime.GOARCH), service.packageURL)
}

func TestEndpointUsEast1Gamma(t *testing.T) {
	service := New("gamma", "us-east-1")
	assert.Equal(t, fmt.Sprintf("https://s3.amazonaws.com/amazon-ssm-packages-us-east-1-gamma/BirdwatcherPackages/{PackageName}/%v/%v", appconfig.PackagePlatform, runtime.GOARCH), service.packageURL)
}

func TestGetLatestVersion_RandomStringsAreNotValid(t *testing.T) {
	versions := []string{"foo", "bar", "asdf", "1234567890", "foo.bar.abc", "-10.-10.-10", "1234567890.asdf.1234567890", "123.abcd"}
	latest := getLatestVersion(versions[:], "")
	assert.Equal(t, "", latest)
}

func TestGetLatestVersion_DifferentLengthMajorMinorBuildVersion(t *testing.T) {
	versions := []string{"123.0.126789", "12.3455.67", "65535.8765432.6543"}
	latest := getLatestVersion(versions[:], "")
	assert.Equal(t, "65535.8765432.6543", latest)
}

func TestGetLatestVersion_ZeroStartingMajorMinorBuildVersion(t *testing.T) {
	versions := []string{"01.1.1", "0.02.1", "0.1.02", "03.03.03"}
	latest := getLatestVersion(versions[:], "")
	assert.Equal(t, "03.03.03", latest)
}

func TestGetLatestVersion_OnlyMajorMinorVersionBuildFormatsAreValid(t *testing.T) {
	versions := []string{"1.0.0", "0.0", "1", "", "4.5.6.7.8.9.1.2.3"}
	latest := getLatestVersion(versions[:], "")
	assert.Equal(t, "1.0.0", latest)
}

func TestGetLatestVersion_NegativeVersionsAreNotValid(t *testing.T) {
	versions := []string{"-1.-1.-1", "-2.0.1", "0.1.-1", "1.-2.0"}
	latest := getLatestVersion(versions[:], "")
	assert.Equal(t, "", latest)
}

func TestSuccessfulDownloadManifest(t *testing.T) {
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")

	ds := &PackageService{packageURL: "https://abc.s3.mock-region.amazonaws.com/"}
	packageArn, result, isSameAsCache, err := ds.DownloadManifest(tracer, "packageName", "1234")

	assert.Equal(t, "packageName", packageArn)
	assert.Equal(t, "1234", result)
	assert.True(t, isSameAsCache)
	assert.NoError(t, err)
}

func TestDownloadManifestWithLatest(t *testing.T) {
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")

	mockObj := new(SSMS3Mock)
	mockObj.On("ListS3Folders", mock.Anything, mock.Anything).Return([]string{"1.0.0", "2.0.0"}, nil)

	networkdep = mockObj

	ds := &PackageService{packageURL: "https://abc.s3.mock-region.amazonaws.com/"}
	packageArn, result, isSameAsCache, err := ds.DownloadManifest(tracer, "packageName", "latest")

	assert.Equal(t, "packageName", packageArn)
	assert.Equal(t, "2.0.0", result)
	assert.True(t, isSameAsCache)
	assert.NoError(t, err)
}

func TestDownloadManifestWithError(t *testing.T) {
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")

	mockObj := new(SSMS3Mock)
	mockObj.On("ListS3Folders", mock.Anything, mock.Anything).Return([]string{"1.0.0", "2.0.0"}, errors.New("testerror"))

	networkdep = mockObj

	ds := &PackageService{packageURL: "https://abc.s3.mock-region.amazonaws.com/"}
	_, _, _, err := ds.DownloadManifest(tracer, "packageName", "latest")

	assert.Error(t, err)
}

func TestSuccessfulDownloadArtifact(t *testing.T) {
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")

	mockObj := new(SSMS3Mock)
	mockObj.On("Download", mock.Anything, mock.Anything).Return(artifact.DownloadOutput{"somePath", false, true}, nil)

	networkdep = mockObj

	ds := &PackageService{packageURL: "https://abc.s3.mock-region.amazonaws.com/"}
	result, err := ds.DownloadArtifact(tracer, "packageName", "1234")

	assert.Equal(t, "somePath", result)
	assert.NoError(t, err)
}

func TestDownloadArtifactWithError(t *testing.T) {
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")

	mockObj := new(SSMS3Mock)
	mockObj.On("Download", mock.Anything, mock.Anything).Return(artifact.DownloadOutput{"somePath", false, true}, errors.New("testerror"))

	networkdep = mockObj

	ds := &PackageService{packageURL: "https://abc.s3.mock-region.amazonaws.com/"}
	_, err := ds.DownloadArtifact(tracer, "packageName", "1234")

	assert.Error(t, err)
}

func TestUseSSMS3Service_True(t *testing.T) {
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")

	mockObj := new(SSMS3Mock)
	mockObj.On("CanGetS3Object", mock.Anything, mock.Anything).Return(true)

	networkdep = mockObj

	assert.True(t, UseSSMS3Service(tracer, "", "eu-central-1"))
}

func TestUseSSMS3Service_False(t *testing.T) {
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")

	mockObj := new(SSMS3Mock)
	mockObj.On("CanGetS3Object", mock.Anything, mock.Anything).Return(false)

	networkdep = mockObj

	assert.False(t, UseSSMS3Service(tracer, "beta", "eu-central-1"))
}
