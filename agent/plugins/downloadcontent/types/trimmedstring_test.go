/*
 * Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"). You may not
 * use this file except in compliance with the License. A copy of the
 * License is located at
 *
 * http://aws.amazon.com/apache2.0/
 *
 * or in the "license" file accompanying this file. This file is distributed
 * on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing
 * permissions and limitations under the License.
 */

package types

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/stretchr/testify/assert"
)

func TestNewTrimmedString_NewTrimmedString(t *testing.T) {
	trimmedString := NewTrimmedString("   test a b  ")
	assert.Equal(t, "test a b", trimmedString.Val())
}

type TestStruct struct {
	Attr TrimmedString `json:"attr"`
}

func TestNewTrimmedString_UnmarshalJson(t *testing.T) {
	json := `{
		"attr": " attr val  "
	}`

	testStruct := TestStruct{}

	err := jsonutil.Unmarshal(json, &testStruct)
	assert.NoError(t, err)
	assert.Equal(t, "attr val", testStruct.Attr.Val())
}
