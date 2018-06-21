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

//Package fileutil contains utilities for working with the file system.
package fileutil

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
)

type ByteOrderMark uint8

const (
	ByteOrderMarkEmit ByteOrderMark = iota
	ByteOrderMarkSkip ByteOrderMark = iota
)

func CreateUTF8ByteOrderMark() (result []byte) {
	return []byte{0xEF, 0xBB, 0xBF}
}

// DiskSpaceInfo stores the available, free, and total bytes
type DiskSpaceInfo struct {
	AvailBytes int64
	FreeBytes  int64
	TotalBytes int64
}

// DeleteFile deletes the specified file
func DeleteFile(filepath string) (err error) {
	return fs.Remove(filepath)
}

// DeleteDirectory deletes a directory and all its content.
func DeleteDirectory(dirName string) (err error) {

	return os.RemoveAll(dirName)
}

// ReadAllText reads all content from the specified file
func ReadAllText(filePath string) (text string, err error) {
	var exists = false
	exists, err = LocalFileExist(filePath)
	if err != nil || exists == false {
		return
	}

	buf := bytes.NewBuffer(nil)
	f, _ := os.Open(filePath)
	defer f.Close()
	_, err = io.Copy(buf, f)
	if err != nil {
		return
	}
	text = string(buf.Bytes())
	return
}

// AppendToFile appends content to file
func AppendToFile(fileDirectory string, filename string, content string) (filePath string, err error) {
	filePath = filepath.Join(fileDirectory, filename)
	fileWriter, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, appconfig.ReadWriteAccess)
	if err != nil {
		err = fmt.Errorf("failed to open the file at %v: %v", filePath, err)
	}

	if fileWriter.WriteString(content); err != nil {
		err = fmt.Errorf("failed to write contents to file")
	}
	defer fileWriter.Close()
	return filePath, err
}

// WriteAllText writes all text content to the specified file
func WriteAllText(filePath string, text string) (err error) {
	f, _ := os.Create(filePath)
	defer f.Close()
	_, err = f.WriteString(text)
	return
}

// Exists returns true if the given file exists, false otherwise, ignoring any underlying error
func Exists(filePath string) bool {
	exist, _ := LocalFileExist(filePath)
	return exist
}

// LocalFileExist returns true if the given file exists, false otherwise.
func LocalFileExist(path string) (bool, error) {
	_, err := fs.Stat(path)
	if err == nil {
		return true, nil
	}
	if fs.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// BuildPath joins the orchestration directory path with valid components.
func BuildPath(root string, elements ...string) string {
	fullPath := root
	for _, element := range elements {
		fullPath = filepath.Join(fullPath, removeInvalidColon(element))
	}
	return fullPath
}

// BuildS3Path joins the root directory path with valid components.
func BuildS3Path(root string, elements ...string) string {
	fullPath := root
	for _, element := range elements {
		fullPath = path.Join(fullPath, removeInvalidColon(element))
	}
	return fullPath
}

// removeInvalidColon strips any invalid colon from plugin name.
func removeInvalidColon(pluginName string) string {
	if pluginName != "" {
		return strings.Replace(pluginName, ":", "", -1)
	}
	return pluginName
}

// MakeDirs create the directories along the path if missing.
func MakeDirs(destinationDir string) (err error) {
	// create directory
	err = fs.MkdirAll(destinationDir, appconfig.ReadWriteAccess)
	if err != nil {
		err = fmt.Errorf("failed to create directory %v. %v", destinationDir, err)
	}
	return
}

// MakeDirsWithExecuteAccess create the directories along the path if missing.
func MakeDirsWithExecuteAccess(destinationDir string) (err error) {
	// create directory
	if err = fs.MkdirAll(destinationDir, appconfig.ReadWriteExecuteAccess); err != nil {
		err = fmt.Errorf("failed to create directory %v. %v", destinationDir, err)
	}
	return
}

func GetFileMode(path string) (mode os.FileMode) {
	fileInfo, err := fs.Stat(path)
	if err != nil {
		err = fmt.Errorf("error looking up path information Path: %v, Error: %v", path, err)
		return 0
	}

	return fileInfo.Mode()
}

// IsDirectory returns true or false depending
// if given srcPath is directory or not
func IsDirectory(srcPath string) bool {

	srcFileInfo, err := fs.Stat(srcPath)
	if err != nil {
		err = fmt.Errorf("error looking up path information Path: %v, Error: %v", srcPath, err)
		return false
	}

	return srcFileInfo.Mode().IsDir()
}

// IsFile returns true or false depending if given
// srcPath is a regular file or not
func IsFile(srcPath string) bool {

	srcFileInfo, err := fs.Stat(srcPath)
	if err != nil {
		err = fmt.Errorf("error looking up path information Path: %v, Error: %v", srcPath, err)
		return false
	}

	return srcFileInfo.Mode().IsRegular()
}

// MoveFile moves file from srcPath directory to dstPath directory
// only if both directories exist
func MoveFile(filename, srcPath, dstPath string) (result bool, err error) {
	return MoveAndRenameFile(srcPath, filename, dstPath, filename)
}

// MoveAndRenameFile moves a file from the srcPath directory to dstPath directory and gives it a new name
func MoveAndRenameFile(srcPath, originalName, dstPath, newName string) (result bool, err error) {
	srcFile := filepath.Join(srcPath, originalName)
	dstFile := filepath.Join(dstPath, newName)

	if err = fs.Rename(srcFile, dstFile); err != nil {
		return false, fmt.Errorf("unexpected error encountered while moving the file. Error details - %v", err)
	}
	return true, nil
}

// WriteIntoFileWithPermissions writes into file with given file mode permissions
func WriteIntoFileWithPermissions(absolutePath, content string, perm os.FileMode) (result bool, err error) {
	return WriteIntoFileWithPermissionsExtended(absolutePath, content, perm, ByteOrderMarkSkip)
}

// WriteIntoFileWithPermissionsExtended writes into file with given file mode permissions
func WriteIntoFileWithPermissionsExtended(absolutePath, content string, perm os.FileMode, byteOrderMark ByteOrderMark) (result bool, err error) {
	result = true
	if byteOrderMark == ByteOrderMarkEmit {
		err = ioUtil.WriteFile(absolutePath, append(CreateUTF8ByteOrderMark(), []byte(content)...), perm)
	} else {
		err = ioUtil.WriteFile(absolutePath, []byte(content), perm)
	}
	if err != nil {
		err = fmt.Errorf("couldn't write into file - %v", err)
		result = false
	}
	return
}

// GetFileModificationTime returns the modification time of the file
func GetFileModificationTime(srcPath string) (modificationTime time.Time, err error) {

	srcFileInfo, err := fs.Stat(srcPath)
	if err != nil {
		err = fmt.Errorf("error looking up file information: %v, Error: %v", srcPath, err)
		return
	}

	modificationTime = srcFileInfo.ModTime()

	return
}

// IsDirEmpty returns true if the given directory is empty else it returns false
func IsDirEmpty(location string) (bool, error) {
	f, err := os.Open(location)
	if err != nil {
		err = fmt.Errorf("couldn't open path - %v", err)
		return false, err
	}
	defer f.Close()

	// read in ONLY one file
	_, err = f.Readdir(1)

	// if file is EOF -> dir is empty
	// else -> dir is non-empty
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

// GetDirectoryNames returns the names of all directories under a give srcPath
func GetDirectoryNames(srcPath string) (directories []string, err error) {
	if list, err := ioutil.ReadDir(srcPath); err == nil {
		directories = make([]string, 0)
		for _, fileinfo := range list {
			if fileinfo.Mode().IsDir() {
				directories = append(directories, fileinfo.Name())
			}
		}
	}
	return
}

// GetFileNames returns the names of all non-directories under a give srcPath
func GetFileNames(srcPath string) (files []string, err error) {
	if list, err := ioutil.ReadDir(srcPath); err == nil {
		files = make([]string, 0)
		for _, fileinfo := range list {
			if !fileinfo.Mode().IsDir() {
				files = append(files, fileinfo.Name())
			}
		}
	}
	return
}

// ReadDir returns files within the given location
func ReadDir(location string) ([]os.FileInfo, error) {
	files := []os.FileInfo{}
	if location == "" {
		return files, fmt.Errorf("location cannot be empty")
	}
	return ioutil.ReadDir(location)
}

// isUnderDir determines if a given path is in or under a given parent directory (after accounting for path traversal)
func isUnderDir(childPath, parentDirPath string) bool {
	return strings.HasPrefix(filepath.Clean(childPath)+string(filepath.Separator), filepath.Clean(parentDirPath)+string(filepath.Separator))
}

// Unzip unzips the installation package (using platform agnostic zip functionality)
// For platform specific implementation that uses tar.gz on Linux, use Uncompress
func Unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			return
		}
	}()

	os.MkdirAll(dest, appconfig.ReadWriteExecuteAccess)
	// Closure to address file descriptors issue with all the deferred .Close() methods
	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer func() {
			if err := rc.Close(); err != nil {
				return
			}
		}()

		path := filepath.Join(dest, f.Name)

		if !isUnderDir(path, dest) {
			return fmt.Errorf("%v attepts to place files outside %v subtree", f.Name, dest)
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
		} else {
			os.MkdirAll(filepath.Dir(path), f.Mode())
			f, err := os.OpenFile(path, appconfig.FileFlagsCreateOrTruncate, f.Mode())
			if err != nil {
				return err
			}
			defer func() {
				if err := f.Close(); err != nil {
					return
				}
			}()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
		return nil
	}
	for _, f := range r.File {
		err := extractAndWriteFile(f)
		if err != nil {
			return err
		}
	}

	return nil
}
