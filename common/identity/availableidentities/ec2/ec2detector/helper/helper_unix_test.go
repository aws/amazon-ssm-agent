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

//go:build freebsd || linux || netbsd || openbsd
// +build freebsd linux netbsd openbsd

package helper

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type returnFromCommand struct {
	str string
	err error
}

func TestReadFile(t *testing.T) {
	var obj detectorHelper
	var returnThis returnFromCommand

	readFile = func(string) ([]byte, error) {
		return []byte(returnThis.str), returnThis.err
	}

	returnThis.str, returnThis.err = "", nil
	assert.Equal(t, "", obj.GetSystemInfo(""))

	returnThis.str, returnThis.err = "something", fmt.Errorf("file not exist")
	assert.Equal(t, "", obj.GetSystemInfo(""))

	returnThis.str, returnThis.err = "  something \n\n\t ", nil
	assert.Equal(t, "something", obj.GetSystemInfo(""))
}
