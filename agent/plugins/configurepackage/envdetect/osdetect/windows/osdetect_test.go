package windows

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"

	c "github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect/constants"
	"github.com/stretchr/testify/assert"
)

func TestDetectPkgManager(t *testing.T) {
	d := Detector{}
	result, err := d.DetectPkgManager("", "", "") // parameters only matter for linux

	assert.NoError(t, err)
	assert.Equal(t, c.PackageManagerWindows, result)
}

func TestDetectInitSystem(t *testing.T) {
	d := Detector{}
	result, err := d.DetectInitSystem()

	assert.NoError(t, err)
	assert.Equal(t, c.InitWindows, result)
}

func TestDetectPlatformDetails(t *testing.T) {
	data := []struct {
		name            string
		wmioutput       string
		expectedVersion string
		expectError     bool
	}{
		{
			"Microsoft(R) Windows(R) Server 2003 Datacenter x64 Edition",
			`

BootDevice=\Device\HarddiskVolume1
BuildNumber=3790
BuildType=Multiprocessor Free
Caption=Microsoft(R) Windows(R) Server 2003 Datacenter x64 Edition
CodeSet=1252
CountryCode=1
CreationClassName=Win32_OperatingSystem
CSCreationClassName=Win32_ComputerSystem
CSDVersion=Service Pack 2
CSName=AMAZON-901389D0
CurrentTimeZone=0
DataExecutionPrevention_32BitApplications=TRUE
DataExecutionPrevention_Available=TRUE
DataExecutionPrevention_Drivers=TRUE
DataExecutionPrevention_SupportPolicy=3
Debug=FALSE
Description=
Distributed=FALSE
EncryptionLevel=256
ForegroundApplicationBoost=0
FreePhysicalMemory=15747592
FreeSpaceInPagingFiles=518624
FreeVirtualMemory=16201920
InstallDate=20171019094403.000000+000
LargeSystemCache=1
LastBootUpTime=20171019094424.510250+000
LocalDateTime=20171019114056.709000+000
Locale=0409
Manufacturer=Microsoft Corporation
MaxNumberOfProcesses=-1
MaxProcessMemorySize=8589934464
Name=Microsoft Windows Server 2003 R2 Datacenter x64 Edition|C:\WINDOWS|\Device\
Harddisk0\Partition1
NumberOfLicensedUsers=5
NumberOfProcesses=45
NumberOfUsers=3
Organization=AMAZON
OSLanguage=1033
OSProductSuite=402
OSType=18
OtherTypeDescription=R2
PAEEnabled=
PlusProductID=
PlusVersionNumber=
Primary=TRUE
ProductType=3
QuantumLength=0
QuantumType=0
RegisteredUser=AMAZON
SerialNumber=76871-644-6528304-50084
ServicePackMajorVersion=2
ServicePackMinorVersion=0
SizeStoredInPagingFiles=524288
Status=OK
SuiteMask=402
SystemDevice=\Device\HarddiskVolume1
SystemDirectory=C:\WINDOWS\system32
SystemDrive=C:
TotalSwapSpaceSize=
TotalVirtualMemorySize=16691380
TotalVisibleMemorySize=16776488
Version=5.2.3790
WindowsDirectory=C:\WINDOWS

`,
			"5.2.3790",
			false,
		},
		{
			"Microsoft Windows Server 2016 Datacenter Nano",
			`BootDevice                    = \Device\HarddiskVolume1
BuildNumber                   = 14393
BuildType                     = Multiprocessor Free
Caption                       = Microsoft Windows Server 2016 Datacenter
CodeSet                       = 1252
CountryCode                   = 1
CreationClassName             = Win32_OperatingSystem
CSCreationClassName           = Win32_ComputerSystem
CSDVersion                    =
CurrentTimeZone               = 0
DataExecutionPrevention_32BitApplications= TRUE
DataExecutionPrevention_Available= TRUE
DataExecutionPrevention_Drivers= TRUE
DataExecutionPrevention_SupportPolicy= 2
Debug                         =
Distributed                   = FALSE
EncryptionLevel               =
FreeSpaceInPagingFiles        = 1048316
FreeVirtualMemory             = 1885364
InstallDate                   = 20171018143126.000000+000
LargeSystemCache              =
LocalDateTime                 = 20171019121946.074000+000
Locale                        = 0409
Manufacturer                  = Microsoft Corporation
MaxNumberOfProcesses          = 4294967295
MaxProcessMemorySize          = 137438953344
Name                          = Microsoft Windows Server 2016 Datacenter|C:\Windows|\Device\Harddisk0\Partition1
NumberOfLicensedUsers         = 0
NumberOfProcesses             = 21
NumberOfUsers                 = 0
OperatingSystemSKU            = 143
Organization                  = Amazon
OSArchitecture                = 64-bit
OSLanguage                    = 1033
OSProductSuite                = 272
OSType                        = 18
OtherTypeDescription          =
Primary                       = TRUE
ProductType                   = 3
RegisteredUser                = Amazon
SerialNumber                  =
ServicePackMinorVersion       = 0
SizeStoredInPagingFiles       = 1048316
Status                        = OK
SuiteMask                     = 272
SystemDevice                  = \Device\HarddiskVolume1
SystemDirectory               = C:\Windows\system32
SystemDrive                   = C:
TotalSwapSpaceSize            =
TotalVisibleMemorySize        = 1048176
Version                       = 10.0.14393
WindowsDirectory              = C:\Windows
`,
			"10.0.14393nano",
			false,
		},
		{
			"missing OperatingSystemSKU",
			"Version=10.0.14393",
			"10.0.14393",
			false,
		},
		{
			"missing version",
			"OperatingSystemSKU = 143",
			"",
			true,
		},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			_, resultVersion, _, err := detectPlatformDetails(log.NewMockLog(), d.wmioutput)

			if d.expectError {
				assert.True(t, err != nil, "error expected")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, d.expectedVersion, resultVersion)
			}
		})
	}
}

func TestParseVersion(t *testing.T) {
	data := []struct {
		name            string
		wmioutput       string
		expectedVersion string
		expectError     bool
	}{
		{
			"simple single line version",
			"Version=10.0.14393",
			"10.0.14393",
			false,
		},
		{
			"simple multiline line version",
			"Version=10.0.14393\n",
			"10.0.14393",
			false,
		},
		{
			"whitespace version",
			"  \t Version  \t  = \t  10.0.14393  \t",
			"10.0.14393",
			false,
		},
		{
			"multiple version",
			"CdVersion=342\nVersion=10.0.14393",
			"10.0.14393",
			false,
		},
		{
			"windows newline",
			"\r\nVersion=10.0.14393\r\n",
			"10.0.14393",
			false,
		},
		{
			"empty input",
			"",
			"",
			true,
		},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			resultVersion, err := parseVersion(d.wmioutput)

			if d.expectError {
				assert.True(t, err != nil, "error expected")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, d.expectedVersion, resultVersion)
			}
		})
	}
}

func TestParseOperatingSystemSKU(t *testing.T) {
	data := []struct {
		name        string
		wmioutput   string
		expectedSKU string
		expectError bool
	}{
		{
			"simple single line SKU",
			"OperatingSystemSKU=7",
			"7",
			false,
		},
		{
			"simple multiline line SKU",
			"OperatingSystemSKU=7\n",
			"7",
			false,
		},
		{
			"whitespace SKU",
			"  \t OperatingSystemSKU  \t  = \t  7  \t",
			"7",
			false,
		},
		{
			"multiple SKU",
			"SomeOtherOperatingSystemSKU=143\nOperatingSystemSKU=7",
			"7",
			false,
		},
		{
			"windows newline",
			"\r\nOperatingSystemSKU=7\r\n",
			"7",
			false,
		},
		{
			"empty input",
			"",
			"",
			true,
		},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			resultSKU, err := parseOperatingSystemSKU(d.wmioutput)

			if d.expectError {
				assert.True(t, err != nil, "error expected")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, d.expectedSKU, resultSKU)
			}
		})
	}
}
