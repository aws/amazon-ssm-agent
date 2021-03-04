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

// Package ssmlog is used to initialize ssm functional logger
package ssmlog

import (
	"path/filepath"
	"runtime/debug"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/fsnotify/fsnotify"
)

// IFileWatcher interface for FileWatcher with functions to initialize, start and stop the watcher
type IFileWatcher interface {
	Init(log log.T, configFilePath string, replaceLogger func())
	Start()
	Stop()
}

// FileWatcher implements the IFileWatcher by using fileChangeWatcher and fileExistsWatcher
type FileWatcher struct {
	configFilePath string
	replaceLogger  func()
	log            log.T
	watcher        *fsnotify.Watcher
}

// Init initializes the data and channels for the filewatcher
func (fileWatcher *FileWatcher) Init(log log.T, configFilePath string, replaceLogger func()) {
	fileWatcher.replaceLogger = replaceLogger
	fileWatcher.configFilePath = configFilePath
	fileWatcher.log = log
}

// Start creates and starts the go routines for filewatcher
func (fileWatcher *FileWatcher) Start() {

	fileWatcher.log.Debugf("Start File Watcher On: %v", fileWatcher.configFilePath)

	// Since the filewatcher fails if the file does not exist, need to watch the parent directory for any changes
	dirPath := filepath.Dir(fileWatcher.configFilePath)
	fileWatcher.log.Debugf("Start Watcher on directory: %v", dirPath)

	// Creating Watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		// Error initializing the watcher
		fileWatcher.log.Errorf("Error initializing the watcher: %v", err)
		return
	}

	fileWatcher.watcher = watcher

	// Starting the goroutine for event handler
	go fileWatcher.fileEventHandler()

	// Add the directory to watcher
	err = fileWatcher.watcher.Add(dirPath)
	if err != nil {
		// Error adding the file to watcher
		fileWatcher.log.Warnf("Error adding the directory '%s' to watcher: %v", dirPath, err)
		return
	}
}

// fileEventHandler implements handling of the events triggered by the OS
func (fileWatcher *FileWatcher) fileEventHandler() {
	defer func() {
		if r := recover(); r != nil {
			fileWatcher.log.Errorf("File event handler panic: \n%v", r)
			fileWatcher.log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()
	// Waiting on signals from OS
	for event := range fileWatcher.watcher.Events {
		// Event signalled by OS on file
		fileWatcher.log.Debugf("Event on file %v : %v", event.Name, event)
		if event.Name == fileWatcher.configFilePath {
			// Event on the file being watched
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create || event.Op&fsnotify.Rename == fsnotify.Rename {
				// One of Write or Create or Rename Event
				fileWatcher.log.Debugf("File Watcher Triggers Function Execution: %v", fileWatcher.configFilePath)
				// Execute the function
				fileWatcher.replaceLogger()
			}
		}
	}
}

// Stop stops the filewatcher
func (fileWatcher *FileWatcher) Stop() {
	fileWatcher.log.Infof("Stop the filewatcher on :%v", fileWatcher.configFilePath)
	// Check if watcher instance is set
	if fileWatcher.watcher != nil {
		err := fileWatcher.watcher.Close()
		if err != nil {
			// Error closing the filewatcher. Logging the error
			fileWatcher.log.Debugf("Error Closing the filewatcher :%v", err)
		}
	}
}
