// Copyright 2022 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package helper

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type returnFromCommand struct {
	str string
	err error
}

func getOutputString(uuid, vendor, version string) string {
	return "SMBIOSBIOSVersion : " + version +
		"\r\nManufacturer      : " + vendor +
		"\r\nName              : Revision: 1.221" +
		"\r\nSerialNumber      : " + uuid +
		"\r\nVersion           : Xen - 0"
}

func TestReadSystemProductInfo(t *testing.T) {
	var obj detectorHelper
	var returnThis returnFromCommand

	oldExecCommand := execCommand
	defer func() { execCommand = oldExecCommand }()

	var called int
	execCommand = func(string, ...string) (string, error) {
		called++
		return returnThis.str, returnThis.err
	}

	returnThis.str, returnThis.err = getOutputString("uuid123", "vendor123", "version123"), nil

	assert.Equal(t, "version123", obj.GetSystemInfo("SMBIOSBIOSVersion"))
	assert.Equal(t, "uuid123", obj.GetSystemInfo("SerialNumber"))
	assert.Equal(t, "vendor123", obj.GetSystemInfo("Manufacturer"))
	assert.Equal(t, 1, called)
}

func TestReadSystemProductInfo_CacheMiss(t *testing.T) {
	var obj detectorHelper
	var returnThis returnFromCommand

	oldExecCommand := execCommand
	defer func() { execCommand = oldExecCommand }()

	var called int
	execCommand = func(string, ...string) (string, error) {
		called++
		return returnThis.str, returnThis.err
	}

	returnThis.str, returnThis.err = getOutputString("uuid123", "vendor123", "version123"), nil

	assert.Equal(t, "version123", obj.GetSystemInfo("SMBIOSBIOSVersion"))
	assert.Equal(t, "uuid123", obj.GetSystemInfo("SerialNumber"))
	assert.Equal(t, "vendor123", obj.GetSystemInfo("Manufacturer"))
	assert.Equal(t, 1, called)

	obj.GetSystemInfo("NonExistentAttribute")
	assert.Equal(t, 2, called)

	assert.Equal(t, "version123", obj.GetSystemInfo("SMBIOSBIOSVersion"))
	assert.Equal(t, "uuid123", obj.GetSystemInfo("SerialNumber"))
	assert.Equal(t, "vendor123", obj.GetSystemInfo("Manufacturer"))
	assert.Equal(t, 2, called)

}
