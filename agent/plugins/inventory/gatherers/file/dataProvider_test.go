package file

import (
	"errors"
	"testing"

	"fmt"

	"time"

	"path/filepath"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/stretchr/testify/assert"
)

func MockFilePathWalk(root string, walkFn filepath.WalkFunc) error {
	mfi := MockFileInfo{
		name:    "abc.json",
		size:    1024,
		mode:    0,
		modTime: time.Now(),
		isDir:   false,
	}
	mfi2 := MockFileInfo{
		name:    "abc2.json",
		size:    1024,
		mode:    0,
		modTime: time.Now(),
		isDir:   false,
	}
	mdir := MockFileInfo{
		name:    "abc",
		size:    1024,
		mode:    0,
		modTime: time.Now(),
		isDir:   true,
	}
	mdir2 := MockFileInfo{
		name:    "abc2",
		size:    1024,
		mode:    0,
		modTime: time.Now(),
		isDir:   true,
	}

	walkFn("abc", mdir, nil)
	walkFn("abc2.json", mfi2, nil)
	walkFn("abc2", mdir2, nil)
	return walkFn("abc.json", mfi, nil)
}

func MockFilePathWalkErrFileLimit(root string, walkFn filepath.WalkFunc) error {
	return FileCountLimitError
}

func MockFilePathWalkErrDirLimit(root string, walkFn filepath.WalkFunc) error {
	return DirScanLimitError
}

func MockFilePathOtherErr(root string, walkFn filepath.WalkFunc) error {
	return errors.New("Error")
}

func createMockExists(exists []bool, err []error) func(string) (bool, error) {
	var index = 0
	return func(string) (bool, error) {
		if index < len(exists) {
			index += 1
		}
		return exists[index-1], err[index-1]
	}
}

func MockGetFiles(log log.T, path string, pattern []string, recursive bool, fileLimit int, dirLimit int) (data []string, err error) {
	MockFileData := []string{
		"abc.json",
	}
	return MockFileData, nil
}

func MockGetFilesErr(log log.T, path string, pattern []string, recursive bool, fileLimit int, dirLimit int) (data []string, err error) {
	MockFileData := []string{
		"abc.json",
	}
	return MockFileData, errors.New("error")
}

func MockGetMetaData(log log.T, paths []string) (fileInfo []model.FileData, err error) {
	MockFileData := []model.FileData{
		{
			Name:             "abc.json",
			Size:             "12",
			Description:      "mock file",
			FileVersion:      "",
			ProductVersion:   "",
			ProductName:      "",
			ProductLanguage:  "",
			CompanyName:      "",
			InstalledDate:    "",
			ModificationTime: "",
			LastAccessTime:   "",
			InstalledDir:     "",
		},
	}
	return MockFileData, nil
}

func MockGetFullPath(path string, mapping func(string) string) (string, error) {
	return "", nil
}

func createMockFullPath(paths []string, errors []error) func(string, func(string) string) (string, error) {
	var index = 0
	return func(string, func(string) string) (string, error) {
		if index < len(paths) {
			index += 1
		}
		return paths[index-1], errors[index-1]
	}
}

func TestGetAllMeta(t *testing.T) {
	mockContext := context.NewMockDefault()
	mockLog := mockContext.Log()
	mockFilters := `[{"Path": "$HOME","Pattern":["*.txt"],"Recursive": false}, {"Path": "$HOME","Pattern":["*.txt"],"Recursive": false, "DirScanLimit": 4000}]`
	mockConfig := model.Config{Collection: "Enabled", Filters: mockFilters, Location: ""}
	getFilesFunc = MockGetFiles
	getFullPath = createMockFullPath([]string{"a1", ""}, []error{nil, errors.New("error")})
	getMetaDataFunc = MockGetMetaData
	data, err := getAllMeta(mockLog, mockConfig)
	assert.Nil(t, err, "err not nil")
	fmt.Println(data)
	assert.NotNil(t, data, "data is Nil")
}

func TestGetAllMetaOtherError(t *testing.T) {
	mockContext := context.NewMockDefault()
	mockLog := mockContext.Log()
	mockFilters := `[{"Path": "$HOME","Pattern":["*.txt"],"Recursive": false}]`
	mockConfig := model.Config{Collection: "Enabled", Filters: mockFilters, Location: ""}
	getFilesFunc = MockGetFilesErr
	getFullPath = MockGetFullPath
	getMetaDataFunc = MockGetMetaData
	data, err := getAllMeta(mockLog, mockConfig)
	assert.Nil(t, err, "err not nil")
	assert.NotNil(t, data, "data is Nil")
}

func TestGetAllMetaFilterErr(t *testing.T) {
	mockContext := context.NewMockDefault()
	mockLog := mockContext.Log()
	mockFilters := `Invalid`
	mockConfig := model.Config{Collection: "Enabled", Filters: mockFilters, Location: ""}
	getFilesFunc = MockGetFilesErr
	getFullPath = MockGetFullPath
	getMetaDataFunc = MockGetMetaData
	data, err := getAllMeta(mockLog, mockConfig)
	assert.NotNil(t, err, "err not nil")
	assert.Nil(t, data, "data is Nil")
}

func TestRemoveDuplicates(t *testing.T) {
	MockFileData := []model.FileData{
		{
			Name:             "abc.json",
			Size:             "12",
			Description:      "mock file",
			FileVersion:      "",
			ProductVersion:   "",
			ProductName:      "",
			ProductLanguage:  "",
			CompanyName:      "",
			InstalledDate:    "",
			ModificationTime: "",
			LastAccessTime:   "",
			InstalledDir:     "",
		},
		{
			Name:             "abc.json",
			Size:             "12",
			Description:      "mock file",
			FileVersion:      "",
			ProductVersion:   "",
			ProductName:      "",
			ProductLanguage:  "",
			CompanyName:      "",
			InstalledDate:    "",
			ModificationTime: "",
			LastAccessTime:   "",
			InstalledDir:     "",
		},
	}
	data := removeDuplicatesFileData(MockFileData)
	fmt.Println(data)
	assert.Equal(t, len(data), 1, "data should be deduplicated")
}

func TestGetFiles(t *testing.T) {
	mockContext := context.NewMockDefault()
	mockLog := mockContext.Log()
	existsPath = createMockExists([]bool{true, true}, []error{nil, nil})
	filepathWalk = MockFilePathWalk
	readDirFunc = MockReadDir
	data, err := getFiles(mockLog, "mockPath", []string{"*.json"}, true, 10, 10)
	assert.Nil(t, err, "err not nil")
	fmt.Println(data)
	assert.NotNil(t, data, "data is Nil")
	data, err = getFiles(mockLog, "mockPath", []string{"*.json"}, false, 10, 10)
	assert.Nil(t, err, "err not nil")
	fmt.Println(data)
	assert.NotNil(t, data, "data is Nil")
}

func TestGetFilesLimitError(t *testing.T) {
	mockContext := context.NewMockDefault()
	mockLog := mockContext.Log()
	mockFilters := `[{"Path": "$HOME","Pattern":["*.txt"],"Recursive": true}, {"Path": "$HOME","Pattern":["*.txt"],"Recursive": false, "DirScanLimit": 4000}]`
	mockConfig := model.Config{Collection: "Enabled", Filters: mockFilters, Location: ""}
	existsPath = MockExistsPath
	filepathWalk = MockFilePathWalkErrFileLimit
	readDirFunc = MockReadDir
	getFullPath = createMockFullPath([]string{"a1", ""}, []error{nil, errors.New("error occured")})
	getMetaDataFunc = MockGetMetaData
	getFilesFunc = getFiles
	data, err := getAllMeta(mockLog, mockConfig)
	assert.NotNil(t, err)
	assert.Nil(t, data)
}

func TestGetFilesLimitErrorWalkFn(t *testing.T) {
	mockContext := context.NewMockDefault()
	mockLog := mockContext.Log()
	existsPath = createMockExists([]bool{true, true}, []error{nil, nil})
	filepathWalk = MockFilePathWalk
	readDirFunc = MockReadDir
	data, err := getFiles(mockLog, "mockPath", []string{"*.json"}, true, 1, 10)
	assert.NotNil(t, err)
	assert.NotNil(t, data)
}

func TestGetFilesLimitErrorNonRecursive(t *testing.T) {
	mockContext := context.NewMockDefault()
	mockLog := mockContext.Log()
	existsPath = createMockExists([]bool{true, true}, []error{nil, nil})
	filepathWalk = MockFilePathWalk
	readDirFunc = MockReadDir
	data, err := getFiles(mockLog, "mockPath", []string{"*.json"}, true, 1, 10)
	assert.NotNil(t, err)
	assert.NotNil(t, data)
}

func TestGetFilesDirLimitError(t *testing.T) {
	mockContext := context.NewMockDefault()
	mockLog := mockContext.Log()
	mockFilters := `[{"Path": "$HOME","Pattern":["*.txt"],"Recursive": true}, {"Path": "$HOME","Pattern":["*.txt"],"Recursive": false, "DirScanLimit": 4000}]`
	mockConfig := model.Config{Collection: "Enabled", Filters: mockFilters, Location: ""}
	existsPath = MockExistsPath
	filepathWalk = MockFilePathWalkErrDirLimit
	readDirFunc = MockReadDir
	getFullPath = createMockFullPath([]string{"a1", ""}, []error{nil, errors.New("error")})
	getMetaDataFunc = MockGetMetaData
	getFilesFunc = getFiles
	data, err := getAllMeta(mockLog, mockConfig)
	assert.NotNil(t, err)
	assert.Nil(t, data)
}

func TestGetFilesDirLimitErrorWalkFn(t *testing.T) {
	mockContext := context.NewMockDefault()
	mockLog := mockContext.Log()
	existsPath = createMockExists([]bool{true, true}, []error{nil, nil})
	filepathWalk = MockFilePathWalk
	readDirFunc = MockReadDir
	data, err := getFiles(mockLog, "mockPath", []string{"*.json"}, false, 1, 10)
	assert.NotNil(t, err)
	assert.NotNil(t, data)
}

func TestGetFilesPathExists(t *testing.T) {
	mockContext := context.NewMockDefault()
	mockLog := mockContext.Log()
	existsPath = createMockExists([]bool{true, false, false}, []error{nil, nil, errors.New("error")})
	filepathWalk = MockFilePathWalk
	readDirFunc = MockReadDir
	data, err := getFiles(mockLog, "mockPath", []string{"*.json"}, true, 10, 10)
	assert.Nil(t, err, "err not nil")
	fmt.Println(data)
	assert.NotNil(t, data, "data is Nil")
	data, err = getFiles(mockLog, "mockPath", []string{"*.json"}, true, 10, 10)
	assert.Nil(t, err, "err not nil")
	fmt.Println(data)
	assert.Nil(t, data, "data is not Nil")
	data, err = getFiles(mockLog, "mockPath", []string{"*.json"}, true, 10, 10)
	assert.NotNil(t, err, "err is nil")
	fmt.Println(data)
	assert.Nil(t, data, "data is not Nil")
}
