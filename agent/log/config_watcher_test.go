// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package log is used to initialize the logger.

package log

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/stretchr/testify/assert"
)

var logMock = NewMockLog()

func TestWatcherOnWrite(t *testing.T) {
	path := filepath.Join("testdata", "testfile1.txt")
	// Create File
	fileutil.WriteAllText(path, "TestData")
	fileWatcher := &FileWatcher{}
	testValue := false

	fileWatcher.Init(logMock, path, func() {
		fmt.Println("TestInitWatcher Function Triggered")
		testValue = true
	})

	//Start Watcher
	fileWatcher.Start()
	time.Sleep(100 * time.Millisecond)

	// Writing to file
	fileutil.WriteAllText(path, "TestData1")
	time.Sleep(200 * time.Millisecond)

	// Checking whether function triggered
	assert.True(t, testValue, "Function not triggered on file write")

}

func TestWatcherOnExists(t *testing.T) {
	path := filepath.Join("testdata", "testfile1.txt")
	// Create File
	fileutil.WriteAllText(path, "TestData")
	fileWatcher := &FileWatcher{}
	testValue := false

	fileWatcher.Init(logMock, path, func() {
		fmt.Println("TestInitWatcher Function Triggered")
		testValue = true
	})

	//Start Watcher
	fileWatcher.Start()
	time.Sleep(200 * time.Millisecond)

	assert.False(t, testValue, "Function should not be triggered as file exists and no action performed")
}

func TestWatcherOnDelete(t *testing.T) {

	path := filepath.Join("testdata", "testfileOnDelete.txt")
	// Create File
	fileutil.WriteAllText(path, "TestData")
	fileWatcher := &FileWatcher{}
	testValue := false

	fileWatcher.Init(logMock, path, func() {
		fmt.Println("TestInitWatcher Function Triggered")
		testValue = true
	})

	//Start Watcher
	fileWatcher.Start()
	time.Sleep(100 * time.Millisecond)

	// Deleting File
	fileutil.DeleteFile(path)
	time.Sleep(200 * time.Millisecond)

	assert.False(t, testValue, "Function should not be triggered on file delete")
}

func TestWatcherOnCreate(t *testing.T) {
	path := filepath.Join("testdata", "testfile2.txt")
	// Delete File if exists
	if fileutil.Exists(path) {
		fileutil.DeleteFile(path)
	}

	functionTriggered := false
	fileWatcher := &FileWatcher{}
	fileWatcher.Init(logMock, path, func() {
		fmt.Println("Function Triggered")
		functionTriggered = true
	})

	// Start the watcher
	fileWatcher.Start()
	time.Sleep(100 * time.Millisecond)

	// Create File
	fileutil.WriteAllText(path, "TestData")
	time.Sleep(200 * time.Millisecond)

	// Checking whether function was triggered
	assert.True(t, functionTriggered, "Function not triggered even though file created")

}

func TestWatcherOnFileDoesNotExist(t *testing.T) {
	path := filepath.Join("testdata", "testfiledoesnotexist.txt")
	// Delete File if exists
	if fileutil.Exists(path) {
		fileutil.DeleteFile(path)
	}

	functionTriggered := false
	fileWatcher := &FileWatcher{}
	fileWatcher.Init(logMock, path, func() {
		fmt.Println("Function Triggered")
		functionTriggered = true
	})

	// Starting Watcher
	fileWatcher.Start()
	time.Sleep(100 * time.Millisecond)

	time.Sleep(200 * time.Millisecond)

	// Checking whether function was triggered
	assert.False(t, functionTriggered, "Function Should not be triggered as file does not exist")

}

func TestWatcherOnMultipleDeletesWrites(t *testing.T) {
	path := filepath.Join("testdata", "testfileMultiple.txt")
	// Create File
	fileutil.WriteAllText(path, "TestData")

	testValue := false

	fileWatcher := &FileWatcher{}
	fileWatcher.Init(logMock, path, func() {
		fmt.Println("Function Triggered")
		testValue = true
	})

	//Start Watcher
	fileWatcher.Start()
	time.Sleep(100 * time.Millisecond)

	// Delete File
	fileutil.DeleteFile(path)
	time.Sleep(200 * time.Millisecond)
	assert.False(t, testValue, "Function should not be triggered on file delete")

	// Write To File
	fileutil.WriteAllText(path, "TestData")
	time.Sleep(200 * time.Millisecond)
	assert.True(t, testValue, "Function should be triggered on file write")

	// Flipping flag to false
	testValue = false

	// 2nd Write To File
	fileutil.WriteAllText(path, "TestData")
	time.Sleep(100 * time.Millisecond)
	assert.True(t, testValue, "Function should be triggered on 2nd file write")

}
