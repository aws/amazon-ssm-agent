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

//go:build windows
// +build windows

package instancedetailedinformation

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/stretchr/testify/assert"
)

var (
	sampleDataWindows = [][]string{
		{
			// Windows Server 2016 c4.8xlarge
			`{"CPUModel":"Intel(R) Xeon(R) CPU E5-2666 v3 @ 2.90GHz","CPUSpeedMHz":"2900","CPUs":"36","CPUSockets":"2","CPUCores":"18","CPUHyperThreadEnabled":"true"}`,
			`{"OSServicePack":"0"}`,
			`{"KernelVersion":"10.0.20348.643 (WinBuild.160101.0800)"}`,
		},
		{
			// Windows Server 2016 t2.2xlarge
			`{"CPUModel":"Intel(R) Xeon(R) CPU E5-2676 v3 @ 2.40GHz","CPUSpeedMHz":"2395","CPUs":"8","CPUSockets":"1","CPUCores":"8","CPUHyperThreadEnabled":"false"}`,
			`{"OSServicePack":"0"}`,
			`{"KernelVersion":"10.0.20348.643 (WinBuild.160101.0800)"}`,
		},
		{
			// Windows Server 2003 R2 t2.2xlarge
			`{"CPUModel":"Intel(R) Xeon(R) CPU E5-2676 v3 @ 2.40GHz","CPUSpeedMHz":"2395","CPUs":"8","CPUSockets":"8","CPUCores":"8","CPUHyperThreadEnabled":"false"}`,
			`{"OSServicePack":"2"}`,
			`{"KernelVersion":"5.2.9600.20302 (winblue_ltsb_escrow.110222-1158)"}`,
		},
		{
			// Windows Server 2008 R2 SP1 m4.16xlarge
			`{"CPUModel":"Intel(R) Xeon(R) CPU E5-2686 v4 @ 2.30GHz","CPUSpeedMHz":"2301","CPUs":"64","CPUSockets":"2","CPUCores":"32","CPUHyperThreadEnabled":"true"}`,
			`{"OSServicePack":"1"}`,
			`{"KernelVersion":"6.0.9600.20302 (winblue_ltsb_escrow.110222-1157)"}`,
		},
	}
)

var sampleDataWindowsParsed = []model.InstanceDetailedInformation{
	{
		CPUModel:              "Intel(R) Xeon(R) CPU E5-2666 v3 @ 2.90GHz",
		CPUSpeedMHz:           "2900",
		CPUs:                  "36",
		CPUSockets:            "2",
		CPUCores:              "18",
		CPUHyperThreadEnabled: "true",
		OSServicePack:         "0",
		KernelVersion:         "10.0.20348.643 (WinBuild.160101.0800)",
	},
	{
		CPUModel:              "Intel(R) Xeon(R) CPU E5-2676 v3 @ 2.40GHz",
		CPUSpeedMHz:           "2395",
		CPUs:                  "8",
		CPUSockets:            "1",
		CPUCores:              "8",
		CPUHyperThreadEnabled: "false",
		OSServicePack:         "0",
		KernelVersion:         "10.0.20348.643 (WinBuild.160101.0800)",
	},
	{
		CPUModel:              "Intel(R) Xeon(R) CPU E5-2676 v3 @ 2.40GHz",
		CPUSpeedMHz:           "2395",
		CPUs:                  "8",
		CPUSockets:            "8",
		CPUCores:              "8",
		CPUHyperThreadEnabled: "false",
		OSServicePack:         "2",
		KernelVersion:         "5.2.9600.20302 (winblue_ltsb_escrow.110222-1158)",
	},
	{
		CPUModel:              "Intel(R) Xeon(R) CPU E5-2686 v4 @ 2.30GHz",
		CPUSpeedMHz:           "2301",
		CPUs:                  "64",
		CPUSockets:            "2",
		CPUCores:              "32",
		CPUHyperThreadEnabled: "true",
		OSServicePack:         "1",
		KernelVersion:         "6.0.9600.20302 (winblue_ltsb_escrow.110222-1157)",
	},
}

func TestCollectPlatformDependentInstanceData(t *testing.T) {
	mockContext := context.NewMockDefault()
	for i, sampleDataItem := range sampleDataWindows {
		cmdExecutor = createMockExecutor(sampleDataItem...)
		parsedItems := collectPlatformDependentInstanceData(mockContext)
		assert.Equal(t, len(parsedItems), 1)
		assert.Equal(t, sampleDataWindowsParsed[i], parsedItems[0])
	}
}

func TestCollectPlatformDependentInstanceDataWithCPUInfoAndOsInfoError(t *testing.T) {
	mockContext := context.NewMockDefault()
	cmdExecutor = createMockExecutorWithErrorOnNthExecution(1)
	parsedItems := collectPlatformDependentInstanceData(mockContext)
	assert.Equal(t, len(parsedItems), 0)
}

func TestCollectPlatformDependentInstanceDataWithKernelCollectionError(t *testing.T) {
	mockContext := context.NewMockDefault()
	for i, sampleDataItem := range sampleDataWindows {
		cmdExecutor = createMockExecutorWithErrorOnNthExecution(3, sampleDataItem...)
		parsedItems := collectPlatformDependentInstanceData(mockContext)
		assert.Equal(t, len(parsedItems), 1)
		recordWithoutKernel := sampleDataWindowsParsed[i]
		recordWithoutKernel.KernelVersion = ""
		assert.Equal(t, recordWithoutKernel, parsedItems[0])
	}
}
