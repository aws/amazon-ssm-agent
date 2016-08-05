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

// Package log is used to initialize the logger. This package should be imported once, usually from main, then call GetLogger.
package log

import (
	"fmt"
	"sync"

	"github.com/cihub/seelog"
)

const (
	LogFile   = "amazon-ssm-agent.log"
	ErrorFile = "errors.log"
)

// seelogDefault is the underlying seelog logger.
var seelogDefault seelog.LoggerInterface

// pkgMutex is the lock used to serialize calls to the logger.
var pkgMutex = new(sync.Mutex)

// loaded logger
var loadedLogger *T
var lock sync.RWMutex

// Logger loads logger based on the application configuration.
// it returns the loaded version, if any exists.
func Logger() T {
	if !isLoaded() {
		logger := initLogger()
		cache(logger)
	}
	return getCached()
}

// GetUpdaterLogger loads logger for agent updater
func GetUpdaterLogger(logRoot string, logFile string) T {
	if !isLoaded() {
		logger := loadUpdaterLogger(logRoot, logFile)
		cache(logger)
	}
	return getCached()
}

// check if a logger has be loaded
func isLoaded() bool {
	lock.RLock()
	defer lock.RUnlock()
	return loadedLogger != nil
}

// cache the loaded logger
func cache(logger T) {
	lock.Lock()
	defer lock.Unlock()
	loadedLogger = &logger
}

// return the cached logger
func getCached() T {
	lock.RLock()
	defer lock.RUnlock()
	return *loadedLogger
}

// loadUpdaterLogger loads the logger config from the hardcoded default config
func loadUpdaterLogger(logRoot string, logFile string) (logger T) {
	return initLoggerFromBytes(defaultUpdaterConfig(logRoot, logFile))
}

// WithContext creates a logger that includes the given context with every log message.
func WithContext(context ...string) (contextLogger T) {
	return withContext(seelogDefault, context...)
}

func withContext(logger seelog.LoggerInterface, context ...string) (contextLogger T) {
	formatFilter := &ContextFormatFilter{Context: context}
	contextLogger = &Wrapper{Delegate: logger, Format: formatFilter, M: pkgMutex}

	// additional stack depth so that we print the calling function correctly
	// stack depth 0 would print the function in the seelog logger (e.g. seelog.Debug)
	// stack depth 1 would print the function in the wrapper (e.g. wrapper.Debug)
	// stack depth 2 prints the function calling the logger (wrapper), which is what we want.
	logger.SetAdditionalStackDepth(2)
	return contextLogger
}

// ContextFormatFilter is a filter that can add a context to the parameters of a log message.
type ContextFormatFilter struct {
	Context []string
}

// Filter adds the context at the beginning of the parameter slice.
func (f ContextFormatFilter) Filter(params ...interface{}) (newParams []interface{}) {
	newParams = make([]interface{}, len(f.Context)+len(params))
	for i, param := range f.Context {
		newParams[i] = param + " "
	}
	ctxLen := len(f.Context)
	for i, param := range params {
		newParams[ctxLen+i] = param
	}
	return newParams
}

// Filterf adds the context in from of the format string.
func (f ContextFormatFilter) Filterf(format string, params ...interface{}) (newFormat string, newParams []interface{}) {
	newFormat = ""
	for _, param := range f.Context {
		newFormat += param + " "
	}
	newFormat += format
	newParams = params
	return
}

// initLoggerFromBytes initializes the logger using the specified configuration as bytes.
func initLoggerFromBytes(seelogConfig []byte) (logger T) {
	var seelogger seelog.LoggerInterface
	var err error
	if seelogger, err = seelog.LoggerFromConfigAsBytes(seelogConfig); err != nil {
		fmt.Println("Error parsing logger config:", err)
		return nil
	}
	seelogDefault = seelogger
	return withContext(seelogDefault)
}
