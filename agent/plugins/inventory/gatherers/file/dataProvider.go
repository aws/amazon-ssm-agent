package file

import (
	"errors"
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
	Path         string
	Pattern      []string
	Recursive    bool
	DirScanLimit *int
}

type fileInfoObject struct {
	log  log.T
	fi   os.FileInfo
	path string
}

// Limits to help keep file information under item size limit and prevent long scanning.
// The Dir Limits can be configured through input parameters
const FileCountLimit = 500
const FileCountLimitExceeded = "File Count Limit Exceeded"
const DirScanLimit = 5000
const DirScanLimitExceeded = "Directory Scan Limit Exceeded"

//decoupling for easy testability
var readDirFunc = ReadDir
var existsPath = exists
var getFullPath func(path string, mapping func(string) string) (string, error)
var filepathWalk = filepath.Walk
var getFilesFunc = getFiles
var getMetaDataFunc = getMetaData
var DirScanLimitError = errors.New(DirScanLimitExceeded)
var FileCountLimitError = errors.New(FileCountLimitExceeded)

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

func getFiles(log log.T, path string, pattern []string, recursive bool, fileLimit int, dirLimit int) (validFiles []string, err error) {
	var ex bool
	ex, err = existsPath(path)
	if err != nil {
		LogError(log, err)
		return
	}
	if !ex {
		LogError(log, fmt.Errorf("Path %v does not exist!", path))
		return
	}
	dirScanCount := 0
	if recursive {
		err = filepathWalk(path, func(fp string, fi os.FileInfo, err error) error {
			if err != nil {
				LogError(log, err)
				return nil
			}
			if fi.IsDir() {
				dirScanCount++
				if dirScanCount > dirLimit {
					log.Errorf("Scanned maximum allowed directories. Returning collected files")
					return DirScanLimitError
				}
				return nil

			}
			if fileMatchesAnyPattern(log, pattern, fi.Name()) {
				validFiles = append(validFiles, fp)
				if len(validFiles) > fileLimit {
					log.Errorf("Found more than limit of %d files", FileCountLimit)
					return FileCountLimitError
				}
			}
			return nil
		})
	} else {
		files, readDirErr := readDirFunc(path)
		if readDirErr != nil {
			LogError(log, readDirErr)
			err = readDirErr
			return
		}

		dirScanCount++
		for _, fi := range files {
			if fi.IsDir() {
				continue
			}
			if fileMatchesAnyPattern(log, pattern, fi.Name()) {
				validFiles = append(validFiles, filepath.Join(path, fi.Name()))
				if len(validFiles) > fileLimit {
					log.Errorf("Found more than limit of %d files", FileCountLimit)
					err = FileCountLimitError
					return
				}
			}
		}

	}

	log.Infof("DirScanned %d", dirScanCount)
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
	var fileList []string
	for _, filter := range filterList {

		var fullPath string
		var getPathErr error
		var dirScanLimit int
		if fullPath, getPathErr = getFullPath(filter.Path, os.Getenv); getPathErr != nil {
			LogError(log, getPathErr)
			continue
		}
		fileLimit := FileCountLimit - len(fileList)
		if filter.DirScanLimit == nil {
			dirScanLimit = DirScanLimit
		} else {
			dirScanLimit = *filter.DirScanLimit
		}
		log.Infof("Dir Scan Limit %d", dirScanLimit)
		foundFiles, getFilesErr := getFilesFunc(log, fullPath, filter.Pattern, filter.Recursive, fileLimit, dirScanLimit)
		// We should only break, if we get limit error, otherwise we should continue collecting other data
		if getFilesErr != nil {
			LogError(log, getFilesErr)
			if getFilesErr == FileCountLimitError || getFilesErr == DirScanLimitError {
				return nil, getFilesErr
			}
		}
		fileList = append(fileList, foundFiles...)
		fileList = removeDuplicatesString(fileList)
	}

	if len(fileList) > 0 {
		data, err = getMetaDataFunc(log, fileList)
	}
	log.Infof("Collected Files %d", len(data))
	return
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
