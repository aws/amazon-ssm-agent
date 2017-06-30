package file

import (
	"fmt"
	"io/ioutil"
	"os"

	"strings"

	"encoding/json"

	"path/filepath"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
)

type filterObj struct {
	Path      string
	Pattern   []string
	Recursive bool
}

type fileInfoObject struct {
	log  log.T
	fi   os.FileInfo
	path string
}

const FileCountLimit = 1000
const FileCountLimitExceeded = "File Count Limit Exceeded"

//decoupling for easy testability
var readDirFunc = ReadDir
var existsPath = exists
var getFullPath func(path string, mapping func(string) string) (string, error)
var filepathWalk = filepath.Walk
var getFilesFunc = getFiles
var getMetaDataFunc = getMetaData

// ReadDir is a wrapper on ioutil.ReadDir for easy testability
func ReadDir(dirname string) ([]os.FileInfo, error) {
	return ioutil.ReadDir(dirname)
}

//removeDuplicates deduplicates the input array of model.FileData
func removeDuplicatesFileData(elements []model.FileData) (result []model.FileData) {
	// Use map to record duplicates as we find them.
	encountered := map[model.FileData]bool{}
	for v := range elements {
		if !encountered[elements[v]] {
			// Record this element as an encountered element.
			encountered[elements[v]] = true
			// Append to result slice.
			result = append(result, elements[v])
		}
	}
	// Return the new slice.
	return result
}

//removeDuplicatesString deduplicates array of strings
func removeDuplicatesString(elements []string) (result []string) {
	encountered := map[string]bool{}
	for v := range elements {
		if !encountered[elements[v]] {
			encountered[elements[v]] = true
			result = append(result, elements[v])
		}
	}
	return result
}

// LogError is a wrapper on log.Error for easy testability
func LogError(log log.T, err error) {
	// To debug unit test, please uncomment following line
	// fmt.Println(err)
	log.Error(err)
}

//exists check if the file path exists
func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func getFiles(log log.T, path string, pattern []string, recursive bool) (data []model.FileData, err error) {
	var ex bool
	ex, err = existsPath(path)
	var validFiles []string
	if err != nil {
		LogError(log, err)
		return
	}
	if !ex {
		LogError(log, fmt.Errorf("Path does not exist!"))
		return
	}
	if recursive {
		filepathWalk(path, func(fp string, fi os.FileInfo, err error) error {
			if err != nil {
				LogError(log, err)
				return nil
			}
			if fi.IsDir() {
				return nil

			}
			if fileMatchesAnyPattern(log, pattern, fi.Name()) {
				validFiles = append(validFiles, fp)
			}
			return nil
		})
	} else {
		files, readDirErr := readDirFunc(path)
		if readDirErr != nil {
			LogError(log, readDirErr)
			return
		}
		for _, fi := range files {
			if fi.IsDir() {
				continue
			}
			if fileMatchesAnyPattern(log, pattern, fi.Name()) {
				validFiles = append(validFiles, filepath.Join(path, fi.Name()))
			}
		}

	}
	validFiles = removeDuplicatesString(validFiles)
	log.Debugf("len validFiles %v", len(validFiles))

	if len(validFiles) > FileCountLimit {
		err = fmt.Errorf("File count limit exceeded. Max Allowed - %v, Received count - %v.", FileCountLimit, len(validFiles))
		return
	}
	data, err = getMetaDataFunc(log, validFiles)
	return
}

//getAllMeta processes the filter, gets paths of all filtered files, and get file info of all files
func getAllMeta(log log.T, config model.Config) (data []model.FileData, err error) {
	jsonBody := []byte(strings.Replace(config.Filters, `\`, `/`, -1)) //this is to convert the backslash in windows path to slash
	var filterList []filterObj
	if err = json.Unmarshal(jsonBody, &filterList); err != nil {
		LogError(log, err)
		return
	}
	for _, filter := range filterList {

		var fullPath string
		var getPathErr error
		if fullPath, getPathErr = getFullPath(filter.Path, os.Getenv); getPathErr != nil {
			LogError(log, getPathErr)
			continue
		}
		dataTmp, err := getFilesFunc(log, fullPath, filter.Pattern, filter.Recursive)
		if err != nil {
			LogError(log, err)
			if err.Error() == FileCountLimitExceeded {
				return nil, err
			}
		}
		data = removeDuplicatesFileData(append(data, dataTmp...))
		if len(data) > FileCountLimit {
			err = fmt.Errorf(FileCountLimitExceeded)
			return nil, err
		}
	}
	return data, nil
}

//fileMatchesAnyPattern returns true if file name matches any pattern specified
func fileMatchesAnyPattern(log log.T, pattern []string, fname string) bool {
	for _, item := range pattern {
		matched, matchErr := filepath.Match(item, fname)
		if matchErr != nil {
			LogError(log, matchErr)
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

//collectFileData returns a list of file information based on the given configuration
func collectFileData(context context.T, config model.Config) (data []model.FileData, err error) {
	log := context.Log()
	getFullPath = expand
	data, err = getAllMeta(log, config)
	return
}
