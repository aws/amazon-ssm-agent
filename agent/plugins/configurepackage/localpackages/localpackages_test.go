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
	"fmt"
	"io/ioutil"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TODO:MF: test deps, replace filesysdep test version in test_configurepackage with usage of the mocked repository

const testRepoRoot = "testdata"
const testPackage = "SsmTest"

var contextMock context.T = context.NewMockDefault()

func TestGetInstallState(t *testing.T) {
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, "installstate")).Return(true).Once()
	mockFileSys.On("ReadFile", path.Join(testRepoRoot, testPackage, "installstate")).Return(loadFile(t, path.Join(testRepoRoot, testPackage, "installstate_success")), nil).Once()

	// Instantiate repository with mock
	repo := NewRepository(&mockFileSys, testRepoRoot)

	// Call and validate mock expectations and return value
	state, version := repo.GetInstallState(contextMock, testPackage)
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
	repo := NewRepository(&mockFileSys, testRepoRoot)

	// Call and validate mock expectations and return value
	state, version := repo.GetInstallState(contextMock, testPackage)
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
	repo := NewRepository(&mockFileSys, testRepoRoot)

	// Call and validate mock expectations and return value
	state, version := repo.GetInstallState(contextMock, testPackage)
	mockFileSys.AssertExpectations(t)
	assert.Equal(t, Unknown, state)
	assert.Equal(t, version, version)
}

func TestGetInstallStateCorrupt(t *testing.T) {
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, "installstate")).Return(true).Once()
	mockFileSys.On("ReadFile", path.Join(testRepoRoot, testPackage, "installstate")).Return(loadFile(t, path.Join(testRepoRoot, testPackage, "installstate_corrupt")), nil).Once()

	// Instantiate repository with mock
	repo := NewRepository(&mockFileSys, testRepoRoot)

	// Call and validate mock expectations and return value
	state, version := repo.GetInstallState(contextMock, testPackage)
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
	repo := NewRepository(&mockFileSys, testRepoRoot)

	// Call and validate mock expectations and return value
	state, version := repo.GetInstallState(contextMock, testPackage)
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
	repo := NewRepository(&mockFileSys, testRepoRoot)

	// Call and validate mock expectations and return value
	version := repo.GetInstalledVersion(contextMock, testPackage)
	mockFileSys.AssertExpectations(t)
	assert.Equal(t, version, version)
}

func TestGetInstalledVersionCompat(t *testing.T) {
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, "installstate")).Return(false).Once()
	mockFileSys.On("GetDirectoryNames", path.Join(testRepoRoot, testPackage)).Return([]string{"0.0.1"}, nil).Once()

	// Instantiate repository with mock
	repo := NewRepository(&mockFileSys, testRepoRoot)

	// Call and validate mock expectations and return value
	version := repo.GetInstalledVersion(contextMock, testPackage)
	mockFileSys.AssertExpectations(t)
	assert.Equal(t, version, version)
}

func TestGetInstalledVersionInstalling(t *testing.T) {
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, "installstate")).Return(true).Once()
	mockFileSys.On("ReadFile", path.Join(testRepoRoot, testPackage, "installstate")).Return(loadFile(t, path.Join(testRepoRoot, testPackage, "installstate_installing")), nil).Once()

	// Instantiate repository with mock
	repo := NewRepository(&mockFileSys, testRepoRoot)

	// Call and validate mock expectations and return value
	version := repo.GetInstalledVersion(contextMock, testPackage)
	mockFileSys.AssertExpectations(t)
	assert.Equal(t, version, version)
}

func TestValidatePackage(t *testing.T) {
	version := "0.0.1"
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, version, "SsmTest.json")).Return(true).Once()
	mockFileSys.On("ReadFile", path.Join(testRepoRoot, testPackage, version, "SsmTest.json")).Return(loadFile(t, path.Join(testRepoRoot, testPackage, version, "SsmTest.json")), nil).Once()
	mockFileSys.On("GetFileNames", path.Join(testRepoRoot, testPackage, version)).Return([]string{"SsmTest.json", "install.json"}, nil)

	// Instantiate repository with mock
	repo := NewRepository(&mockFileSys, testRepoRoot)

	// Call and validate mock expectations and return value
	err := repo.ValidatePackage(contextMock, testPackage, version)
	mockFileSys.AssertExpectations(t)
	assert.Nil(t, err)
}

func TestValidatePackageNoContent(t *testing.T) {
	version := "0.0.1"
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, version, "SsmTest.json")).Return(true).Once()
	mockFileSys.On("ReadFile", path.Join(testRepoRoot, testPackage, version, "SsmTest.json")).Return(loadFile(t, path.Join(testRepoRoot, testPackage, version, "SsmTest.json")), nil).Once()
	mockFileSys.On("GetFileNames", path.Join(testRepoRoot, testPackage, version)).Return([]string{"SsmTest.json"}, nil)
	mockFileSys.On("GetDirectoryNames", path.Join(testRepoRoot, testPackage, version)).Return([]string{}, nil)

	// Instantiate repository with mock
	repo := NewRepository(&mockFileSys, testRepoRoot)

	// Call and validate mock expectations and return value
	err := repo.ValidatePackage(contextMock, testPackage, version)
	mockFileSys.AssertExpectations(t)
	assert.NotNil(t, err)
	assert.True(t, strings.EqualFold(err.Error(), "Package manifest exists, but all other content is missing"))
}

func TestValidatePackageCorruptManifest(t *testing.T) {
	version := "0.0.10"
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, version, "SsmTest.json")).Return(true).Once()
	mockFileSys.On("ReadFile", path.Join(testRepoRoot, testPackage, version, "SsmTest.json")).Return(loadFile(t, path.Join(testRepoRoot, testPackage, version, "SsmTest.json")), nil).Once()

	// Instantiate repository with mock
	repo := NewRepository(&mockFileSys, testRepoRoot)

	// Call and validate mock expectations and return value
	err := repo.ValidatePackage(contextMock, testPackage, version)
	mockFileSys.AssertExpectations(t)
	assert.NotNil(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "Package manifest is invalid:"))
}

// TODO:MF: Unit test validatePackageManifest

func TestAddPackage(t *testing.T) {
	version := "0.0.1"
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("MakeDirExecute", path.Join(testRepoRoot, testPackage, version)).Return(nil).Once()

	mockDownload := MockedDownloader{}
	mockDownload.On("Download", path.Join(testRepoRoot, testPackage, version)).Return(nil).Once()

	// Instantiate repository with mock
	repo := NewRepository(&mockFileSys, testRepoRoot)

	// Call and validate mock expectations and return value
	err := repo.AddPackage(contextMock, testPackage, version, mockDownload.Download)
	mockFileSys.AssertExpectations(t)
	mockDownload.AssertExpectations(t)
	assert.Nil(t, err)
}

func TestRefreshPackage(t *testing.T) {
	version := "0.0.1"
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("MakeDirExecute", path.Join(testRepoRoot, testPackage, version)).Return(nil).Once()

	mockDownload := MockedDownloader{}
	mockDownload.On("Download", path.Join(testRepoRoot, testPackage, version)).Return(nil).Once()

	// Instantiate repository with mock
	repo := NewRepository(&mockFileSys, testRepoRoot)

	// Call and validate mock expectations and return value
	err := repo.RefreshPackage(contextMock, testPackage, version, mockDownload.Download)
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
	repo := NewRepository(&mockFileSys, testRepoRoot)

	// Call and validate mock expectations and return value
	err := repo.RemovePackage(contextMock, testPackage, version)
	mockFileSys.AssertExpectations(t)
	assert.Nil(t, err)
}

func TestGetAction(t *testing.T) {
	version := "0.0.1"
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, version, "Foo.json")).Return(true).Once()
	mockFileSys.On("ReadFile", path.Join(testRepoRoot, testPackage, version, "Foo.json")).Return(loadFile(t, path.Join(testRepoRoot, testPackage, version, "install.json")), nil).Once()

	// Instantiate repository with mock
	repo := NewRepository(&mockFileSys, testRepoRoot)

	// Call and validate mock expectations and return value
	exists, actionDoc, err := repo.GetAction(contextMock, testPackage, version, "Foo")
	mockFileSys.AssertExpectations(t)
	assert.True(t, exists)
	assert.NotEmpty(t, actionDoc)
	assert.Nil(t, err)
}

func TestGetActionInvalid(t *testing.T) {
	version := "0.0.11"
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, version, "Foo.json")).Return(true).Once()
	mockFileSys.On("ReadFile", path.Join(testRepoRoot, testPackage, version, "Foo.json")).Return(loadFile(t, path.Join(testRepoRoot, testPackage, version, "install.json")), nil).Once()

	// Instantiate repository with mock
	repo := NewRepository(&mockFileSys, testRepoRoot)

	// Call and validate mock expectations and return value
	exists, actionDoc, err := repo.GetAction(contextMock, testPackage, version, "Foo")
	mockFileSys.AssertExpectations(t)
	assert.True(t, exists)
	assert.Empty(t, actionDoc)
	assert.NotNil(t, err)
}

func TestGetActionMissing(t *testing.T) {
	version := "0.0.1"
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testRepoRoot, testPackage, version, "Foo.json")).Return(false).Once()

	// Instantiate repository with mock
	repo := NewRepository(&mockFileSys, testRepoRoot)

	// Call and validate mock expectations and return value
	exists, actionDoc, err := repo.GetAction(contextMock, testPackage, version, "Foo")
	mockFileSys.AssertExpectations(t)
	assert.False(t, exists)
	assert.Empty(t, actionDoc)
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
		Manifest: PackageManifest{Name: "SsmTest", Version: "0.0.1", Platform: "windows", Architecture: "amd64"},
	}
	expectedInventory := model.ApplicationData{
		Name:          "SsmTest",
		Version:       "0.0.1",
		Architecture:  "x86_64",
		CompType:      model.AWSComponent,
		InstalledTime: installTime.Format(time.RFC3339),
	}

	testInventory(t, map[string]InventoryTestData{"SsmTest": testData}, []model.ApplicationData{expectedInventory})
}

func TestGetInventoryDataMultiple(t *testing.T) {
	installTime := time.Now()
	testData1 := InventoryTestData{
		Name:     "SsmTest",
		Version:  "0.0.1",
		State:    PackageInstallState{Name: "SsmTest", Version: "0.0.1", State: Installed, Time: installTime},
		Manifest: PackageManifest{Name: "SsmTest", Version: "0.0.1", Platform: "windows", Architecture: "amd64"},
	}
	testData2 := InventoryTestData{
		Name:     "Foo",
		Version:  "1.0.1",
		State:    PackageInstallState{Name: "Foo", Version: "1.0.1", State: Installed, Time: installTime},
		Manifest: PackageManifest{Name: "Foo", Version: "1.0.1", Platform: "windows", Architecture: "amd64"},
	}
	expectedInventory1 := model.ApplicationData{
		Name:          "SsmTest",
		Version:       "0.0.1",
		Architecture:  "x86_64",
		CompType:      model.AWSComponent,
		InstalledTime: installTime.Format(time.RFC3339),
	}
	expectedInventory2 := model.ApplicationData{
		Name:          "Foo",
		Version:       "1.0.1",
		Architecture:  "x86_64",
		CompType:      model.AWSComponent,
		InstalledTime: installTime.Format(time.RFC3339),
	}

	testInventory(t, map[string]InventoryTestData{"SsmTest": testData1, "Foo": testData2}, []model.ApplicationData{expectedInventory1, expectedInventory2})
}

func TestGetInventoryDataComplex(t *testing.T) {
	installTime := time.Now()
	testData1 := InventoryTestData{
		Name:     "SsmTest",
		Version:  "0.0.1",
		State:    PackageInstallState{Name: "SsmTest", Version: "0.0.1", State: Installed, Time: installTime},
		Manifest: PackageManifest{Name: "SsmTest", Version: "0.0.1", Platform: "windows", Architecture: "386", AppName: "SSM Test Package", AppPublisher: "Test"},
	}
	testData2 := InventoryTestData{
		Name:    "Foo",
		Version: "1.0.1",
		State:   PackageInstallState{Name: "Foo", Version: "1.0.1", State: Installing, Time: installTime},
	}
	expectedInventory := model.ApplicationData{
		Name:          "SSM Test Package",
		Version:       "0.0.1",
		Architecture:  "i386",
		Publisher:     "Test",
		InstalledTime: installTime.Format(time.RFC3339),
	}

	testInventory(t, map[string]InventoryTestData{"SsmTest": testData1, "Foo": testData2}, []model.ApplicationData{expectedInventory})
}

func testInventory(t *testing.T, testData map[string]InventoryTestData, expected []model.ApplicationData) {
	mockPackages := make([]string, len(testData))
	i := 0
	for packageName, _ := range testData {
		mockPackages[i] = packageName
		i++
	}
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("GetDirectoryNames", path.Join(testRepoRoot)).Return(mockPackages, nil).Once()
	for packageName, testItem := range testData {
		mockFileSys.On("Exists", path.Join(testRepoRoot, packageName, "installstate")).Return(true).Once()
		stateContent, _ := jsonutil.Marshal(testItem.State)
		mockFileSys.On("ReadFile", path.Join(testRepoRoot, packageName, "installstate")).Return([]byte(stateContent), nil).Once()

		if (testItem.Manifest != PackageManifest{}) {
			mockFileSys.On("Exists", path.Join(testRepoRoot, packageName, testItem.Version, fmt.Sprintf("%v.json", packageName))).Return(true).Once()
			manifestContent, _ := jsonutil.Marshal(testItem.Manifest)
			mockFileSys.On("ReadFile", path.Join(testRepoRoot, packageName, testItem.Version, fmt.Sprintf("%v.json", packageName))).Return([]byte(manifestContent), nil).Once()
		}
	}

	// Instantiate repository with mock
	repo := NewRepository(&mockFileSys, testRepoRoot)

	// Call and validate mock expectations and return value
	inventory := repo.GetInventoryData(contextMock)
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
	repo := NewRepository(&mockFileSys, testRepoRoot)

	// Call and validate mock expectations and return value
	err := repo.SetInstallState(contextMock, testPackage, "0.0.1", newState)
	mockFileSys.AssertExpectations(t)
	assert.Nil(t, err)
	var expectedState PackageInstallState
	jsonutil.Unmarshal(mockFileSys.ContentWritten, &expectedState)
	assertStateEqual(t, finalState, expectedState)
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

func (downloadMock *MockedDownloader) Download(targetDirectory string) error {
	args := downloadMock.Called(targetDirectory)
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
