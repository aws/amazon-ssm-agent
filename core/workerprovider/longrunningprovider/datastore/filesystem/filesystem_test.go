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

// Package filesystem contains related functions from os, io, and io/ioutil packages
package filesystem

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

func TestWrite_WhenPathExists(t *testing.T) {

}

type FileSystemTestSuite struct {
	suite.Suite

	fileSys *FileSystem
}

func (suite *FileSystemTestSuite) SetupTest() {
	fileSys := &FileSystem{}
	suite.fileSys = fileSys
}

// Execute the test suite
func TestFileSystemTestSuite(t *testing.T) {
	suite.Run(t, new(FileSystemTestSuite))
}

func (suite *FileSystemTestSuite) TestWrite_Directory() {

	err := suite.fileSys.MkdirAll("test", 0600)
	assert.Nil(suite.T(), err)

	os.RemoveAll("test")
}

func (suite *FileSystemTestSuite) TestWrite_File() {
	fileContent := "test"
	fileName := "test.txt"

	err := suite.fileSys.WriteFile(fileName, []byte(fileContent), 0600)
	assert.Nil(suite.T(), err)

	fileContentBytes, err := suite.fileSys.ReadFile(fileName)
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), fileContent, string(fileContentBytes))

	fileInfo, err := suite.fileSys.Stat(fileName)
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), fileInfo.Name(), fileName)

	os.Remove(fileName)
}

func (suite *FileSystemTestSuite) TestWrite_FileExist() {
	fileName := "test.txt"

	_, err := suite.fileSys.Stat(fileName)
	result := suite.fileSys.IsNotExist(err)
	assert.True(suite.T(), result)

}

func (suite *FileSystemTestSuite) TestWrite_AppendDeleteFile() {
	fileName := "test.txt"
	fileContent := "Test Content"

	defer os.Remove(fileName)

	_, err := suite.fileSys.Stat(fileName)
	result := suite.fileSys.IsNotExist(err)
	assert.True(suite.T(), result)

	//Append/Create test case
	fileErr := suite.fileSys.AppendToFile(fileName, fileContent, 0600)
	assert.Nil(suite.T(), fileErr)

	fileContentBytes, err := suite.fileSys.ReadFile(fileName)
	assert.Equal(suite.T(), fileContent, string(fileContentBytes))
	assert.Nil(suite.T(), err)

	fileErr = suite.fileSys.AppendToFile(fileName, fileContent, 0600)
	assert.Nil(suite.T(), fileErr)

	fileContentBytes, err = suite.fileSys.ReadFile(fileName)
	assert.Equal(suite.T(), fileContent+fileContent, string(fileContentBytes))
	assert.Nil(suite.T(), err)

	//Delete file test case
	err = suite.fileSys.DeleteFile(fileName)
	assert.Nil(suite.T(), err)

	_, err = suite.fileSys.Stat(fileName)
	result = suite.fileSys.IsNotExist(err)
	assert.True(suite.T(), result)
}

func (suite *FileSystemTestSuite) TestWrite_ReadDir() {
	fileContent := "Test Content"
	dirPath := "test"

	err := suite.fileSys.MkdirAll(dirPath, 0600)
	assert.Nil(suite.T(), err)
	fileNames := []string{"test1.txt", "test2.txt"}
	defer func() {
		for _, file := range fileNames {
			os.Remove(file)
		}
		os.RemoveAll(dirPath)
	}()
	for _, file := range fileNames {
		suite.fileSys.WriteFile(file, []byte(fileContent), 0600)
	}

	fileList, err := suite.fileSys.ReadDir(dirPath)
	for i := 0; i < len(fileList); i++ {
		assert.Equal(suite.T(), fileList[i], fileNames[i])
		assert.Nil(suite.T(), err)
	}
}
