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

func ExampleRemoveInvalidChars() {
	// Path with invalid char
	path1 := "Fix:ThisPath"
	content1 := RemoveInvalidChars(path1)
	fmt.Println(content1)

	// path with no invalid char
	path2 := "DoNotFixThisPath"
	content2 := RemoveInvalidChars(path2)
	fmt.Println(content2)

	// empty path should not return error
	path3 := ""
	content3 := RemoveInvalidChars(path3)
	fmt.Println(content3)
	// Output:
	// FixThisPath
	// DoNotFixThisPath
	//
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
