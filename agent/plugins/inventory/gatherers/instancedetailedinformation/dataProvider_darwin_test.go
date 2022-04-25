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

//go:build darwin
// +build darwin

package instancedetailedinformation

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/stretchr/testify/assert"
)

var (
	sampleDataMac = []string{
		`machdep.cpu.brand_string: Intel(R) Core(TM) i7-8569U CPU @ 2.80GHz
hw.physicalcpu: 4
hw.logicalcpu: 8
hw.cpufrequency: 2800000000
hw.cputhreadtype: 1
kern.osrelease: 20.6.0`,
	}
)

var sampleDataMacParsed = []model.InstanceDetailedInformation{
	{
		CPUModel:              "Intel(R) Core(TM) i7-8569U CPU @ 2.80GHz",
		CPUSpeedMHz:           "2800",
		CPUs:                  "8",
		CPUSockets:            "",
		CPUCores:              "4",
		CPUHyperThreadEnabled: "true",
		KernelVersion:         "20.6.0",
	},
}

func TestParseSysctlOutput(t *testing.T) {
	for i, sampleData := range sampleDataMac {
		parsedItems := parseSysctlOutput(sampleData)
		assert.Equal(t, len(parsedItems), 1)
		assert.Equal(t, sampleDataMacParsed[i], parsedItems[0])
	}
}

func TestCollectPlatformDependentInstanceData(t *testing.T) {
	mockContext := context.NewMockDefault()
	for i, sampleData := range sampleDataMac {
		cmdExecutor = createMockExecutor(sampleData)
		parsedItems := collectPlatformDependentInstanceData(mockContext)
		assert.Equal(t, len(parsedItems), 1)
		assert.Equal(t, sampleDataMacParsed[i], parsedItems[0])
	}
}

func TestCollectPlatformDependentInstanceDataWithSysctlError(t *testing.T) {
	mockContext := context.NewMockDefault()
	cmdExecutor = createMockExecutorWithErrorOnNthExecution(1)
	parsedItems := collectPlatformDependentInstanceData(mockContext)
	assert.Equal(t, len(parsedItems), 0)
}
