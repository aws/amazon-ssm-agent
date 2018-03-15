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

// Package log is used to initialize the logger. This package should be imported once.
package log

import (
	"github.com/cihub/seelog"
)

// GetLogger creates a new logger with the provided config and returns it
func GetLogger(log T, seelogConfig string) (logger seelog.LoggerInterface) {
	logger, err := seelog.LoggerFromConfigAsBytes([]byte(seelogConfig))
	if err != nil {
		logger, _ = seelog.LoggerFromConfigAsBytes(DefaultConfig())
	}
	defer logger.Flush()

	err = seelog.ReplaceLogger(logger)
	if err != nil {
		log.Debug("Error is %v", err.Error())
	}
	return logger
}
