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

func TestReadSystemProductInfo(t *testing.T) {
	var obj detectorHelper
	var returnThis returnFromCommand

	execCommand = func(string, ...string) (string, error) {
		return returnThis.str, returnThis.err
	}

	returnThis.str, returnThis.err = "", nil
	assert.Equal(t, "", obj.GetSystemInfo(""))

	returnThis.str, returnThis.err = "UUID                          = EC2493C6-586B-1C89-DD8E-EB2F21F5E50D", nil
	assert.Equal(t, "EC2493C6-586B-1C89-DD8E-EB2F21F5E50D", obj.GetSystemInfo(""))

	returnThis.str, returnThis.err = "UUID =                 EC2493C6-586B-1C89-DD8E-EB2F21F5E50D", nil
	assert.Equal(t, "EC2493C6-586B-1C89-DD8E-EB2F21F5E50D", obj.GetSystemInfo(""))

	returnThis.str, returnThis.err = "UUID = EC2493C6-586B-1C89-DD8E-EB2F21F5E50D", nil
	assert.Equal(t, "EC2493C6-586B-1C89-DD8E-EB2F21F5E50D", obj.GetSystemInfo(""))

	returnThis.str, returnThis.err = "UUID \r\n EC2493C6-586B-1C89-DD8E-EB2F21F5E50D \r\n", nil
	assert.Equal(t, "EC2493C6-586B-1C89-DD8E-EB2F21F5E50D", obj.GetSystemInfo(""))

	returnThis.str, returnThis.err = "somerandomsingleline", nil
	assert.Equal(t, "", obj.GetSystemInfo(""))
}
