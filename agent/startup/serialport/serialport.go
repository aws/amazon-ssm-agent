// Copyright 2025 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
//
//go:build linux || windows
// +build linux windows

// Package serialport implements serial port capabilities
package serialport

import (
	"runtime/debug"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

// attempt to initialize and open the serial port.
// since only three minute is allowed to write logs to console during boot,
// it attempts to open serial port for approximately three minutes.
func NewSerialPortWithRetry(log log.T) (sp *SerialPort, err error) {
	retryCount := 0
	for retryCount < serialPortRetryMaxCount {
		sp = NewSerialPort(log)
		if err = sp.OpenPort(); err != nil {
			log.Errorf("%v. Retrying in %v seconds...", err.Error(), serialPortRetryWaitTime)
			time.Sleep(serialPortRetryWaitTime * time.Second)
			retryCount++
		} else {
			break
		}

		// if the retry count hits the maximum count, log the error and return.
		if retryCount == serialPortRetryMaxCount {
			return nil, err
		}
	}
	return sp, nil
}

// Replaceable in unit tests.
var (
	isEC2Func                  = IsEC2Environment
	newSerialPortWithRetryFunc = NewSerialPortWithRetry
)

// EmitSerialPortMessage sends a message to the serial port.
// This function is intended to be used by components that need to log to the serial port.
// The message is only written when running in EC2; on other hosts it is skipped without
// opening the port.
func EmitSerialPortMessage(log log.T, msg string) {
	// Create serial port specific logger context
	serialLog := log.WithContext("[SerialPort]")

	defer func() {
		if r := recover(); r != nil {
			serialLog.Errorf("Serial port message panic: %v", r)
			serialLog.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()

	if !isEC2Func(serialLog) {
		serialLog.Debugf("Not running in EC2, skipping serial port message: %s", msg)
		return
	}

	sp, err := newSerialPortWithRetryFunc(serialLog)
	if err != nil {
		serialLog.Errorf("Error occurred while opening serial port: %v", err.Error())
		return
	}

	defer func() {
		sp.ClosePort()
	}()

	sp.WritePort(msg)
}
