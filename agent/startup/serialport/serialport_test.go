// Copyright 2026 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package serialport

import (
	"errors"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/stretchr/testify/assert"
)

// stubEmitDeps replaces the EC2 check and the port opener, restoring them when the test ends.
// It returns a pointer to the number of times the port opener was called.
func stubEmitDeps(t *testing.T, isEC2 bool, openErr error) *int {
	origIsEC2, origOpen := isEC2Func, newSerialPortWithRetryFunc
	t.Cleanup(func() {
		isEC2Func = origIsEC2
		newSerialPortWithRetryFunc = origOpen
	})

	opens := 0
	isEC2Func = func(log.T) bool { return isEC2 }
	newSerialPortWithRetryFunc = func(log.T) (*SerialPort, error) {
		opens++
		return nil, openErr
	}
	return &opens
}

func TestEmitSerialPortMessage_NotEC2_DoesNotOpenPort(t *testing.T) {
	opens := stubEmitDeps(t, false, nil)

	EmitSerialPortMessage(logmocks.NewMockLog(), "test message")

	assert.Equal(t, 0, *opens, "serial port must not be opened outside EC2")
}

func TestEmitSerialPortMessage_EC2_OpensPort(t *testing.T) {
	opens := stubEmitDeps(t, true, errors.New("port unavailable"))

	EmitSerialPortMessage(logmocks.NewMockLog(), "test message")

	assert.Equal(t, 1, *opens, "serial port must be opened in EC2")
}
