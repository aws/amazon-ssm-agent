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

package fsvault

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	key           = "some-key"
	data          = []byte("some-data")
	storePath     = filepath.Join(storeFolderPath, key)
	oriEnsureInit = ensureInitialized
	oriSaveMf     = saveManifest
)

func reset() {
	initialized = false
	manifest = make(map[string]string)
	fs = &fsvFileSystem{}
	jh = &fsvJsonHandler{}
	ensureInitialized = oriEnsureInit
	saveManifest = oriSaveMf
}

func TestSuite(t *testing.T) {

	// ensureInitialized
	ensureInitErrorMkdir(t)
	ensureInitErrorHarden(t)
	ensureInitWithManifestFile(t)
	ensureInitErrorReadManifestFile(t)
	ensureInitErrorUnMarshalManifestFile(t)
	ensureInitWithoutManifestFile(t)

	// saveManifest
	saveManifestErrorMarshalTest(t)
	saveManifestErrorWriteFileTest(t)

	// main test cases
	store(t)
	storeErrorEnsureInitTest(t)
	storeErrorStoreDataTest(t)
	storeErrorSaveManifestTest(t)
	retrieve(t)
	retrieveErrorNotExists(t)
	retrieveErrorEnsureInitTest(t)
	retrieveErrorFileMissingTest(t)
	retrieveErrorReadDataTest(t)
	remove(t)
	removeNotExists(t)
	removeErrorEnsureInitTest(t)
	removeErrorSaveManifestTest(t)
	removeErrorRemoveDataTest(t)
}

func TestIsManifestExists(t *testing.T) {
	testCases := []struct {
		testName                  string
		manifestExistsOnFile      bool
		manifestFileNamePrefix    string
		initializedManifestPrefix string
		manifestInMemory          map[string]string
		expectedResult            bool
	}{
		{
			testName:                  "TestManifestExists_WhenAlternateManifestCached_AndManifestNotOnDisk_ReturnsFalse",
			manifestExistsOnFile:      false,
			manifestFileNamePrefix:    "EC2",
			initializedManifestPrefix: "",
			manifestInMemory:          map[string]string{"InstanceId": "i-example123"},
			expectedResult:            false,
		},
		{
			testName:                  "TestManifestExists_WhenAlternateManifestCached_AndManifestIsOnDisk_ReturnsTrue",
			manifestExistsOnFile:      true,
			manifestFileNamePrefix:    "EC2",
			initializedManifestPrefix: "",
			manifestInMemory:          map[string]string{},
			expectedResult:            true,
		},
		{
			testName:                  "TestManifestExists_WhenSameManifestIsCached_AndManifestNotOnDisk_ReturnsTrue",
			manifestExistsOnFile:      false,
			manifestFileNamePrefix:    "",
			initializedManifestPrefix: "",
			manifestInMemory:          map[string]string{"InstanceId": "i-example123"},
			expectedResult:            true,
		},
		{
			testName:                  "TestManifestExists_WhenSameManifestIsCached_AndManifestOnDisk_ReturnsTrue",
			manifestExistsOnFile:      true,
			manifestFileNamePrefix:    "",
			initializedManifestPrefix: "",
			manifestInMemory:          map[string]string{"InstanceId": "i-example123"},
			expectedResult:            true,
		},
		{
			testName:                  "TestManifestExists_WhenNoManifestContentInMemory_AndNoManifestOnDisk_ReturnsFalse",
			manifestExistsOnFile:      false,
			manifestFileNamePrefix:    "EC2",
			initializedManifestPrefix: "EC2",
			manifestInMemory:          map[string]string{},
			expectedResult:            false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.testName, func(t *testing.T) {
			// Arrange
			initializedManifestPrefix = testCase.initializedManifestPrefix
			manifest = testCase.manifestInMemory
			manifestPath := getManifestPath(testCase.manifestFileNamePrefix)
			fsMock := &fsvFileSystemMock{}
			fsMock.On("Exists", manifestPath).Return(testCase.manifestExistsOnFile).Repeatability = 0
			fs = fsMock

			// Act
			result := IsManifestExists(testCase.manifestFileNamePrefix)

			// Assert
			assert.Equal(t, testCase.expectedResult, result)
		})
	}
}

func storeErrorEnsureInitTest(t *testing.T) {
	// arrange
	ensureInitialized = func(manifestFileNamePrefix string) error { return errors.New("err") }

	// act
	err := Store("", key, data)

	// assert
	assert.Error(t, err)

	// clean up
	reset()
}

func store(t *testing.T) {
	// arrange
	initialized = true // skip initialization

	fsMock := &fsvFileSystemMock{}
	fsMock.On("HardenedWriteFile", storePath, data).Return(nil)
	fs = fsMock

	smCalled := false
	saveManifest = func(manifestFileNamePrefix string) error {
		smCalled = true
		return nil
	}

	// act
	err := Store("", key, data)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, storePath, manifest[key])
	assert.True(t, smCalled)
	fsMock.AssertExpectations(t)

	// clean up
	reset()
}

func storeErrorStoreDataTest(t *testing.T) {
	// arrange
	initialized = true // skip initialization

	fsMock := &fsvFileSystemMock{}
	fsMock.On("HardenedWriteFile", storePath, data).Return(errors.New("err"))
	fs = fsMock

	// act
	err := Store("", key, data)

	// assert
	assert.Error(t, err)
	assert.Empty(t, manifest[key])
	fsMock.AssertExpectations(t)

	// clean up
	reset()
}

func storeErrorSaveManifestTest(t *testing.T) {
	// arrange
	initialized = true // skip initialization

	fsMock := &fsvFileSystemMock{}
	fsMock.On("HardenedWriteFile", storePath, data).Return(nil)
	fs = fsMock

	saveManifest = func(manifestFileNamePrefix string) error { return errors.New("err") }

	// act
	err := Store("", key, data)

	// assert
	assert.Error(t, err)
	assert.Empty(t, manifest[key])
	fsMock.AssertExpectations(t)

	// clean up
	manifest = make(map[string]string)
}

func retrieve(t *testing.T) {
	// arrange
	initialized = true // skip initialization

	manifest = map[string]string{key: storePath}

	fsMock := &fsvFileSystemMock{}
	fsMock.On("Exists", storePath).Return(true)
	fsMock.On("ReadFile", storePath).Return(data, nil)
	fs = fsMock

	// act
	d, err := Retrieve("", key)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, data, d)
	fsMock.AssertExpectations(t)

	// clean up
	reset()
}

func retrieveErrorNotExists(t *testing.T) {
	// arrange
	initialized = true // skip initialization
	manifest = map[string]string{}

	// act
	d, err := Retrieve("", key)

	// assert
	assert.Error(t, err)
	assert.Nil(t, d)

	// clean up
	reset()
}

func retrieveErrorEnsureInitTest(t *testing.T) {
	// arrange
	ensureInitialized = func(manifestFileNamePrefix string) error { return errors.New("err") }

	// act
	_, err := Retrieve("", key)

	// assert
	assert.Error(t, err)

	// clean up
	reset()
}

func retrieveErrorFileMissingTest(t *testing.T) {
	// arrange
	initialized = true // skip initialization

	manifest[key] = storePath
	fsMock := &fsvFileSystemMock{}
	fsMock.On("Exists", storePath).Return(false)
	fs = fsMock

	// act
	_, err := Retrieve("", key)

	// assert
	assert.Error(t, err)
	fsMock.AssertExpectations(t)

	// clean up
	reset()
}

func retrieveErrorReadDataTest(t *testing.T) {
	// arrange
	initialized = true // skip initialization

	manifest[key] = storePath
	fsMock := &fsvFileSystemMock{}
	fsMock.On("Exists", storePath).Return(true)
	fsMock.On("ReadFile", storePath).Return([]byte(""), errors.New("err"))
	fs = fsMock

	// act
	_, err := Retrieve("", key)

	// assert
	assert.Error(t, err)
	fsMock.AssertExpectations(t)

	// clean up
	reset()
}

func remove(t *testing.T) {
	// arrange
	initialized = true // skip initialization
	manifest = map[string]string{key: storePath}

	smCalled := false
	saveManifest = func(manifestFileNamePrefix string) error {
		smCalled = true
		return nil
	}

	fsMock := &fsvFileSystemMock{}
	fsMock.On("Remove", storePath).Return(nil)
	fs = fsMock

	// act
	err := Remove("", key)

	// assert
	assert.NoError(t, err)
	_, ok := manifest[key]
	assert.False(t, ok)
	assert.True(t, smCalled)
	fsMock.AssertExpectations(t)

	// clean up
	reset()
}

func removeNotExists(t *testing.T) {
	// arrange
	initialized = true // skip initialization
	manifest = map[string]string{}

	// act
	err := Remove("", key)

	// assert
	assert.NoError(t, err)

	// clean up
	reset()
}

func removeErrorEnsureInitTest(t *testing.T) {
	// arrange
	ensureInitialized = func(manifestFileNamePrefix string) error { return errors.New("err") }

	// act
	err := Remove("", key)

	// assert
	assert.Error(t, err)

	// clean up
	reset()
}

func removeErrorRemoveDataTest(t *testing.T) {
	// arrange
	initialized = true // skip initialization
	manifest = map[string]string{key: storePath}

	smCalled := false
	saveManifest = func(manifestFileNamePrefix string) error {
		smCalled = true
		return nil
	}

	fsMock := &fsvFileSystemMock{}
	fsMock.On("Remove", storePath).Return(errors.New("err"))
	fs = fsMock

	// act
	err := Remove("", key)

	// assert
	assert.Error(t, err)
	_, ok := manifest[key]
	assert.False(t, ok)
	assert.True(t, smCalled)
	fsMock.AssertExpectations(t)

	// clean up
	reset()
}

func removeErrorSaveManifestTest(t *testing.T) {
	// arrange
	initialized = true // skip initialization
	manifest = map[string]string{key: storePath}
	saveManifest = func(manifestFileNamePrefix string) error { return errors.New("err") }

	// act
	err := Remove("", key)

	// assert
	assert.Error(t, err)
	assert.Equal(t, storePath, manifest[key])

	// clean up
	reset()
}

func saveManifestErrorMarshalTest(t *testing.T) {
	// arrange
	jhMock := &fsvJsonHandlerMock{}
	jhMock.On("Marshal", manifest).Return([]byte(""), errors.New("err"))
	jh = jhMock

	// act
	err := saveManifest("")

	// assert
	assert.Error(t, err)
	jhMock.AssertExpectations(t)

	// clean up
	reset()
}

func saveManifestErrorWriteFileTest(t *testing.T) {
	// arrange
	mData, _ := json.Marshal(manifest)
	fsMock := &fsvFileSystemMock{}
	manifestFilePath := getManifestPath("")
	fsMock.On("HardenedWriteFile", manifestFilePath, mData).Return(errors.New("err"))
	fs = fsMock

	// act
	err := saveManifest("")

	// assert
	assert.Error(t, err)
	fsMock.AssertExpectations(t)

	// clean up
	reset()
}

func ensureInitErrorMkdir(t *testing.T) {
	// arrange
	fsMock := &fsvFileSystemMock{}
	fsMock.On("MakeDirs", storeFolderPath).Return(errors.New("err"))
	fs = fsMock

	// act
	err := ensureInitialized("")

	// assert
	assert.Error(t, err)
	assert.False(t, initialized)
	fsMock.AssertExpectations(t)

	// clean up
	reset()
}

func ensureInitErrorHarden(t *testing.T) {
	// arrange
	fsMock := &fsvFileSystemMock{}
	fsMock.On("MakeDirs", storeFolderPath).Return(nil)
	fsMock.On("RecursivelyHarden", vaultFolderPath).Return(errors.New("err"))
	fs = fsMock

	// act
	err := ensureInitialized("")

	// assert
	assert.Error(t, err)
	assert.False(t, initialized)
	fsMock.AssertExpectations(t)

	// clean up
	reset()
}

func ensureInitWithManifestFile(t *testing.T) {
	// arrange
	manifestFilePath := getManifestPath("")
	m := map[string]string{key: filepath.Join(storeFolderPath, key)}
	mData, _ := json.Marshal(m)
	fsMock := &fsvFileSystemMock{}
	fsMock.On("MakeDirs", storeFolderPath).Return(nil)
	fsMock.On("RecursivelyHarden", vaultFolderPath).Return(nil)
	fsMock.On("Exists", manifestFilePath).Return(true)
	fsMock.On("ReadFile", manifestFilePath).Return(mData, nil)
	fs = fsMock

	// act
	err := ensureInitialized("")

	// assert
	assert.NoError(t, err)
	assert.Equal(t, m, manifest)
	assert.True(t, initialized)
	fsMock.AssertExpectations(t)

	// clean up
	reset()
}

func ensureInitErrorReadManifestFile(t *testing.T) {
	// arrange
	manifestFilePath := getManifestPath("")
	fsMock := &fsvFileSystemMock{}
	fsMock.On("MakeDirs", storeFolderPath).Return(nil)
	fsMock.On("RecursivelyHarden", vaultFolderPath).Return(nil)
	fsMock.On("Exists", manifestFilePath).Return(true)
	fsMock.On("ReadFile", manifestFilePath).Return([]byte(""), errors.New("err"))
	fs = fsMock

	// act
	err := ensureInitialized("")

	// assert
	assert.Error(t, err)
	assert.False(t, initialized)
	fsMock.AssertExpectations(t)

	// clean up
	reset()
}

func ensureInitErrorUnMarshalManifestFile(t *testing.T) {
	// arrange
	manifestFilePath := getManifestPath("")
	mData, _ := json.Marshal(map[string]string{key: filepath.Join(storeFolderPath, key)})
	fsMock := &fsvFileSystemMock{}
	fsMock.On("MakeDirs", storeFolderPath).Return(nil)
	fsMock.On("RecursivelyHarden", vaultFolderPath).Return(nil)
	fsMock.On("Exists", manifestFilePath).Return(true)
	fsMock.On("ReadFile", manifestFilePath).Return(mData, nil)
	fs = fsMock

	jhMock := &fsvJsonHandlerMock{}
	jhMock.On("Unmarshal", mData, &manifest).Return(errors.New("err"))
	jh = jhMock

	// act
	err := ensureInitialized("")

	// assert
	assert.Error(t, err)
	assert.False(t, initialized)
	fsMock.AssertExpectations(t)

	// clean up
	reset()
}

func ensureInitWithoutManifestFile(t *testing.T) {
	// arrange
	manifestFilePath := getManifestPath("")
	m := map[string]string{}
	fsMock := &fsvFileSystemMock{}
	fsMock.On("MakeDirs", storeFolderPath).Return(nil)
	fsMock.On("RecursivelyHarden", vaultFolderPath).Return(nil)
	fsMock.On("Exists", manifestFilePath).Return(false)
	fs = fsMock

	// act
	err := ensureInitialized("")

	// assert
	assert.NoError(t, err)
	assert.Equal(t, m, manifest)
	assert.True(t, initialized)
	fsMock.AssertExpectations(t)

	// clean up
	reset()
}
