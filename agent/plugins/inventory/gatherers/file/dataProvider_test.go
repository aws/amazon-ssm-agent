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
	mdir := MockFileInfo{
		name:    "abc.json",
		size:    1024,
		mode:    0,
		modTime: time.Now(),
		isDir:   true,
	}

	walkFn(root, mdir, nil)
	return walkFn(root, mfi, nil)
}

func MockFilePathWalkErr(root string, walkFn filepath.WalkFunc) error {
	return FileCountLimitError
}

func MockFilePathOtherErr(root string, walkFn filepath.WalkFunc) error {
	return errors.New("Error")
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

func TestGetAllMeta(t *testing.T) {
	mockContext := context.NewMockDefault()
	mockLog := mockContext.Log()
	mockFilters := `[{"Path": "$HOME","Pattern":["*.txt"],"Recursive": false}]`
	mockConfig := model.Config{Collection: "Enabled", Filters: mockFilters, Location: ""}
	getFilesFunc = MockGetFiles
	getFullPath = MockGetFullPath
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
	existsPath = MockExistsPath
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
	existsPath = MockExistsPath
	filepathWalk = MockFilePathWalkErr
	readDirFunc = MockReadDir
	data, err := getFiles(mockLog, "mockPath", []string{"*.json"}, true, 10, 10)
	assert.NotNil(t, err)
	assert.Nil(t, data)
}
