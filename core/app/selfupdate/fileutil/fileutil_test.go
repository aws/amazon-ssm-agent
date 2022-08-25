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

package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type FileUtilTestSuite struct {
	suite.Suite
	log      *log.Mock
	fileutil *Fileutil
}

func (suite *FileUtilTestSuite) SetupTest() {
	suite.log = log.NewMockLog()
	suite.fileutil = NewFileUtil(suite.log)
}

func (suite *FileUtilTestSuite) TestLocalFileExist() {
	// file exists
	path := filepath.Join("testdata", "test.txt")
	isExist, err := suite.fileutil.LocalFileExist(path)
	if err != nil {
		suite.log.Errorf("error: %v", err)
	}
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), true, isExist)

	// file does not exist
	path = filepath.Join("testdata", "blah")
	isExist, err = suite.fileutil.LocalFileExist(path)
	if err != nil {
		suite.log.Errorf("error: %v", err)
	}
	assert.Equal(suite.T(), false, isExist)
}

func (suite *FileUtilTestSuite) TestExists() {
	path := filepath.Join("testdata", "test.txt")
	isExist := suite.fileutil.Exists(path)
	assert.Equal(suite.T(), true, isExist)
}

func (suite *FileUtilTestSuite) TestReadAllText() {
	// valid file
	path := filepath.Join("testdata", "test.txt")
	expectContent := "This is a test file"
	content, err := suite.fileutil.ReadAllText(path)
	if err != nil {
		suite.log.Errorf("error: %v", err)
	}
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), content, expectContent)

	// invalid file
	path = filepath.Join("testdata", "invalid.txt")
	content, err = suite.fileutil.ReadAllText(path)
	if err != nil {
		suite.log.Errorf("error: %v", err)
	}
	assert.Equal(suite.T(), content, "")
}

func (suite *FileUtilTestSuite) TestMakeDirs() {
	// No error test
	dir := "sampledir"
	err := suite.fileutil.MakeDirs(dir)
	assert.NoError(suite.T(), err, "expected no error")

	// error test
	suite.fileutil.fs = osFSStub{err: fmt.Errorf("someerror")}
	dir = "sampledir"
	err = suite.fileutil.MakeDirs(dir)
	assert.Error(suite.T(), err, "expected some error")
}

func (suite *FileUtilTestSuite) TestDeleteFile() {
	file := "samplefile"

	// No error test
	suite.fileutil.fs = osFSStub{}
	err := suite.fileutil.DeleteFile(file)
	assert.NoError(suite.T(), err, "expected no error")

	// error test
	suite.fileutil.fs = osFSStub{err: fmt.Errorf("someerror")}
	err = suite.fileutil.MakeDirs(file)
	assert.Error(suite.T(), err, "expected some error")
}

func (suite *FileUtilTestSuite) TestUnderDir() {
	// Remove one or more directory levels
	assert.True(suite.T(), suite.fileutil.isUnderDir(`~/foo/bar/../`, `~/foo`))
	assert.True(suite.T(), suite.fileutil.isUnderDir(`~/foo/bar/..`, `~/foo`))
	assert.False(suite.T(), suite.fileutil.isUnderDir(`~/foo/bar/../..`, `~/foo`))

	// Remove one or more directory levels and add some levels
	assert.True(suite.T(), suite.fileutil.isUnderDir(`~/foo/bar/../../foo`, `~/foo`))
	assert.False(suite.T(), suite.fileutil.isUnderDir(`~/foo/bar/../../bar`, `~/foo`))
	assert.True(suite.T(), suite.fileutil.isUnderDir(`~/foo/bar/../../foo/bar`, `~/foo/bar`))
	assert.False(suite.T(), suite.fileutil.isUnderDir(`~/foo/bar/../../bar`, `~/foo/bar`))

	// Ensure partial hex and unicode encoded strings also work
	assert.True(suite.T(), suite.fileutil.isUnderDir("~\x2ffoo\x2fbar", `~/foo`))
	assert.True(suite.T(), suite.fileutil.isUnderDir("~/foo/bar\x2f\x2e\x2e", `~/foo`))
	assert.False(suite.T(), suite.fileutil.isUnderDir("~/foo/bar\x2f\x2e\x2e", `~/foo/bar`))
	assert.False(suite.T(), suite.fileutil.isUnderDir("~/foo/bar\x2f\x2e\u002e", `~/foo/bar`))

	// Ensure handling of trailing separators and substrings that are different directories works correctly
	assert.True(suite.T(), suite.fileutil.isUnderDir("/foo/bar/", "/foo/bar"))
	assert.True(suite.T(), suite.fileutil.isUnderDir("/foo/bar", "/foo/bar/"))
	assert.False(suite.T(), suite.fileutil.isUnderDir("/foo/barbaz", "/foo/bar"))

	// Assert behavior involving ~ (it is treated as a single directory level)
	assert.True(suite.T(), suite.fileutil.isUnderDir(`~/../foo`, `foo`))
	assert.True(suite.T(), suite.fileutil.isUnderDir(`~/../../foo`, `../foo`))
}

// Execute the test suite
func TestFileUtilTestSuite(t *testing.T) {
	suite.Run(t, new(FileUtilTestSuite))
}

type osFSStub struct {
	exists   bool
	file     *os.File
	fileInfo os.FileInfo
	err      error
}

// TODO: optimize the test with Mock
func (a osFSStub) IsNotExist(err error) bool                                      { return a.exists }
func (a osFSStub) MkdirAll(path string, perm os.FileMode) error                   { return a.err }
func (a osFSStub) Open(name string) (*os.File, error)                             { return a.file, a.err }
func (a osFSStub) Stat(name string) (os.FileInfo, error)                          { return a.fileInfo, a.err }
func (a osFSStub) Remove(name string) error                                       { return a.err }
func (a osFSStub) Rename(oldpath string, newpath string) error                    { return a.err }
func (a osFSStub) WriteFile(filename string, data []byte, perm os.FileMode) error { return a.err }
