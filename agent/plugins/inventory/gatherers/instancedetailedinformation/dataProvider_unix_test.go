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

package instancedetailedinformation

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/stretchr/testify/assert"
)

var (
	sampleDataUnix = []string{
		// Amazon Linux XLarge
		`Architecture:          x86_64
CPU op-mode(s):        32-bit, 64-bit
Byte Order:            Little Endian
CPU(s):                64
On-line CPU(s) list:   0-63
Thread(s) per core:    2
Core(s) per socket:    16
Socket(s):             2
NUMA node(s):          2
Vendor ID:             GenuineIntel
CPU family:            6
Model:                 79
Model name:            Intel(R) Xeon(R) CPU E5-2686 v4 @ 2.30GHz
Stepping:              1
CPU MHz:               1772.167
BogoMIPS:              4661.72
Hypervisor vendor:     Xen
Virtualization type:   full
L1d cache:             32K
L1i cache:             32K
L2 cache:              256K
L3 cache:              46080K
NUMA node0 CPU(s):     0-15,32-47
NUMA node1 CPU(s):     16-31,48-63`,

		// Ubuntu
		`Architecture:          x86_64
CPU op-mode(s):        32-bit, 64-bit
Byte Order:            Little Endian
CPU(s):                2
On-line CPU(s) list:   0,1
Thread(s) per core:    1
Core(s) per socket:    2
Socket(s):             1
NUMA node(s):          1
Vendor ID:             GenuineIntel
CPU family:            6
Model:                 63
Model name:            Intel(R) Xeon(R) CPU E5-2676 v3 @ 2.40GHz
Stepping:              2
CPU MHz:               2400.072
BogoMIPS:              4800.14
Hypervisor vendor:     Xen
Virtualization type:   full
L1d cache:             32K
L1i cache:             32K
L2 cache:              256K
L3 cache:              30720K
NUMA node0 CPU(s):     0,1
Flags:                 fpu vme de pse tsc msr pae mce cx8 apic sep mtrr pge mca cmov pat pse36 clflush mmx fxsr sse sse2 ht syscall nx rdtscp lm constant_tsc rep_good nopl xtopology eagerfpu pni pclmulqdq ssse3 fma cx16 pcid sse4_1 sse4_2 x2apic movbe popcnt tsc_deadline_timer aes xsave avx f16c rdrand hypervisor lahf_lm abm fsgsbase bmi1 avx2 smep bmi2 erms invpcid xsaveopt`,

		// Suse
		`Architecture:          x86_64
CPU op-mode(s):        32-bit, 64-bit
Byte Order:            Little Endian
CPU(s):                2
On-line CPU(s) list:   0,1
Thread(s) per core:    1
Core(s) per socket:    2
Socket(s):             1
NUMA node(s):          1
Vendor ID:             GenuineIntel
CPU family:            6
Model:                 63
Model name:            Intel(R) Xeon(R) CPU E5-2676 v3 @ 2.40GHz
Stepping:              2
CPU MHz:               2394.532
BogoMIPS:              4789.06
Hypervisor vendor:     Xen
Virtualization type:   full
L1d cache:             32K
L1i cache:             32K
L2 cache:              256K
L3 cache:              30720K
NUMA node0 CPU(s):     0,1
Flags:                 fpu vme de pse tsc msr pae mce cx8 apic sep mtrr pge mca cmov pat pse36 clflush mmx fxsr sse sse2 ht syscall nx rdtscp lm constant_tsc rep_good nopl xtopology eagerfpu pni pclmulqdq ssse3 fma cx16 pcid sse4_1 sse4_2 x2apic movbe popcnt tsc_deadline_timer aes xsave avx f16c rdrand hypervisor lahf_lm abm fsgsbase bmi1 avx2 smep bmi2 erms invpcid xsaveopt`,

		// RedHat
		`Architecture:          x86_64
CPU op-mode(s):        32-bit, 64-bit
Byte Order:            Little Endian
CPU(s):                1
On-line CPU(s) list:   0
Thread(s) per core:    1
Core(s) per socket:    1
Socket(s):             1
NUMA node(s):          1
Vendor ID:             GenuineIntel
CPU family:            6
Model:                 63
Model name:            Intel(R) Xeon(R) CPU E5-2676 v3 @ 2.40GHz
Stepping:              2
CPU MHz:               2400.125
BogoMIPS:              4800.12
Hypervisor vendor:     Xen
Virtualization type:   full
L1d cache:             32K
L1i cache:             32K
L2 cache:              256K
L3 cache:              30720K
NUMA node0 CPU(s):     0`,
	}
)

var sampleDataUnixParsed = []model.InstanceDetailedInformation{
	{
		CPUModel:              "Intel(R) Xeon(R) CPU E5-2686 v4 @ 2.30GHz",
		CPUSpeedMHz:           "1772",
		CPUs:                  "64",
		CPUSockets:            "2",
		CPUCores:              "32",
		CPUHyperThreadEnabled: "true",
	},
	{
		CPUModel:              "Intel(R) Xeon(R) CPU E5-2676 v3 @ 2.40GHz",
		CPUSpeedMHz:           "2400",
		CPUs:                  "2",
		CPUSockets:            "1",
		CPUCores:              "2",
		CPUHyperThreadEnabled: "false",
	},
	{
		CPUModel:              "Intel(R) Xeon(R) CPU E5-2676 v3 @ 2.40GHz",
		CPUSpeedMHz:           "2394",
		CPUs:                  "2",
		CPUSockets:            "1",
		CPUCores:              "2",
		CPUHyperThreadEnabled: "false",
	},
	{
		CPUModel:              "Intel(R) Xeon(R) CPU E5-2676 v3 @ 2.40GHz",
		CPUSpeedMHz:           "2400",
		CPUs:                  "1",
		CPUSockets:            "1",
		CPUCores:              "1",
		CPUHyperThreadEnabled: "false",
	},
}

func TestParseLscpuOutput(t *testing.T) {
	for i, sampleData := range sampleDataUnix {
		parsedItems := parseLscpuOutput(sampleData)
		assert.Equal(t, len(parsedItems), 1)
		assert.Equal(t, sampleDataUnixParsed[i], parsedItems[0])
	}
}

func TestCollectPlatformDependentInstanceData(t *testing.T) {
	mockContext := context.NewMockDefault()
	for i, sampleData := range sampleDataUnix {
		cmdExecutor = createMockExecutor(sampleData)
		parsedItems := collectPlatformDependentInstanceData(mockContext)
		assert.Equal(t, len(parsedItems), 1)
		assert.Equal(t, sampleDataUnixParsed[i], parsedItems[0])
	}

	cmdExecutor = MockTestExecutorWithError
	parsedItems := collectPlatformDependentInstanceData(mockContext)
	assert.Equal(t, len(parsedItems), 0)
}
