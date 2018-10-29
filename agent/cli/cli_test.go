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

// Package cli represents the entry point of the ssm agent cli.
package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCliUsage(t *testing.T) {
	var buffer bytes.Buffer
	args := []string{"ssm-cli"}
	res := RunCommand(args, &buffer)
	assert.Contains(t, buffer.String(), "usage")
	assert.Equal(t, 1, res)
}
