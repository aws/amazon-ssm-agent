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

// Package system have all the files related dependencies used by the copy package
package system

import (
	"github.com/aws/amazon-ssm-agent/agent/filemanager"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"

	"fmt"
	"os"
	"path/filepath"
)

var SetPermission = chmod

// SaveFileContent is a method that returns the content in a file and saves it on disk
func SaveFileContent(log log.T, filesysdep filemanager.FileSystem, destDir string, contents string, resourceFilePath string) (err error) {

	filePath := fileutil.BuildPath(destDir, resourceFilePath)
	destinationDir := filepath.Dir(filePath)

	log.Debugf("Destination dir is %v and the file path is %v ", destinationDir, filePath)
	// create directory to download github resources
	if err = filesysdep.MakeDirs(destinationDir); err != nil {
		log.Error("failed to create directory for github - ", err)
		return err
	}
	log.Debug("Content obtained - ", contents)

	if err = filesysdep.WriteFile(filePath, contents); err != nil {
		log.Errorf("Error writing to file %v - %v", filePath, err)
		return err
	}

	return nil
}

// ReadFileContents is a method to read the contents of a give file path
func ReadFileContents(log log.T, filesysdep filemanager.FileSystem, destinationPath string) (fileContent []byte, err error) {

	log.Debug("Reading file contents from file - ", destinationPath)

	var rawFile string
	if rawFile, err = filesysdep.ReadFile(destinationPath); err != nil {
		log.Error("Error occured while reading file - ", err)
		return nil, err
	}
	if rawFile == "" {
		return []byte(rawFile), fmt.Errorf("File is empty!")
	}

	return []byte(rawFile), nil
}

// RenameFile is a method that renames a file and deletes the original copy
func RenameFile(log log.T, filesys filemanager.FileSystem, fullSourceName, destName string) error {

	filePath := filepath.Dir(fullSourceName)
	log.Debug("File path is ", filePath)
	log.Debug("New file name is ", destName)

	if _, err := filesys.MoveAndRenameFile(filePath, filepath.Base(fullSourceName), filePath, destName); err != nil {
		return err
	}
	return nil
}

func chmod(name string, mode os.FileMode) error {
	return os.Chmod(name, mode)
}
