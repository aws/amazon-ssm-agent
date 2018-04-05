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

var loadedLogger T
var PkgMutex = new(sync.Mutex)

func DefaultLogger() T {
	if loadedLogger == nil {
		fmt.Println("Initializing new default seelog logger")
		seelogger, _ := seelog.LoggerFromConfigAsBytes(DefaultConfig())
		seelogger.SetAdditionalStackDepth(1)

		loggerInstance := &DelegateLogger{}
		loggerInstance.BaseLoggerInstance = seelogger

		formatFilter := &ContextFormatFilter{Context: []string{}}
		loadedLogger = &Wrapper{Format: formatFilter, M: PkgMutex, Delegate: loggerInstance}
		fmt.Println("Initialized new default seelog logger")
	}
	return loadedLogger
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

func GetLogConfigBytes() []byte {
	return getLogConfigBytes()
}
