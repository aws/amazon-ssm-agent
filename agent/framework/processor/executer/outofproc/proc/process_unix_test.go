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
	"fmt"
	"testing"

	"src/github.com/stretchr/testify/assert"
)

//Output format is verified to be identical on RHEL, CENTOS, UBUNTU, AL. However darwin has a different time format
//TODO add darwin compile once we support darwin
func TestFindProcess(t *testing.T) {
	testOutput := "  PID  STARTED\n" +
		"6611 10:23:32\n" +
		"20440 14:01:38\n" +
		"20441 14:01:40\n"
	ps = func() ([]byte, error) {
		return []byte(testOutput), nil
	}
	testPidExist := 6611
	testPidExistTime := "10:23:32"
	fmt.Println(get_current_time())
	testPidNonExist := 10000
	exists, err := find_process(testPidExist, testPidExistTime)
	assert.NoError(t, err)
	assert.True(t, exists)
	exists, err = find_process(testPidNonExist, testPidExistTime)
	assert.NoError(t, err)
	assert.False(t, exists)
	exists, err = find_process(testPidExist, "00:00:00")
	assert.NoError(t, err)
	assert.False(t, exists)

}
