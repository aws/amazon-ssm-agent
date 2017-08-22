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

// +build darwin freebsd linux netbsd openbsd

// Package process wraps up the os.Process interface and also provides os-specific process lookup functions
package proc

import (
	"testing"

	"time"

	"os/exec"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

var logger = log.NewMockLog()

//TODO add process start time
func TestIsProcessExists(t *testing.T) {
	cmd := exec.Command("sleep", "10")
	err := cmd.Start()
	//do not call wait in case the process are recycled
	assert.NoError(t, err)
	pid := cmd.Process.Pid
	logger.Infof("process pid: %v", pid)
	assert.True(t, IsProcessExists(logger, pid, time.Now()))
}

//Output format is verified to be identical on RHEL, CENTOS, UBUNTU, AL. However darwin has a different time format
func TestFindProcess(t *testing.T) {
	testInput := " PID STARTED" + "\n" +
		"2598 Fri Aug  4 11:39:23 2017" + "\n" +
		"2600 Fri Aug  4 11:39:23 2017" + "\n" +
		"16198 Fri Aug 18 15:28:01 2017" + "\n" +
		"54770 Mon Aug  7 17:39:34 2017" + "\n" +
		"2608 Fri Aug  4 11:39:33 2017" + "\n" +
		"2610 Fri Aug  4 11:39:33 2017" + "\n" +
		"49380 Mon Aug  7 16:29:09 2017" + "\n" +
		"49382 Mon Aug  7 16:29:10 2017" + "\n" +
		"16179 Fri Aug 18 15:26:15 2017" + "\n" +
		"49394 Mon Aug  7 16:29:19 2017"
	ps = func() ([]byte, error) {
		return []byte(testInput), nil
	}
	testPidExist := 2598
	//TODO add time to the overall result
	testPidExistTime := time.Now()
	testPidNonExist := 10000
	exists, err := find_process(testPidExist, testPidExistTime)
	assert.NoError(t, err)
	assert.True(t, exists)
	exists, err = find_process(testPidNonExist, testPidExistTime)
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestCompareTime(t *testing.T) {
	testInput := "Fri Aug  4 11:39:23 2017"
	testTime := time.Date(2017, 8, 4, 11, 39, 23, 10000, time.UTC)
	assert.True(t, compareTimes(testTime, testInput))
}
