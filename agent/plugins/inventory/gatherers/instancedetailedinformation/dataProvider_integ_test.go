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

// +build integration

package instancedetailedinformation

import (
	"runtime"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/stretchr/testify/assert"
)

func TestCollectPlatformDependentInstanceDataInteg(t *testing.T) {
	cmdExecutor = executeCommand
	mockContext := context.NewMockDefault()
	items := collectPlatformDependentInstanceData(mockContext)
	assert.Equal(t, 1, len(items))
	for _, item := range items {
		assert.NotEmpty(t, item.CPUSpeedMHz)
		assert.NotEmpty(t, item.CPUs)
		assert.NotEmpty(t, item.CPUSockets)
		assert.NotEmpty(t, item.CPUCores)
		assert.NotEmpty(t, item.CPUHyperThreadEnabled)
		if runtime.GOOS != "linux" {
			// CPU Model is not always reported by lscpu - most notably on dev desktop
			assert.NotEmpty(t, item.CPUModel)
		}
	}
}
