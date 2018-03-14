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

package fileutil

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func ExampleLocalFileExist() {
	// file exists
	path := filepath.Join("artifact", "testdata", "CheckMyHash.txt")
	content, err := LocalFileExist(path)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	fmt.Println(content)

	// file does not exist
	path = filepath.Join("testdata", "blah")
	content, err = LocalFileExist(path)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	fmt.Println(content)
	// Output:
	// true
	// false
}

func ExampleExists() {
	path := filepath.Join("artifact", "testdata", "CheckMyHash.txt")
	content := Exists(path)
	fmt.Println(content)
	// Output:
	// true
}

func ExampleReadAllText() {
	// valid file
	path := filepath.Join("artifact", "testdata", "CheckMyHash.txt")
	content, err := ReadAllText(path)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	fmt.Println(content)

	// invalid file
	path = filepath.Join("testdata", "invalid.txt")
	content, err = ReadAllText(path)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	fmt.Println(content)
	// Output:
	// Will you please check my hash?
	// Will you please check my hash?
	// Will you please check my hash?
}

func ExampleBuildPath() {
	orchestrationDirectory := "/Users/"
	// Path with invalid char
	name1 := "Fix:ThisPath"
	content1 := BuildPath(orchestrationDirectory, name1)
	fmt.Println(content1)

	// path with no invalid char
	name2 := "DoNotFixThisPath"
	content2 := BuildPath(orchestrationDirectory, name2)
	fmt.Println(content2)

	// empty path should not return error
	name3 := ""
	content3 := BuildPath("", name3)
	fmt.Println(content3)

	// Path with several invalid chars
	name4 := "Fix:ThisPath1"
	name5 := "Fix:ThisPath2"
	content4 := BuildPath(orchestrationDirectory, name4, name5)
	fmt.Println(content4)
	// Output:
	// /Users/FixThisPath
	// /Users/DoNotFixThisPath
	//
	// /Users/FixThisPath1/FixThisPath2
}

func ExampleIsDirectory() {
	path := filepath.Join("artifact", "testdata", "CheckMyHash.txt")
	content := IsDirectory(path)
	fmt.Println(content)
	// Output:
	// false
}

func ExampleIsFile() {
	path := filepath.Join("artifact", "testdata", "CheckMyHash.txt")
	content := IsFile(path)
	fmt.Println(content)
	// Output:
	// true
}

func ExampleIsDirEmpty() {
	path := filepath.Join("artifact", "testdata")
	content, err := IsDirEmpty(path)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	fmt.Println(content)
	// Output:
	// false
}

func TestBuildPath(t *testing.T) {
	orchestrationDirectory := "/C:Users/"
	name1 := "Fix:ThisPath1"
	name2 := "DoNotFixThisPath"
	name3 := "Fix:ThisPath2"
	name4 := "Fix:ThisPath3"
	name1_removed := "FixThisPath1"
	name3_removed := "FixThisPath2"
	name4_removed := "FixThisPath3"
	res := BuildPath(orchestrationDirectory, name1, name2, name3, name4)
	exp := filepath.Join(orchestrationDirectory, name1_removed, name2, name3_removed, name4_removed)
	assert.Equal(t, exp, res)
}

func TestMakeDirs(t *testing.T) {
	// No error test
	fs = osFSStub{}
	dir := "sampledir"
	err := MakeDirs(dir)
	assert.NoError(t, err, "expected no error")

	// error test
	fs = osFSStub{err: fmt.Errorf("someerror")}
	dir = "sampledir"
	err = MakeDirs(dir)
	assert.Error(t, err, "expected some error")

	// reset
	fs = osFS{}
}

func TestDeleteFile(t *testing.T) {
	file := "samplefile"

	// No error test
	fs = osFSStub{}
	err := DeleteFile(file)
	assert.NoError(t, err, "expected no error")

	// error test
	fs = osFSStub{err: fmt.Errorf("someerror")}
	err = MakeDirs(file)
	assert.Error(t, err, "expected some error")

	// reset
	fs = osFS{}
}

func TestIsFile(t *testing.T) {
	file := "samplefile"

	// failure test
	fs = osFSStub{err: fmt.Errorf("someerror")}
	result := IsFile(file)
	assert.False(t, result, "expected false on error")

	// reset
	fs = osFS{}
}

func TestMoveFile(t *testing.T) {
	file := "samplefile"
	source := "samplefile"
	destination := "samplefile"

	// success test
	fs = osFSStub{}
	_, err := MoveFile(file, source, destination)
	assert.NoError(t, err, "expected no error")

	// failure test
	fs = osFSStub{err: fmt.Errorf("someerror")}
	_, err = MoveFile(file, source, destination)
	assert.Error(t, err, "expected error")

	// reset
	fs = osFS{}
}

func TestUnderDir(t *testing.T) {
	// Remove one or more directory levels
	assert.True(t, isUnderDir(`~/foo/bar/../`, `~/foo`))
	assert.True(t, isUnderDir(`~/foo/bar/..`, `~/foo`))
	assert.False(t, isUnderDir(`~/foo/bar/../..`, `~/foo`))

	// Remove one or more directory levels and add some levels
	assert.True(t, isUnderDir(`~/foo/bar/../../foo`, `~/foo`))
	assert.False(t, isUnderDir(`~/foo/bar/../../bar`, `~/foo`))
	assert.True(t, isUnderDir(`~/foo/bar/../../foo/bar`, `~/foo/bar`))
	assert.False(t, isUnderDir(`~/foo/bar/../../bar`, `~/foo/bar`))

	// Ensure partial hex and unicode encoded strings also work
	assert.True(t, isUnderDir("~\x2ffoo\x2fbar", `~/foo`))
	assert.True(t, isUnderDir("~/foo/bar\x2f\x2e\x2e", `~/foo`))
	assert.False(t, isUnderDir("~/foo/bar\x2f\x2e\x2e", `~/foo/bar`))
	assert.False(t, isUnderDir("~/foo/bar\x2f\x2e\u002e", `~/foo/bar`))

	// Ensure handling of trailing separators and substrings that are different directories works correctly
	assert.True(t, isUnderDir("/foo/bar/", "/foo/bar"))
	assert.True(t, isUnderDir("/foo/bar", "/foo/bar/"))
	assert.False(t, isUnderDir("/foo/barbaz", "/foo/bar"))

	// Assert behavior involving ~ (it is treated as a single directory level)
	assert.True(t, isUnderDir(`~/../foo`, `foo`))
	assert.True(t, isUnderDir(`~/../../foo`, `../foo`))
}

type osFSStub struct {
	exists   bool
	file     ioFile
	fileInfo os.FileInfo
	err      error
}

func (a osFSStub) IsNotExist(err error) bool                    { return a.exists }
func (a osFSStub) MkdirAll(path string, perm os.FileMode) error { return a.err }
func (a osFSStub) Open(name string) (ioFile, error)             { return a.file, a.err }
func (a osFSStub) Stat(name string) (os.FileInfo, error)        { return a.fileInfo, a.err }
func (a osFSStub) Remove(name string) error                     { return a.err }
func (a osFSStub) Rename(oldpath string, newpath string) error  { return a.err }

// ioutil stub
type ioUtilStub struct {
	b   []byte
	err error
}

func (a ioUtilStub) ReadFile(filename string) ([]byte, error) {
	return a.b, a.err
}

func TestAppendToFile(t *testing.T) {
	// Valid file
	var file = "testdata/file.txt"
	// call method
	filePath, err := AppendToFile("", file, " This is a sample text")
	assert.NoError(t, err, "expected no error")
	fmt.Println(filePath)
}
