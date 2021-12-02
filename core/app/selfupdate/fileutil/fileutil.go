// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

type Fileutil struct {
	log log.T
	fs  IosFS
}

type IFileutil interface {
	DeleteFile(filepath string) (err error)
	ReadAllText(filePath string) (text string, err error)
	WriteAllText(filePath string, text string) (err error)
	Exists(filePath string) bool
	LocalFileExist(path string) (bool, error)
	MakeDirs(destinationDir string) (err error)
	GetFileMode(path string) (mode os.FileMode)
	Unzip(src, dest string) error
}

func NewFileUtil(log log.T) *Fileutil {
	var fs IosFS = osFS{}
	return &Fileutil{
		log: log,
		fs:  fs,
	}
}

// DeleteFile deletes the specified file
func (futl *Fileutil) DeleteFile(filepath string) (err error) {
	return futl.fs.Remove(filepath)
}

// ReadAllText reads all content from the specified file
func (futl *Fileutil) ReadAllText(filePath string) (text string, err error) {
	var exists = false
	exists, err = futl.LocalFileExist(filePath)
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

// WriteAllText writes all text content to the specified file
func (futl *Fileutil) WriteAllText(filePath string, text string) (err error) {
	f, _ := os.Create(filePath)
	defer f.Close()
	_, err = f.WriteString(text)
	return
}

// Exists returns true if the given file exists, false otherwise, ignoring any underlying error
func (futl *Fileutil) Exists(filePath string) bool {
	exist, _ := futl.LocalFileExist(filePath)
	return exist
}

// LocalFileExist returns true if the given file exists, false otherwise.
func (futl *Fileutil) LocalFileExist(path string) (bool, error) {
	_, err := futl.fs.Stat(path)
	if err == nil {
		return true, nil
	}
	if futl.fs.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// removeInvalidColon strips any invalid colon from plugin name.
func (futl *Fileutil) removeInvalidColon(pluginName string) string {
	if pluginName != "" {
		return strings.Replace(pluginName, ":", "", -1)
	}
	return pluginName
}

// MakeDirs create the directories along the path if missing.
func (futl *Fileutil) MakeDirs(destinationDir string) (err error) {
	// create directory
	err = futl.fs.MkdirAll(destinationDir, appconfig.ReadWriteExecuteAccess)
	if err != nil {
		err = fmt.Errorf("failed to create directory %v. %v", destinationDir, err)
	}
	return
}

func (futl *Fileutil) GetFileMode(path string) (mode os.FileMode) {
	fileInfo, err := futl.fs.Stat(path)
	if err != nil {
		err = fmt.Errorf("error looking up path information Path: %v, Error: %v", path, err)
		return 0
	}

	return fileInfo.Mode()
}

// isUnderDir determines if a given path is in or under a given parent directory (after accounting for path traversal)
func (futl *Fileutil) isUnderDir(childPath, parentDirPath string) bool {
	return strings.HasPrefix(filepath.Clean(childPath)+string(filepath.Separator), filepath.Clean(parentDirPath)+string(filepath.Separator))
}

// Unzip unzips the installation package (using platform agnostic zip functionality)
// For platform specific implementation that uses tar.gz on Linux, use Uncompress
func (futl *Fileutil) Unzip(src, dest string) error {
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

		if !futl.isUnderDir(path, dest) {
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
