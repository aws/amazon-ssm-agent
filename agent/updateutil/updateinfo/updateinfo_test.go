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

package updateinfo

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	"github.com/stretchr/testify/assert"
)

type testUpdateInfo struct {
	platformName          string
	platformNameErr       error
	platformVersion       string
	platformVersionErr    error
	expectedPlatformName  string
	expectedInstallerName string
	expectingError        bool
}

func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestExecCommandHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func TestCreateInstanceContext(t *testing.T) {
	testCases := []testUpdateInfo{
		{updateconstants.PlatformAmazonLinux, nil, "2015.9", nil, updateconstants.PlatformLinux, updateconstants.PlatformLinux, false},
		{updateconstants.PlatformCentOS, nil, "7.1", nil, updateconstants.PlatformCentOS, updateconstants.PlatformLinux, false},
		{updateconstants.PlatformSuseOS, nil, "12", nil, updateconstants.PlatformSuseOS, updateconstants.PlatformLinux, false},
		{updateconstants.PlatformRedHat, nil, "6.8", nil, updateconstants.PlatformRedHat, updateconstants.PlatformLinux, false},
		{updateconstants.PlatformOracleLinux, nil, "7.7", nil, updateconstants.PlatformOracleLinux, updateconstants.PlatformLinux, false},
		{updateconstants.PlatformUbuntu, nil, "12", nil, updateconstants.PlatformUbuntu, updateconstants.PlatformUbuntu, false},
		{updateconstants.PlatformWindows, nil, "5", nil, updateconstants.PlatformWindows, updateconstants.PlatformWindows, false},
		{updateconstants.PlatformMacOsX, nil, "10.14.2", nil, updateconstants.PlatformMacOsX, updateconstants.PlatformDarwin, false},
		{"", fmt.Errorf("error"), "", nil, "", "", true},
		{"", nil, "", fmt.Errorf("error"), "", "", true},
	}

	getPlatformName = PlatformNameStub
	getPlatformVersion = PlatformVersionStub

	for _, test := range testCases {
		fmt.Printf("Test platform name: %s\n", test.platformName)
		// Setup stubs
		testInstanceInfo = test

		contextMock := &context.Mock{}
		contextMock.On("Log").Return(log.NewMockLog())

		info, err := newInner(contextMock)

		if test.expectingError {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, info.GetPlatform(), test.expectedPlatformName)
		}
	}
}

func TestGenerateCompressedFileName(t *testing.T) {
	testCases := []struct {
		obj              updateInfoImpl
		packageName      string
		expectedFileName string
	}{
		{updateInfoImpl{platform: "linux", downloadPlatformOverride: "", arch: "someArch", compressFormat: "someExt"}, "packageName1", "packageName1-linux-someArch.someExt"},
		{updateInfoImpl{platform: updateconstants.PlatformUbuntu, downloadPlatformOverride: "", arch: "someArch", compressFormat: "someExt"}, "packageName2", "packageName2-ubuntu-someArch.someExt"},
		{updateInfoImpl{platform: updateconstants.PlatformMacOsX, downloadPlatformOverride: "darwin", arch: "someArch", compressFormat: "someExt"}, "packageName3", "packageName3-darwin-someArch.someExt"},
		{updateInfoImpl{platform: updateconstants.PlatformLinux, downloadPlatformOverride: "darwin", arch: "someArch", compressFormat: "someExt"}, "packageName4", "packageName4-darwin-someArch.someExt"},
	}

	for _, test := range testCases {
		assert.Equal(t, test.expectedFileName, test.obj.GenerateCompressedFileName(test.packageName))
	}
}

var testInstanceInfo testUpdateInfo

func PlatformVersionStub(log log.T) (version string, err error) {
	return testInstanceInfo.platformVersion, testInstanceInfo.platformVersionErr
}
func PlatformNameStub(log log.T) (name string, err error) {
	return testInstanceInfo.platformName, testInstanceInfo.platformNameErr
}

func TestFileNameConstruction(t *testing.T) {
	contextMock := &context.Mock{}

	testCases := []struct {
		info   updateInfoImpl
		result string
	}{
		{updateInfoImpl{contextMock, "linux", "2015.9", "linux", "amd64", "tar.gz", "installer", "uninstaller"}, "amazon-ssm-agent-linux-amd64.tar.gz"},
		{updateInfoImpl{contextMock, "linux", "2015.9", "linux", "386", "tar.gz", "installer", "uninstaller"}, "amazon-ssm-agent-linux-386.tar.gz"},
		{updateInfoImpl{contextMock, "ubuntu", "12", "ubuntu", "386", "tar.gz", "installer", "uninstaller"}, "amazon-ssm-agent-ubuntu-386.tar.gz"},
		{updateInfoImpl{contextMock, "max os x", "10.14.2", "darwin", "amd64", "tar.gz", "installer", "uninstaller"}, "amazon-ssm-agent-darwin-amd64.tar.gz"},
	}

	for _, test := range testCases {
		fileNameResult := test.info.GenerateCompressedFileName("amazon-ssm-agent")
		assert.Equal(t, fileNameResult, test.result)
	}
}

func TestIsPlatformUsingSystemD(t *testing.T) {
	contextMock := &context.Mock{}

	testCases := []struct {
		context updateInfoImpl
		result  bool
	}{
		{updateInfoImpl{contextMock, updateconstants.PlatformRedHat, "6.5", "linux", "amd64", "tar.gz", "installer", "uninstaller"}, false},
		{updateInfoImpl{contextMock, updateconstants.PlatformRedHat, "7.0", "linux", "amd64", "tar.gz", "installer", "uninstaller"}, true},
		{updateInfoImpl{contextMock, updateconstants.PlatformOracleLinux, "7.7", "linux", "amd64", "tar.gz", "installer", "uninstaller"}, true},
		{updateInfoImpl{contextMock, updateconstants.PlatformOracleLinux, "6.10", "linux", "amd64", "tar.gz", "installer", "uninstaller"}, false},
		{updateInfoImpl{contextMock, updateconstants.PlatformCentOS, "6.1", "linux", "amd64", "tar.gz", "installer", "uninstaller"}, false},
		{updateInfoImpl{contextMock, updateconstants.PlatformSuseOS, "12", "linux", "amd64", "tar.gz", "installer", "uninstaller"}, true},
		{updateInfoImpl{contextMock, updateconstants.PlatformCentOS, "7", "linux", "amd64", "tar.gz", "installer", "uninstaller"}, true},
	}

	for _, test := range testCases {
		result, err := test.context.IsPlatformUsingSystemD()
		assert.NoError(t, err)
		assert.Equal(t, result, test.result)
	}
}

func TestIsPlatformUsingSystemDWithInvalidVersionNumber(t *testing.T) {
	contextMock := &context.Mock{}

	testCases := []struct {
		info   updateInfoImpl
		result bool
	}{
		{updateInfoImpl{contextMock, updateconstants.PlatformRedHat, "wrong version", "linux", "amd64", "tar.gz", "installer", "uninstaller"}, false},
	}

	for _, test := range testCases {
		_, err := test.info.IsPlatformUsingSystemD()
		assert.Error(t, err)
	}
}

func TestIsPlatformUsingSystemDWithPossiblyUsingSystemD(t *testing.T) {
	contextMock := &context.Mock{}

	testCases := []struct {
		context updateInfoImpl
		result  bool
	}{
		{updateInfoImpl{contextMock, updateconstants.PlatformRaspbian, "8", "linux", "amd64", "tar.gz", "installer", "uninstaller"}, true},
	}

	// Stub exec.Command
	execCommand = fakeExecCommand

	for _, test := range testCases {
		result, err := test.context.IsPlatformUsingSystemD()
		assert.NoError(t, err)
		assert.Equal(t, result, test.result)
	}
}
