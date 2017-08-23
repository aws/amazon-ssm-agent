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

// Package repository_mock implements the mock for Repository.
package repository_mock

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/installer"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/localpackages"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/stretchr/testify/mock"
)

type MockedRepository struct {
	mock.Mock
}

func (repoMock *MockedRepository) GetInstalledVersion(context context.T, packageName string) string {
	args := repoMock.Called(context, packageName)
	return args.String(0)
}

func (repoMock *MockedRepository) ValidatePackage(context context.T, packageName string, version string) error {
	args := repoMock.Called(context, packageName, version)
	return args.Error(0)
}

func (repoMock *MockedRepository) RefreshPackage(context context.T, packageName string, version string, packageServiceName string, downloader localpackages.DownloadDelegate) error {
	args := repoMock.Called(context, packageName, version, packageServiceName, downloader)
	return args.Error(0)
}

func (repoMock *MockedRepository) AddPackage(context context.T, packageName string, version string, packageServiceName string, downloader localpackages.DownloadDelegate) error {
	args := repoMock.Called(context, packageName, version, packageServiceName, downloader)
	return args.Error(0)
}

func (repoMock *MockedRepository) SetInstallState(context context.T, packageName string, version string, state localpackages.InstallState) error {
	args := repoMock.Called(context, packageName, version, state)
	return args.Error(0)
}

func (repoMock *MockedRepository) GetInstallState(context context.T, packageName string) (state localpackages.InstallState, version string) {
	args := repoMock.Called(context, packageName)
	return args.Get(0).(localpackages.InstallState), args.String(1)
}

func (repoMock *MockedRepository) RemovePackage(context context.T, packageName string, version string) error {
	args := repoMock.Called(context, packageName, version)
	return args.Error(0)
}

func (repoMock *MockedRepository) GetInventoryData(context context.T) []model.ApplicationData {
	args := repoMock.Called(context)
	return args.Get(0).([]model.ApplicationData)
}

func (repoMock *MockedRepository) GetInstaller(context context.T,
	configuration contracts.Configuration,
	packageName string,
	version string) installer.Installer {
	args := repoMock.Called(context, configuration, packageName, version)
	return args.Get(0).(installer.Installer)
}

func (repoMock *MockedRepository) ReadManifest(packageName string, packageVersion string) ([]byte, error) {
	args := repoMock.Called(packageName, packageVersion)
	return args.Get(0).([]byte), args.Error(1)
}

func (repoMock *MockedRepository) WriteManifest(packageName string, packageVersion string, content []byte) error {
	args := repoMock.Called(packageName, packageVersion, content)
	return args.Error(0)
}
