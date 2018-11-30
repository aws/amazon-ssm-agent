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

// Package localpackages implements the local storage for packages managed by the ConfigurePackage plugin.
package localpackages

import (
	"errors"
	"io/ioutil"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/filelock"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/trace"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TODO:MF: test deps, replace filesysdep test version in test_configurepackage with usage of the mocked repository

const testRepoRoot = "testdata"
const testLockRoot = "testlock"
const testPackage = "SsmTest"

var tracerMock = trace.NewTracer(log.NewMockLog())

func TestGetInstaller(t *testing.T) {
	repo := NewRepository()
	inst := repo.GetInstaller(tracerMock, contracts.Configuration{}, testPackage, "1.0.0")
	assert.NotNil(t, inst)
}

func TestGetInstallState(t *testing.T) {
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, "installstate")).Return(true).Once()
	mockFileSys.On("ReadFile", path.Join(testRepoRoot, testPackage, "installstate")).Return(loadFile(t, path.Join(testRepoRoot, testPackage, "installstate_success")), nil).Once()

	// Instantiate repository with mock
	repo := localRepository{filesysdep: &mockFileSys, repoRoot: testRepoRoot, lockRoot: testLockRoot}

	// Call and validate mock expectations and return value
	state, version := repo.GetInstallState(tracerMock, testPackage)
	mockFileSys.AssertExpectations(t)
	assert.Equal(t, Installed, state)
	assert.Equal(t, version, version)
}

func TestGetInstallStateMissing(t *testing.T) {
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, "installstate")).Return(false).Once()
	mockFileSys.On("GetDirectoryNames", path.Join(testRepoRoot, testPackage)).Return(make([]string, 0), nil).Once()

	// Instantiate repository with mock
	repo := localRepository{filesysdep: &mockFileSys, repoRoot: testRepoRoot, lockRoot: testLockRoot}

	// Call and validate mock expectations and return value
	state, version := repo.GetInstallState(tracerMock, testPackage)
	mockFileSys.AssertExpectations(t)
	assert.Equal(t, None, state)
	assert.Equal(t, "", version)
}

func TestGetInstallStateCompat(t *testing.T) {
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, "installstate")).Return(false).Once()
	mockFileSys.On("GetDirectoryNames", path.Join(testRepoRoot, testPackage)).Return([]string{"0.0.1"}, nil).Once()

	// Instantiate repository with mock
	repo := localRepository{filesysdep: &mockFileSys, repoRoot: testRepoRoot, lockRoot: testLockRoot}

	// Call and validate mock expectations and return value
	state, version := repo.GetInstallState(tracerMock, testPackage)
	mockFileSys.AssertExpectations(t)
	assert.Equal(t, Unknown, state)
	assert.Equal(t, version, version)
}

func TestGetInstallStateCorrupt(t *testing.T) {
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, "installstate")).Return(true).Once()
	mockFileSys.On("ReadFile", path.Join(testRepoRoot, testPackage, "installstate")).Return(loadFile(t, path.Join(testRepoRoot, testPackage, "installstate_corrupt")), nil).Once()

	tracerMock.BeginSection("testtrace")

	// Instantiate repository with mock
	repo := localRepository{filesysdep: &mockFileSys, repoRoot: testRepoRoot, lockRoot: testLockRoot}

	// Call and validate mock expectations and return value
	state, version := repo.GetInstallState(tracerMock, testPackage)
	mockFileSys.AssertExpectations(t)
	assert.Equal(t, Unknown, state)
	assert.Equal(t, "", version)
}

func TestGetInstallStateError(t *testing.T) {
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, "installstate")).Return(true).Once()
	mockFileSys.On("ReadFile", path.Join(testRepoRoot, testPackage, "installstate")).Return(make([]byte, 0), errors.New("Failed to read file")).Once()

	// Instantiate repository with mock
	repo := localRepository{filesysdep: &mockFileSys, repoRoot: testRepoRoot, lockRoot: testLockRoot}

	// Call and validate mock expectations and return value
	state, version := repo.GetInstallState(tracerMock, testPackage)
	mockFileSys.AssertExpectations(t)
	assert.Equal(t, Unknown, state)
	assert.Equal(t, "", version)
}

func TestGetInstalledVersion(t *testing.T) {
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, "installstate")).Return(true).Once()
	mockFileSys.On("ReadFile", path.Join(testRepoRoot, testPackage, "installstate")).Return(loadFile(t, path.Join(testRepoRoot, testPackage, "installstate_success")), nil).Once()

	// Instantiate repository with mock
	repo := localRepository{filesysdep: &mockFileSys, repoRoot: testRepoRoot, lockRoot: testLockRoot}

	// Call and validate mock expectations and return value
	version := repo.GetInstalledVersion(tracerMock, testPackage)
	mockFileSys.AssertExpectations(t)
	assert.Equal(t, version, version)
}

func TestGetInstalledVersionCompat(t *testing.T) {
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, "installstate")).Return(false).Once()
	mockFileSys.On("GetDirectoryNames", path.Join(testRepoRoot, testPackage)).Return([]string{"0.0.1"}, nil).Once()

	// Instantiate repository with mock
	repo := localRepository{filesysdep: &mockFileSys, repoRoot: testRepoRoot, lockRoot: testLockRoot}

	// Call and validate mock expectations and return value
	version := repo.GetInstalledVersion(tracerMock, testPackage)
	mockFileSys.AssertExpectations(t)
	assert.Equal(t, version, version)
}

func TestGetInstalledVersionInstalling(t *testing.T) {
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, "installstate")).Return(true).Once()
	mockFileSys.On("ReadFile", path.Join(testRepoRoot, testPackage, "installstate")).Return(loadFile(t, path.Join(testRepoRoot, testPackage, "installstate_installing")), nil).Once()

	// Instantiate repository with mock
	repo := localRepository{filesysdep: &mockFileSys, repoRoot: testRepoRoot, lockRoot: testLockRoot}

	// Call and validate mock expectations and return value
	version := repo.GetInstalledVersion(tracerMock, testPackage)
	mockFileSys.AssertExpectations(t)
	assert.Equal(t, version, version)
}

func TestValidatePackage(t *testing.T) {
	version := "0.0.1"
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, version, "manifest.json")).Return(true).Once()
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, version, "install.ps1")).Return(true).Once()
	mockFileSys.On("ReadFile", path.Join(testRepoRoot, testPackage, version, "manifest.json")).Return(loadFile(t, path.Join(testRepoRoot, testPackage, version, "manifest.json")), nil).Once()
	mockFileSys.On("GetFileNames", path.Join(testRepoRoot, testPackage, version)).Return([]string{"manifest.json", "install.json"}, nil)
	mockFileSys.On("GetDirectoryNames", path.Join(testRepoRoot, testPackage, version)).Return([]string{}, nil)

	// Instantiate repository with mock
	repo := localRepository{filesysdep: &mockFileSys, repoRoot: testRepoRoot, lockRoot: testLockRoot, fileLocker: &filelock.FileLockerNoop{}}

	// Call and validate mock expectations and return value
	err := repo.ValidatePackage(tracerMock, testPackage, version)
	mockFileSys.AssertExpectations(t)
	assert.Nil(t, err)
}

func TestValidatePackage_Manifest(t *testing.T) {
	version := "0.0.1"
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, version, "manifest.json")).Return(true).Once()
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, version, "install.ps1")).Return(true).Once()
	mockFileSys.On("ReadFile", path.Join(testRepoRoot, testPackage, version, "manifest.json")).Return(loadFile(t, path.Join(testRepoRoot, testPackage, version, "manifest.json")), nil).Once()
	mockFileSys.On("GetFileNames", path.Join(testRepoRoot, testPackage, version)).Return([]string{"manifest.json", "install.json"}, nil)
	mockFileSys.On("GetDirectoryNames", path.Join(testRepoRoot, testPackage, version)).Return([]string{}, nil)

	// Instantiate repository with mock
	repo := localRepository{filesysdep: &mockFileSys, repoRoot: testRepoRoot, lockRoot: testLockRoot, fileLocker: &filelock.FileLockerNoop{}}

	// Call and validate mock expectations and return value
	err := repo.ValidatePackage(tracerMock, testPackage, version)
	mockFileSys.AssertExpectations(t)
	assert.Nil(t, err)
}

func TestValidatePackage_NoManifest(t *testing.T) {
	version := "0.0.1"
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, version, "manifest.json")).Return(false).Once()
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, version, "install.ps1")).Return(true).Once()
	mockFileSys.On("GetFileNames", path.Join(testRepoRoot, testPackage, version)).Return([]string{"install.json", "uninstall.json"}, nil)
	mockFileSys.On("GetDirectoryNames", path.Join(testRepoRoot, testPackage, version)).Return([]string{}, nil)

	// Instantiate repository with mock
	repo := localRepository{filesysdep: &mockFileSys, repoRoot: testRepoRoot, lockRoot: testLockRoot, fileLocker: &filelock.FileLockerNoop{}}

	// Call and validate mock expectations and return value
	err := repo.ValidatePackage(tracerMock, testPackage, version)
	mockFileSys.AssertExpectations(t)
	assert.Nil(t, err)
}

func TestValidatePackageNoContent(t *testing.T) {
	version := "0.0.1"
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, version, "manifest.json")).Return(true).Once()
	mockFileSys.On("ReadFile", path.Join(testRepoRoot, testPackage, version, "manifest.json")).Return(loadFile(t, path.Join(testRepoRoot, testPackage, version, "manifest.json")), nil).Once()
	mockFileSys.On("GetFileNames", path.Join(testRepoRoot, testPackage, version)).Return([]string{"manifest.json"}, nil)
	mockFileSys.On("GetDirectoryNames", path.Join(testRepoRoot, testPackage, version)).Return([]string{}, nil)

	// Instantiate repository with mock
	repo := localRepository{filesysdep: &mockFileSys, repoRoot: testRepoRoot, lockRoot: testLockRoot, fileLocker: &filelock.FileLockerNoop{}}

	// Call and validate mock expectations and return value
	err := repo.ValidatePackage(tracerMock, testPackage, version)
	mockFileSys.AssertExpectations(t)
	assert.NotNil(t, err)
	assert.True(t, strings.EqualFold(err.Error(), "Package is incomplete"))
}

func TestValidatePackageCorruptManifest(t *testing.T) {
	version := "0.0.10"
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, version, "manifest.json")).Return(true).Once()
	mockFileSys.On("ReadFile", path.Join(testRepoRoot, testPackage, version, "manifest.json")).Return(loadFile(t, path.Join(testRepoRoot, testPackage, version, "manifest.json")), nil).Once()

	// Instantiate repository with mock
	repo := localRepository{filesysdep: &mockFileSys, repoRoot: testRepoRoot, lockRoot: testLockRoot, fileLocker: &filelock.FileLockerNoop{}}

	// Call and validate mock expectations and return value
	err := repo.ValidatePackage(tracerMock, testPackage, version)
	mockFileSys.AssertExpectations(t)
	assert.NotNil(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "Package manifest is invalid:"))
}

func TestValidatePackageManifest(t *testing.T) {
	data := []struct {
		name        string
		manifest    *PackageManifest
		arn         string
		version     string
		expectedErr bool
	}{
		{
			"empty manifest",
			&PackageManifest{},
			"arn",
			"version",
			true,
		},
		{
			"empty manifest name",
			&PackageManifest{Name: ""},
			"arn",
			"version",
			true,
		},
		{
			"empty manifest version",
			&PackageManifest{Name: "", Version: ""},
			"arn",
			"version",
			true,
		},
		{
			"non matching manifest name",
			&PackageManifest{Name: "not-arn", Version: "version"},
			"arn",
			"version",
			true,
		},
		{
			"non matching manifest name",
			&PackageManifest{Name: "arn", Version: "not-version"},
			"arn",
			"version",
			true,
		},
		{
			"full arn",
			&PackageManifest{Name: "arn:aws:ssm:us-east-1:401613528637:package/HzsqFmONmi", Version: "version"},
			"arn:aws:ssm:us-east-1:401613528637:package/HzsqFmONmi",
			"version",
			false,
		},
		{
			"short arn",
			&PackageManifest{Name: "arn:aws:ssm:::package/HzsqFmONmi", Version: "version"},
			"arn:aws:ssm:::package/HzsqFmONmi",
			"version",
			false,
		},
		{
			"package name",
			&PackageManifest{Name: "HzsqFmONmi", Version: "version"},
			"arn:aws:ssm:::package/HzsqFmONmi",
			"version",
			false,
		},
	}

	for _, testdata := range data {
		t.Run(testdata.name, func(t *testing.T) {
			err := validatePackageManifest(testdata.manifest, testdata.arn, testdata.version)

			if testdata.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAddPackage(t *testing.T) {
	version := "0.0.1"
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("MakeDirExecute", path.Join(testRepoRoot, testPackage, version)).Return(nil).Once()
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, "installstate")).Return(true).Once()
	mockFileSys.On("ReadFile", path.Join(testRepoRoot, testPackage, "installstate")).Return(loadFile(t, path.Join(testRepoRoot, testPackage, "installstate_success")), nil).Once()

	mockDownload := MockedDownloader{}
	mockDownload.On("Download", tracerMock, path.Join(testRepoRoot, testPackage, version)).Return(nil).Once()

	// Instantiate repository with mock
	repo := localRepository{filesysdep: &mockFileSys, repoRoot: testRepoRoot, lockRoot: testLockRoot, fileLocker: &filelock.FileLockerNoop{}}

	// Call and validate mock expectations and return value
	err := repo.AddPackage(tracerMock, testPackage, version, "mock-package-service", mockDownload.Download)
	mockFileSys.AssertExpectations(t)
	mockDownload.AssertExpectations(t)
	assert.Nil(t, err)
}

func TestAddNewPackage(t *testing.T) {
	version := "0.0.1"
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("MakeDirExecute", path.Join(testRepoRoot, testPackage, version)).Return(nil).Once()
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, "installstate")).Return(false).Once()
	mockFileSys.On("GetDirectoryNames", path.Join(testRepoRoot, testPackage)).Return(make([]string, 0), nil).Once()
	mockFileSys.On("WriteFile", path.Join(testRepoRoot, testPackage, "installstate"), mock.Anything).Return(nil).Once()

	mockDownload := MockedDownloader{}
	mockDownload.On("Download", tracerMock, path.Join(testRepoRoot, testPackage, version)).Return(nil).Once()

	// Instantiate repository with mock
	repo := localRepository{filesysdep: &mockFileSys, repoRoot: testRepoRoot, lockRoot: testLockRoot, fileLocker: &filelock.FileLockerNoop{}}

	// Call and validate mock expectations and return value
	err := repo.AddPackage(tracerMock, testPackage, version, "mock-package-service", mockDownload.Download)
	mockFileSys.AssertExpectations(t)
	mockDownload.AssertExpectations(t)
	assert.Nil(t, err)
}

func TestAddPackageWithDownloadFailure(t *testing.T) {
	version := "0.0.1"
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("MakeDirExecute", path.Join(testRepoRoot, testPackage, version)).Return(nil).Once()
	mockFileSys.On("RemoveAll", path.Join(testRepoRoot, testPackage, version)).Return(nil).Once()

	mockDownload := MockedDownloader{}
	mockDownload.On("Download", tracerMock, path.Join(testRepoRoot, testPackage, version)).Return(errors.New("Download error.")).Once()

	// Instantiate repository with mock
	repo := localRepository{filesysdep: &mockFileSys, repoRoot: testRepoRoot, lockRoot: testLockRoot, fileLocker: &filelock.FileLockerNoop{}}

	// Call and validate mock expectations and return value
	err := repo.AddPackage(tracerMock, testPackage, version, "mock-package-service", mockDownload.Download)
	mockFileSys.AssertExpectations(t)
	mockDownload.AssertExpectations(t)
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "Download error.")
}

func TestRefreshPackage(t *testing.T) {
	version := "0.0.1"
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("MakeDirExecute", path.Join(testRepoRoot, testPackage, version)).Return(nil).Once()
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, "installstate")).Return(true).Once()
	mockFileSys.On("ReadFile", path.Join(testRepoRoot, testPackage, "installstate")).Return(loadFile(t, path.Join(testRepoRoot, testPackage, "installstate_success")), nil).Once()

	mockDownload := MockedDownloader{}
	mockDownload.On("Download", tracerMock, path.Join(testRepoRoot, testPackage, version)).Return(nil).Once()

	// Instantiate repository with mock
	repo := localRepository{filesysdep: &mockFileSys, repoRoot: testRepoRoot, lockRoot: testLockRoot, fileLocker: &filelock.FileLockerNoop{}}

	// Call and validate mock expectations and return value
	err := repo.RefreshPackage(tracerMock, testPackage, version, "mock-package-service", mockDownload.Download)
	mockFileSys.AssertExpectations(t)
	mockDownload.AssertExpectations(t)
	assert.Nil(t, err)
}

func TestRemovePackage(t *testing.T) {
	version := "0.0.1"
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("RemoveAll", path.Join(testRepoRoot, testPackage, version)).Return(nil).Once()

	// Instantiate repository with mock
	repo := localRepository{filesysdep: &mockFileSys, repoRoot: testRepoRoot, lockRoot: testLockRoot, fileLocker: &filelock.FileLockerNoop{}}

	// Call and validate mock expectations and return value
	err := repo.RemovePackage(tracerMock, testPackage, version)
	mockFileSys.AssertExpectations(t)
	assert.Nil(t, err)
}

func TestSetInstallState(t *testing.T) {
	initialState := PackageInstallState{Name: testPackage, Version: "0.0.1", State: None}
	finalState := PackageInstallState{Name: testPackage, Version: "0.0.1", State: Installing, Time: time.Now()}
	testSetInstall(t, initialState, Installing, finalState)
}

func TestSetInstallStateRetry(t *testing.T) {
	initialState := PackageInstallState{Name: testPackage, Version: "0.0.1", State: Installing}
	finalState := PackageInstallState{Name: testPackage, Version: "0.0.1", State: Installing, Time: time.Now(), RetryCount: 1}
	testSetInstall(t, initialState, Installing, finalState)
}

func TestSetInstallStateInstalled(t *testing.T) {
	initialState := PackageInstallState{Name: testPackage, Version: "0.0.1", State: Installing}
	finalState := PackageInstallState{Name: testPackage, Version: "0.0.1", State: Installed, Time: time.Now(), LastInstalledVersion: "0.0.1"}
	testSetInstall(t, initialState, Installed, finalState)
}

func TestSetInstallStateUninstalled(t *testing.T) {
	initialState := PackageInstallState{Name: testPackage, Version: "0.0.1", State: Installed, Time: time.Now(), LastInstalledVersion: "0.0.1"}
	finalState := PackageInstallState{Name: testPackage, Version: "0.0.1", State: Uninstalled, Time: time.Now()}
	testSetInstall(t, initialState, Uninstalled, finalState)
}

type InventoryTestData struct {
	Name     string
	Version  string
	State    PackageInstallState
	Manifest PackageManifest
}

func TestGetInventoryData(t *testing.T) {
	installTime := time.Now()
	testData := InventoryTestData{
		Name:     "SsmTest",
		Version:  "0.0.1",
		State:    PackageInstallState{Name: "SsmTest", Version: "0.0.1", State: Installed, Time: installTime},
		Manifest: PackageManifest{Name: "SsmTest", Version: "0.0.1", Platform: "windows", Architecture: "amd64", AppPublisher: "Amazon AWS"},
	}
	expectedInventory := model.ApplicationData{
		Name:          "SsmTest",
		Version:       "0.0.1",
		Architecture:  "x86_64",
		Publisher:     "Amazon AWS",
		CompType:      model.AWSComponent,
		InstalledTime: installTime.Format(time.RFC3339),
	}

	testInventory(t, []InventoryTestData{testData}, []model.ApplicationData{expectedInventory})
}

func TestGetInventoryDataEmpty(t *testing.T) {
	testInventory(t, []InventoryTestData{}, []model.ApplicationData{})
}

func TestGetInventoryDataMultiple(t *testing.T) {
	installTime := time.Now()
	testData1 := InventoryTestData{
		Name:     "SsmTest",
		Version:  "0.0.1",
		State:    PackageInstallState{Name: "SsmTest", Version: "0.0.1", State: Installed, Time: installTime},
		Manifest: PackageManifest{Name: "SsmTest", Version: "0.0.1", Platform: "windows", Architecture: "amd64", AppPublisher: "Amazon AWS"},
	}
	testData2 := InventoryTestData{
		Name:     "Foo",
		Version:  "1.0.1",
		State:    PackageInstallState{Name: "Foo", Version: "1.0.1", State: Installed, Time: installTime},
		Manifest: PackageManifest{Name: "Foo", Version: "1.0.1", Platform: "windows", Architecture: "amd64", AppType: "Driver"},
	}
	expectedInventory1 := model.ApplicationData{
		Name:          "SsmTest",
		Version:       "0.0.1",
		Architecture:  "x86_64",
		Publisher:     "Amazon AWS",
		CompType:      model.AWSComponent,
		InstalledTime: installTime.Format(time.RFC3339),
	}
	expectedInventory2 := model.ApplicationData{
		Name:            "Foo",
		Version:         "1.0.1",
		Architecture:    "x86_64",
		CompType:        model.AWSComponent,
		ApplicationType: "Driver",
		InstalledTime:   installTime.Format(time.RFC3339),
	}

	testInventory(t, []InventoryTestData{testData1, testData2}, []model.ApplicationData{expectedInventory1, expectedInventory2})
}

func TestGetInventoryDataComplex(t *testing.T) {
	installTime := time.Now()
	testData1 := InventoryTestData{
		Name:     "SsmTest",
		Version:  "0.0.1",
		State:    PackageInstallState{Name: "SsmTest", Version: "0.0.1", State: Installed, Time: installTime},
		Manifest: PackageManifest{Name: "SsmTest", Version: "0.0.1", Platform: "windows", Architecture: "386", AppName: "SSM Test Package", AppPublisher: "Test"},
	}
	testData2 := InventoryTestData{ // no manifest defined
		Name:    "Foo",
		Version: "1.0.1",
		State:   PackageInstallState{Name: "Foo", Version: "1.0.1", State: Installing, Time: installTime},
	}
	testData3 := InventoryTestData{ // only name specified in the manifest
		Name:     "SsmTest2",
		Version:  "0.1.2",
		State:    PackageInstallState{Name: "SsmTest2", Version: "0.1.2", State: Installed, Time: installTime},
		Manifest: PackageManifest{Name: "SsmTest", Version: "0.1.2", Platform: "windows", Architecture: "386"},
	}
	testData4 := InventoryTestData{ // invalid manifest - no name or appname specified
		Name:     "SsmTest3",
		Version:  "0.1.3",
		State:    PackageInstallState{Name: "SsmTest3", Version: "0.1.3", State: Installed, Time: installTime},
		Manifest: PackageManifest{Version: "0.1.3", Platform: "windows", Architecture: "386"},
	}

	expectedInventory := []model.ApplicationData{
		{
			Name:          "SSM Test Package",
			Version:       "0.0.1",
			Architecture:  "i386",
			Publisher:     "Test",
			InstalledTime: installTime.Format(time.RFC3339),
		},
		{
			Name:          "SsmTest",
			Version:       "0.1.2",
			Architecture:  "i386",
			CompType:      model.AWSComponent,
			InstalledTime: installTime.Format(time.RFC3339),
		},
	}

	testInventory(t, []InventoryTestData{testData1, testData2, testData3, testData4}, expectedInventory)
}

func TestGetInventoryBirdwatcherPackageData(t *testing.T) {
	installTime := time.Now()
	testData := []InventoryTestData{
		{ // manifest defined with only the Name
			Name:     "_arnawsssmpackagetestbirdwatcherpackagename_30_MDYHMZE2S4YZLHSTLR4EQ6BT4ZSCW4BDXEV5C2SMMOVWDQZKUHPQ====",
			Version:  "0.0.1",
			State:    PackageInstallState{Name: "arn:aws:ssm:::package/TestBirdwatcherPackageName", Version: "0.0.1", State: Installed, Time: installTime},
			Manifest: PackageManifest{Name: "TestBirdwatcherPackageName", Version: "0.0.1", Platform: "windows", Architecture: "386", AppPublisher: "Test"},
		},
	}
	expectedInventory := model.ApplicationData{
		Name:          "TestBirdwatcherPackageName",
		Version:       "0.0.1",
		Architecture:  "i386",
		Publisher:     "Test",
		InstalledTime: installTime.Format(time.RFC3339),
	}

	testInventory(t, testData, []model.ApplicationData{expectedInventory})
}

func TestGetInventoryError(t *testing.T) {
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("GetDirectoryNames", path.Join(testRepoRoot)).Return([]string{}, errors.New("Failed")).Once()

	// Instantiate repository with mock
	repo := localRepository{filesysdep: &mockFileSys, repoRoot: testRepoRoot, lockRoot: testLockRoot}

	// Call and validate mock expectations and return value
	inventory := repo.GetInventoryData(log.NewMockLog())
	mockFileSys.AssertExpectations(t)

	assert.True(t, len(inventory) == 0)
}

func testInventory(t *testing.T, testData []InventoryTestData, expected []model.ApplicationData) {
	mockPackages := make([]string, len(testData))
	i := 0
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	for _, testItem := range testData {
		mockPackages[i] = testItem.Name
		i++
		mockFileSys.On("Exists", path.Join(testRepoRoot, testItem.Name, "installstate")).Return(true).Once()
		stateContent, _ := jsonutil.Marshal(testItem.State)
		mockFileSys.On("ReadFile", path.Join(testRepoRoot, testItem.Name, "installstate")).Return([]byte(stateContent), nil).Once()

		if (testItem.Manifest != PackageManifest{}) {
			mockFileSys.On("Exists", path.Join(testRepoRoot, normalizeDirectory(testItem.State.Name), testItem.Version, "manifest.json")).Return(true).Once()
			manifestContent, _ := jsonutil.Marshal(testItem.Manifest)
			mockFileSys.On("ReadFile", path.Join(testRepoRoot, normalizeDirectory(testItem.State.Name), testItem.Version, "manifest.json")).Return([]byte(manifestContent), nil).Once()
		}
	}
	mockFileSys.On("GetDirectoryNames", path.Join(testRepoRoot)).Return(mockPackages, nil).Once()

	// Instantiate repository with mock
	repo := localRepository{filesysdep: &mockFileSys, repoRoot: testRepoRoot, lockRoot: testLockRoot, fileLocker: &filelock.FileLockerNoop{}}

	// Call and validate mock expectations and return value
	inventory := repo.GetInventoryData(log.NewMockLog())
	mockFileSys.AssertExpectations(t)

	assert.True(t, len(inventory) == len(expected))
	for i, expectedInventory := range expected {
		assert.Equal(t, expectedInventory, inventory[i])
	}
}

func testSetInstall(t *testing.T, initialState PackageInstallState, newState InstallState, finalState PackageInstallState) {
	initialJson, _ := jsonutil.Marshal(initialState)

	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, "installstate")).Return(true).Once()
	mockFileSys.On("ReadFile", path.Join(testRepoRoot, testPackage, "installstate")).Return([]byte(initialJson), nil).Once()
	mockFileSys.On("WriteFile", path.Join(testRepoRoot, testPackage, "installstate"), mock.Anything).Return(nil).Once()

	// Instantiate repository with mock
	repo := localRepository{filesysdep: &mockFileSys, repoRoot: testRepoRoot, lockRoot: testLockRoot, fileLocker: &filelock.FileLockerNoop{}}

	// Call and validate mock expectations and return value
	err := repo.SetInstallState(tracerMock, testPackage, "0.0.1", newState)
	mockFileSys.AssertExpectations(t)
	assert.Nil(t, err)
	var expectedState PackageInstallState
	jsonutil.Unmarshal(mockFileSys.ContentWritten, &expectedState)
	assertStateEqual(t, finalState, expectedState)
}

func TestLoadTraces(t *testing.T) {
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, "traces")).Return(true)
	mockFileSys.On("ReadFile", path.Join(testRepoRoot, testPackage, "traces")).Return([]byte(
		`[{"Operation": "foo", "Exitcode": 1, "Start": 123, "Stop": 456}]`), nil)
	mockFileSys.On("RemoveAll", path.Join(testRepoRoot, testPackage, "traces")).Return(nil)

	repo := localRepository{filesysdep: &mockFileSys, repoRoot: testRepoRoot, lockRoot: testLockRoot, fileLocker: &filelock.FileLockerNoop{}}

	err := repo.LoadTraces(tracerMock, testPackage)
	assert.Nil(t, err)
	mockFileSys.AssertExpectations(t)

	assert.Equal(t, int64(123), tracerMock.Traces()[0].Start)
}

func TestLoadTracesNoneExist(t *testing.T) {
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, "traces")).Return(false)

	repo := localRepository{filesysdep: &mockFileSys, repoRoot: testRepoRoot, lockRoot: testLockRoot, fileLocker: &filelock.FileLockerNoop{}}

	err := repo.LoadTraces(tracerMock, testPackage)
	assert.Nil(t, err)
	mockFileSys.AssertExpectations(t)
}

func TestLoadTracesCorrupted(t *testing.T) {
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, "traces")).Return(true)
	mockFileSys.On("ReadFile", path.Join(testRepoRoot, testPackage, "traces")).Return([]byte("/!"), nil)
	mockFileSys.On("RemoveAll", path.Join(testRepoRoot, testPackage, "traces")).Return(nil)

	repo := localRepository{filesysdep: &mockFileSys, repoRoot: testRepoRoot, lockRoot: testLockRoot, fileLocker: &filelock.FileLockerNoop{}}

	err := repo.LoadTraces(tracerMock, testPackage)
	assert.Error(t, err)
	mockFileSys.AssertExpectations(t)
}

func TestPersistTraces(t *testing.T) {
	mockFileSys := MockedFileSys{}
	mockFileSys.On("WriteFile", path.Join(testRepoRoot, testPackage, "traces"), mock.Anything).Return(nil)

	repo := localRepository{filesysdep: &mockFileSys, repoRoot: testRepoRoot, lockRoot: testLockRoot, fileLocker: &filelock.FileLockerNoop{}}

	err := repo.PersistTraces(tracerMock, testPackage)
	assert.NoError(t, err)
	mockFileSys.AssertExpectations(t)
}

func TestPersistLoadTracesRoundtrip(t *testing.T) {
	mockFileSys := MockedFileSys{}
	repo := localRepository{filesysdep: &mockFileSys, repoRoot: testRepoRoot, lockRoot: testLockRoot, fileLocker: &filelock.FileLockerNoop{}}

	var file string
	mockFileSys.On("WriteFile", path.Join(testRepoRoot, testPackage, "traces"), mock.Anything).Return(nil).Run(func(args mock.Arguments) { file = args.Get(1).(string) })
	err := repo.PersistTraces(tracerMock, testPackage)
	assert.NoError(t, err)

	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, "traces")).Return(true)
	mockFileSys.On("ReadFile", path.Join(testRepoRoot, testPackage, "traces")).Return([]byte(file), nil)
	mockFileSys.On("RemoveAll", path.Join(testRepoRoot, testPackage, "traces")).Return(nil)
	err = repo.LoadTraces(tracerMock, testPackage)
	assert.NoError(t, err)

	mockFileSys.AssertExpectations(t)
}

// assertStateEqual compares two PackageInstallState and makes sure they are the same (ignoring the time field)
func assertStateEqual(t *testing.T, expected PackageInstallState, actual PackageInstallState) {
	assert.Equal(t, expected.Name, actual.Name)
	assert.Equal(t, expected.Version, actual.Version)
	assert.Equal(t, expected.State, actual.State)
	assert.Equal(t, expected.LastInstalledVersion, actual.LastInstalledVersion)
	assert.Equal(t, expected.RetryCount, actual.RetryCount)
	if (expected.Time != time.Time{}) {
		assert.True(t, actual.Time != time.Time{})
	} else {
		assert.True(t, actual.Time == time.Time{})
	}
}

// Load specified file from file system
func loadFile(t *testing.T, fileName string) (result []byte) {
	var err error
	if result, err = ioutil.ReadFile(fileName); err != nil {
		t.Fatal(err)
	}
	return
}

type MockedDownloader struct {
	mock.Mock
}

func (downloadMock *MockedDownloader) Download(tracer trace.Tracer, targetDirectory string) error {
	args := downloadMock.Called(tracer, targetDirectory)
	return args.Error(0)
}

type MockedFileSys struct {
	mock.Mock
	ContentWritten string
}

func (fileMock *MockedFileSys) MakeDirExecute(destinationDir string) (err error) {
	args := fileMock.Called(destinationDir)
	return args.Error(0)
}

func (fileMock *MockedFileSys) GetDirectoryNames(srcPath string) (directories []string, err error) {
	args := fileMock.Called(srcPath)
	return args.Get(0).([]string), args.Error(1)
}

func (fileMock *MockedFileSys) GetFileNames(srcPath string) (files []string, err error) {
	args := fileMock.Called(srcPath)
	return args.Get(0).([]string), args.Error(1)
}

func (fileMock *MockedFileSys) Exists(filePath string) bool {
	args := fileMock.Called(filePath)
	return args.Bool(0)
}

func (fileMock *MockedFileSys) RemoveAll(path string) error {
	args := fileMock.Called(path)
	return args.Error(0)
}

func (fileMock *MockedFileSys) ReadFile(filename string) ([]byte, error) {
	args := fileMock.Called(filename)
	return args.Get(0).([]byte), args.Error(1)
}

func (fileMock *MockedFileSys) WriteFile(filename string, content string) error {
	args := fileMock.Called(filename, content)
	fileMock.ContentWritten += content
	return args.Error(0)
}
