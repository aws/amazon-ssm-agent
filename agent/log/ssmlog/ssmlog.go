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
	"fmt"

	"sync"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/cihub/seelog"
)

// loaded logger
var loadedLogger *log.T
var lock sync.RWMutex

// pkgMutex is the lock used to serialize calls to the logger.
var pkgMutex = new(sync.Mutex)

// loggerInstance is the delegate logger in the wrapper
var loggerInstance = &log.DelegateLogger{}

func SSMLogger(useWatcher bool) log.T {
	if !isLoaded() {
		logger := initLogger(useWatcher)
		cache(logger)
	}
	return getCached()
}

// initLogger initializes a new logger based on current configurations and starts file watcher on the configurations file
func initLogger(useWatcher bool) (logger log.T) {
	// Read the current configurations or get the default configurations
	logConfigBytes := log.GetLogConfigBytes()
	// Initialize the base seelog logger
	baseLogger, _ := initBaseLoggerFromBytes(logConfigBytes)
	// Create the wrapper logger
	logger = withContext(baseLogger)
	if useWatcher {
		// Start the config file watcher
		startWatcher(logger)
	}
	return
}

// withContext creates a wrapper logger on the base logger passed with context is passed
func withContext(logger seelog.LoggerInterface, context ...string) (contextLogger log.T) {
	loggerInstance.BaseLoggerInstance = logger
	formatFilter := &log.ContextFormatFilter{Context: context}
	contextLogger = &log.Wrapper{Format: formatFilter, M: pkgMutex, Delegate: loggerInstance}

	setStackDepth(logger)
	return contextLogger
}

// setStackDepth sets the stack depth of the logger passed
func setStackDepth(logger seelog.LoggerInterface) {
	// additional stack depth so that we print the calling function correctly
	// stack depth 0 would print the function in the wrapper (e.g. wrapper.Debug)
	// stack depth 1 prints the function calling the logger (wrapper), which is what we want.
	logger.SetAdditionalStackDepth(1)
}

// GetUpdaterLogger loads logger for agent updater
func GetUpdaterLogger(logRoot string, logFile string) log.T {
	if !isLoaded() {
		logger := loadUpdaterLogger(logRoot, logFile)
		cache(logger)
	}
	return getCached()
}

// loadUpdaterLogger loads the logger config from the hardcoded default config
func loadUpdaterLogger(logRoot string, logFile string) (logger log.T) {
	return initLoggerFromBytes(defaultUpdaterConfig(logRoot, logFile))
}

func defaultUpdaterConfig(logRoot string, logFile string) []byte {
	return log.LoadLog(logRoot, logFile)
}

// check if a logger has be loaded
func isLoaded() bool {
	lock.RLock()
	defer lock.RUnlock()
	return loadedLogger != nil
}

// cache the loaded logger
func cache(logger log.T) {
	lock.Lock()
	defer lock.Unlock()
	loadedLogger = &logger
}

// return the cached logger
func getCached() log.T {
	lock.RLock()
	defer lock.RUnlock()
	return *loadedLogger
}

// startWatcher starts the file watcher on the seelog configurations file path
func startWatcher(logger log.T) {
	defer func() {
		// In case the creation of watcher panics, let the current logger continue
		if msg := recover(); msg != nil {
			logger.Errorf("Seelog File Watcher Initilization Failed. Any updates on config file will be ignored unless agent is restarted: %v", msg)
		}
	}()
	fileWatcher := &FileWatcher{}
	fileWatcher.Init(logger, log.DefaultSeelogConfigFilePath, replaceLogger)
	// Start the file watcher
	fileWatcher.Start()
}

// ReplaceLogger replaces the current logger with a new logger initialized from the current configurations file
func replaceLogger() {
	fmt.Println("Replacing Logger")

	// Get the current logger
	logger := getCached()

	//Create new logger
	logConfigBytes := log.GetLogConfigBytes()
	baseLogger, err := initBaseLoggerFromBytes(logConfigBytes)

	// If err in creating logger, do not replace logger
	if err != nil {
		logger.Error("New logger creation failed")
		return
	}

	setStackDepth(baseLogger)
	baseLogger.Debug("New Logger Successfully Created")

	// Safe conversion to *Wrapper
	wrapper, ok := logger.(*log.Wrapper)
	if !ok {
		logger.Errorf("Logger replace failed. The logger is not a wrapper")
		return
	}

	// Replace the underlying base logger in wrapper
	wrapper.ReplaceDelegate(baseLogger)
}

// initLoggerFromBytes creates a new wrapper logger from configurations passed
func initLoggerFromBytes(seelogConfig []byte) log.T {
	logger, _ := initBaseLoggerFromBytes(seelogConfig)
	return withContext(logger)
}

// initBaseLoggerFromBytes initializes the base logger using the specified configuration as bytes.
func initBaseLoggerFromBytes(seelogConfig []byte) (seelogger seelog.LoggerInterface, err error) {
	fmt.Println("Initializing new seelog logger")
	logReceiver := &CloudWatchCustomReceiver{}
	seelog.RegisterReceiver("cloudwatch_receiver", logReceiver)
	seelogger, err = seelog.LoggerFromConfigAsBytes(seelogConfig)
	if err != nil {
		fmt.Println("Error parsing logger config. Creating logger from default config:", err)
		// Create logger with default config
		seelogger, _ = seelog.LoggerFromConfigAsBytes(log.DefaultConfig())
	}

	fmt.Println("New Seelog Logger Creation Complete")
	return
}
