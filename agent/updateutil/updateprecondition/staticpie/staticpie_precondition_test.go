// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package staticpieprecondition

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/stretchr/testify/assert"
)

func TestGetPreconditionName(t *testing.T) {
	obj := &staticpiePrecondition{}
	assert.Equal(t, preconditionName, obj.GetPreconditionName())
}

func TestCheckPrecondition_FailVersionCompare(t *testing.T) {
	obj := &staticpiePrecondition{
		context: context.NewMockDefault(),
	}

	err := obj.CheckPrecondition("$#!.$#!.$#!")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Failed kernel version precondition: Invalid version string $#!.$#!.$#!")
}

func TestCheckPrecondition_SkipPrecondition(t *testing.T) {
	obj := &staticpiePrecondition{
		context: context.NewMockDefault(),
	}

	err := obj.CheckPrecondition("3.0.9999.9999")
	assert.NoError(t, err)
}

func TestCheckPrecondition_InvalidKernel(t *testing.T) {
	tmpFunc := hasValidKernelVersion
	hasValidKernelVersion = func() error { return fmt.Errorf("SomeError") }
	defer func() { hasValidKernelVersion = tmpFunc }()
	obj := &staticpiePrecondition{
		context: context.NewMockDefault(),
	}

	err := obj.CheckPrecondition("3.1.0.0")
	assert.Error(t, err)
	assert.Equal(t, "Failed kernel version precondition: SomeError", err.Error())
}

func TestCheckPrecondition_Success(t *testing.T) {
	tmpFunc := hasValidKernelVersion
	hasValidKernelVersion = func() error { return nil }
	defer func() { hasValidKernelVersion = tmpFunc }()
	obj := &staticpiePrecondition{
		context: context.NewMockDefault(),
	}

	err := obj.CheckPrecondition("3.1.0.0")
	assert.NoError(t, err)
}
