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

// Package updateutil contains updater specific utilities.
package updateutil

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

var logger = log.NewMockLog()

type testInstanceContext struct {
	region                string
	platformName          string
	platformNameErr       error
	platformVersion       string
	platformVersionErr    error
	expectedPlatformName  string
	expectedInstallerName string
	expectingError        bool
}

// TestVersionStringCompare tests version string comparison
func TestVersionStringCompare(t *testing.T) {
	testCases := []struct {
		a      string
		b      string
		result int
	}{
		{"0", "1.0.152.0", -1},
		{"0.0.1.0", "1.0.152.0", -1},
		{"1.05.00.0156", "1.0.221.9289", 1},
		{"2.05.1", "1.3234.221.9289", 1},
		{"1", "1.0.1", -1},
		{"1.0.1", "1.0.2", -1},
		{"1.0.2", "1.0.3", -1},
		{"1.0.3", "1.1", -1},
		{"1.1", "1.1.1", -1},
		{"1.1.0", "1.0.152.0", 1},
		{"1.1.45", "1.0.152.0", 1},
		{"1.1.1", "1.1.2", -1},
		{"1.1.2", "1.2", -1},
		{"1.1.2", "1.1.2", 0},
		{"2.1.2", "2.1.2", 0},
		{"7.1", "7", 1},
	}

	for _, test := range testCases {
		compareResult, err := VersionCompare(test.a, test.b)
		assert.NoError(t, err)
		assert.Equal(t, compareResult, test.result)
	}
}

// TestVersionStringCompare tests version string comparison
func TestVersionStringCompareWithError(t *testing.T) {
	testCases := []struct {
		a string
		b string
	}{
		{"Invalid version", "1.0.152.0"},
		{"0.0.1.0", "Invalid version"},
	}

	for _, test := range testCases {
		_, err := VersionCompare(test.a, test.b)
		assert.Error(t, err)
	}
}

func TestCreateInstanceContext(t *testing.T) {
	testCases := []testInstanceContext{
		{"us-east-1", PlatformAmazonLinux, nil, "2015.9", nil, PlatformLinux, PlatformLinux, false},
		{"us-east-1", PlatformCentOS, nil, "7.1", nil, PlatformCentOS, PlatformLinux, false},
		{"us-east-1", PlatformSuseOS, nil, "12", nil, PlatformSuseOS, PlatformLinux, false},
		{"us-east-1", PlatformRedHat, nil, "6.8", nil, PlatformRedHat, PlatformLinux, false},
		{"us-east-1", PlatformUbuntu, nil, "12", nil, PlatformUbuntu, PlatformUbuntu, false},
		{"us-east-1", PlatformWindows, nil, "5", nil, PlatformWindows, PlatformWindows, false},
		{"us-east-1", "", fmt.Errorf("error"), "", nil, "", "", true},
		{"us-east-1", "", nil, "", fmt.Errorf("error"), "", "", true},
		{"", "", nil, "", nil, "", "", true},
	}

	getRegion = RegionStub
	getPlatformName = PlatformNameStub
	getPlatformVersion = PlatformVersionStub
	util := Utility{}

	for _, test := range testCases {
		// Setup stubs
		context = test

		context, err := util.CreateInstanceContext(logger)

		if test.expectingError {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, context.Platform, test.expectedPlatformName)
			assert.Equal(t, context.InstallerName, test.expectedInstallerName)
		}
	}
}

var context testInstanceContext

func PlatformVersionStub(log log.T) (version string, err error) {
	return context.platformVersion, context.platformVersionErr
}
func PlatformNameStub(log log.T) (name string, err error) {
	return context.platformName, context.platformNameErr
}
func RegionStub() (string, error) {
	var err error = nil
	if context.region == "" {
		err = fmt.Errorf("error")
	}
	return context.region, err
}

func TestFileNameConstruction(t *testing.T) {
	testCases := []struct {
		context InstanceContext
		result  string
	}{
		{InstanceContext{"us-east-1", "linux", "2015.9", "linux", "amd64", "tar.gz"}, "amazon-ssm-agent-linux-amd64.tar.gz"},
		{InstanceContext{"us-east-1", "linux", "2015.9", "linux", "386", "tar.gz"}, "amazon-ssm-agent-linux-386.tar.gz"},
		{InstanceContext{"us-west-1", "ubuntu", "12", "ubuntu", "386", "tar.gz"}, "amazon-ssm-agent-ubuntu-386.tar.gz"},
	}

	for _, test := range testCases {
		fileNameResult := test.context.FileName("amazon-ssm-agent")
		assert.Equal(t, fileNameResult, test.result)
	}
}

func TestBuildMessage(t *testing.T) {
	err := fmt.Errorf("first error message")
	var result = BuildMessage(err, "another message")

	assert.Contains(t, result, "first error message")
	assert.Contains(t, result, "another message")
}

func TestBuildMessages(t *testing.T) {
	errs := []error{fmt.Errorf("first error message"), fmt.Errorf("second error message")}
	var result = BuildMessages(errs, "another message")

	assert.Contains(t, result, "first error message")
	assert.Contains(t, result, "second error message")
	assert.Contains(t, result, "another message")
}

func TestCreateUpdateDownloadFolderSucceeded(t *testing.T) {
	mkDirAll = func(path string, perm os.FileMode) error {
		return nil
	}
	util := Utility{}
	result, _ := util.CreateUpdateDownloadFolder()
	assert.Contains(t, result, "update")
}

func TestCreateUpdateDownloadFolderFailed(t *testing.T) {
	mkDirAll = func(path string, perm os.FileMode) error {
		return fmt.Errorf("Folder cannot be created")
	}
	util := Utility{}
	_, err := util.CreateUpdateDownloadFolder()
	assert.Error(t, err)
}

func TestBuildUpdateCommand(t *testing.T) {
	testCases := []struct {
		cmd      string
		value    string
		expected string
		result   bool
	}{
		{"test", "value", "-test value", true},
		{"test", "", "-test value", false},
		{"", "value", "-test value", false},
	}

	for _, test := range testCases {
		result := BuildUpdateCommand("Cmd", test.cmd, test.value)
		assert.Equal(t, strings.Contains(result, test.expected), test.result)
	}
}

func TestUpdateArtifactFolder(t *testing.T) {
	testCases := []struct {
		pkgname string
		version string
	}{
		{"amazon-ssm-agent", "1.0.0.0"},
		{"amazon-ssm-agent-updater", "2.0.0.0"},
	}

	for _, test := range testCases {
		result := UpdateArtifactFolder(appconfig.UpdaterArtifactsRoot, test.pkgname, test.version)
		assert.Contains(t, result, test.pkgname)
		assert.Contains(t, result, test.version)
	}
}

func TestUpdateContextFilePath(t *testing.T) {
	result := UpdateContextFilePath(appconfig.UpdaterArtifactsRoot)
	assert.Contains(t, result, UpdateContextFileName)
}

func TestUpdateOutputDirectory(t *testing.T) {
	result := UpdateOutputDirectory(appconfig.UpdaterArtifactsRoot)
	assert.Equal(t, strings.Contains(result, DefaultOutputFolder), true)
}

func TestUpdateStandOutPath(t *testing.T) {
	testCases := []struct {
		filename         string
		expectedFileName string
	}{
		{"std.out", "std.out"},
		{"", DefaultStandOut},
	}

	for _, test := range testCases {
		result := UpdateStdOutPath(appconfig.UpdaterArtifactsRoot, test.filename)
		assert.Contains(t, result, test.expectedFileName)
	}
}

func TestUpdateStandErrPath(t *testing.T) {
	testCases := []struct {
		filename         string
		expectedFileName string
	}{
		{"std.err", "std.err"},
		{"", DefaultStandErr},
	}

	for _, test := range testCases {
		result := UpdateStdErrPath(appconfig.UpdaterArtifactsRoot, test.filename)
		assert.Contains(t, result, test.expectedFileName)
	}
}

func TestUpdatePluginResultFilePath(t *testing.T) {
	result := UpdatePluginResultFilePath(appconfig.UpdaterArtifactsRoot)
	assert.Contains(t, result, UpdatePluginResultFileName)
}

func TestUpdaterFilePath(t *testing.T) {
	testCases := []struct {
		pkgname string
		version string
	}{
		{"amazon-ssm-agent", "1.0.0.0"},
		{"amazon-ssm-agent-updater", "2.0.0.0"},
	}

	for _, test := range testCases {
		result := UpdaterFilePath(appconfig.UpdaterArtifactsRoot, test.pkgname, test.version)
		assert.Contains(t, result, test.pkgname)
		assert.Contains(t, result, test.version)
		assert.Contains(t, result, Updater)
	}
}

func TestInstallerFilePath(t *testing.T) {
	testCases := []struct {
		pkgname string
		version string
	}{
		{"amazon-ssm-agent", "1.0.0.0"},
		{"amazon-ssm-agent-updater", "2.0.0.0"},
	}

	for _, test := range testCases {
		result := InstallerFilePath(appconfig.UpdaterArtifactsRoot, test.pkgname, test.version)
		assert.Contains(t, result, test.pkgname)
		assert.Contains(t, result, test.version)
		assert.Contains(t, result, Installer)
	}
}

func TestUnInstallerFilePath(t *testing.T) {
	testCases := []struct {
		pkgname string
		version string
	}{
		{"amazon-ssm-agent", "1.0.0.0"},
		{"amazon-ssm-agent-updater", "2.0.0.0"},
	}

	for _, test := range testCases {
		result := UnInstallerFilePath(appconfig.UpdaterArtifactsRoot, test.pkgname, test.version)
		assert.Contains(t, result, test.pkgname)
		assert.Contains(t, result, test.version)
		assert.Contains(t, result, UnInstaller)
	}
}

func TestIsPlatformUsingSystemD(t *testing.T) {
	testCases := []struct {
		context InstanceContext
		result  bool
	}{
		{InstanceContext{"us-east-1", PlatformRedHat, "6.5", "linux", "amd64", "tar.gz"}, false},
		{InstanceContext{"us-east-1", PlatformRedHat, "7.0", "linux", "amd64", "tar.gz"}, true},
		{InstanceContext{"us-west-1", PlatformCentOS, "6.1", "linux", "amd64", "tar.gz"}, false},
		{InstanceContext{"us-east-1", PlatformSuseOS, "12", "linux", "amd64", "tar.gz"}, true},
		{InstanceContext{"us-west-1", PlatformCentOS, "7", "linux", "amd64", "tar.gz"}, true},
	}

	for _, test := range testCases {
		result, err := test.context.IsPlatformUsingSystemD(logger)
		assert.NoError(t, err)
		assert.Equal(t, result, test.result)
	}
}

func TestIsPlatformUsingSystemDWithInvalidVersionNumber(t *testing.T) {
	testCases := []struct {
		context InstanceContext
		result  bool
	}{
		{InstanceContext{"us-east-1", PlatformRedHat, "wrong version", "linux", "amd64", "tar.gz"}, false},
	}

	for _, test := range testCases {
		_, err := test.context.IsPlatformUsingSystemD(logger)
		assert.Error(t, err)
	}
}

func TestIsPlatformUsingSystemDWithPossiblyUsingSystemD(t *testing.T) {
	testCases := []struct {
		context InstanceContext
		result  bool
	}{
		{InstanceContext{"us-east-1", PlatformRaspbian, "8", "linux", "amd64", "tar.gz"}, true},
	}

	// Stub exec.Command
	execCommand = fakeExecCommand

	for _, test := range testCases {
		result, err := test.context.IsPlatformUsingSystemD(logger)
		assert.NoError(t, err)
		assert.Equal(t, result, test.result)
	}
}

func TestIsServiceRunning(t *testing.T) {
	util := Utility{}
	testCases := []struct {
		context InstanceContext
		result  bool
	}{
		// test system with upstart
		{InstanceContext{"us-east-1", PlatformRedHat, "6.5", "linux", "amd64", "tar.gz"}, true},
		// test system with systemD
		{InstanceContext{"us-east-1", PlatformRedHat, "7.1", "linux", "amd64", "tar.gz"}, true},
	}

	// Stub exec.Command
	execCommand = fakeExecCommand

	for _, test := range testCases {
		result, _ := util.IsServiceRunning(logger, &test.context)
		assert.Equal(t, result, test.result)
	}
}

func TestIsServiceRunningWithErrorMessageFromCommandExec(t *testing.T) {
	util := Utility{}
	testCases := []struct {
		context InstanceContext
	}{
		// test system with upstart
		{InstanceContext{"us-east-1", PlatformRedHat, "6.5", "linux", "amd64", "tar.gz"}},
		// test system with systemD
		{InstanceContext{"us-east-1", PlatformRedHat, "7.1", "linux", "amd64", "tar.gz"}},
	}

	// Stub exec.Command
	execCommand = fakeExecCommandWithError

	for _, test := range testCases {
		_, err := util.IsServiceRunning(logger, &test.context)
		assert.Error(t, err)
	}
}

func TestExeCommandSucceeded(t *testing.T) {
	testCases := []struct {
		cmd            string
		workingDir     string
		stdOut         string
		stdErr         string
		isAsync        bool
		expectingError bool
	}{
		// test system with upstart
		{"-update -target.version 5.0.0", "temp", "stdout", "stderr", true, false},
		// test system with systemD
		{"-update -target.version 5.0.0", "temp", "stdout", "stderr", false, true},
	}

	mkDirAll = func(path string, perm os.FileMode) error {
		return nil
	}
	openFile = func(name string, flag int, perm os.FileMode) (*os.File, error) {
		return &os.File{}, nil
	}

	// Stub exec.Command
	execCommand = fakeExecCommand
	cmdStart = func(*exec.Cmd) error { return nil }

	util := Utility{}

	for _, test := range testCases {
		err := util.ExeCommand(logger,
			test.cmd,
			test.workingDir,
			appconfig.UpdaterArtifactsRoot,
			test.stdOut,
			test.stdErr,
			test.isAsync)

		if test.expectingError {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
	}
}

func TestKillProcess(t *testing.T) {
	// Stub exec.Command
	var cmd = fakeExecCommand("-update", "-target.version 5.0.0")
	cmd.Process = &os.Process{}

	timer := time.NewTimer(time.Duration(1) * time.Millisecond)
	killProcessOnTimeout(logger, cmd, timer)
}

func TestSetExeOutErrCannotCreateFolder(t *testing.T) {
	// Stub exec.Command
	mkDirAll = func(path string, perm os.FileMode) error {
		return fmt.Errorf("create folder error")
	}
	_, _, err := setExeOutErr(appconfig.UpdaterArtifactsRoot, "std", "err")
	assert.Error(t, err, "create folder error")
}

func TestSetExeOutErrCannotOpenFile(t *testing.T) {
	// Stub exec.Command
	mkDirAll = func(path string, perm os.FileMode) error {
		return nil
	}
	openFile = func(name string, flag int, perm os.FileMode) (*os.File, error) {
		return &os.File{}, fmt.Errorf("create file error")
	}
	_, _, err := setExeOutErr(appconfig.UpdaterArtifactsRoot, "std", "err")
	assert.Error(t, err, "create file error")
}

func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestExecCommandHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func fakeExecCommandWithError(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestExecCommandHelperProcess", "-test.error", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

// TestHelperProcess is not a real test, it's the helper method for other tests
func TestExecCommandHelperProcess(*testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	args := os.Args
	testError := false
	for len(args) > 0 {
		if args[0] == "-test.error" {
			testError = true
		}
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command\n")
		os.Exit(2)
	}
	cmd, args := args[0], args[1:]
	if testError {
		fmt.Fprintf(os.Stderr, "Error")
	} else {
		switch cmd {
		case "systemctl":
			fmt.Println("Active: active (running)")
		case "status":
			fmt.Println("amazon-ssm-agent start/running")
		case "update":
			fmt.Println("test update")
		}
	}
}

func TestIsDiskSpaceSufficientForUpdateWithSufficientSpace(t *testing.T) {
	getDiskSpaceInfo = func() (fileutil.DiskSpaceInfo, error) {
		return fileutil.DiskSpaceInfo{
			AvailBytes: MinimumDiskSpaceForUpdate,
			FreeBytes:  0,
			TotalBytes: 0,
		}, nil
	}

	util := Utility{}
	isSufficient, err := util.IsDiskSpaceSufficientForUpdate(logger)

	assert.NoError(t, err)
	assert.True(t, isSufficient)
}

func TestIsDiskSpaceSufficientForUpdateWithInsufficientSpace(t *testing.T) {
	getDiskSpaceInfo = func() (fileutil.DiskSpaceInfo, error) {
		return fileutil.DiskSpaceInfo{
			AvailBytes: MinimumDiskSpaceForUpdate - 1,
			FreeBytes:  0,
			TotalBytes: 0,
		}, nil
	}

	util := Utility{}
	isSufficient, err := util.IsDiskSpaceSufficientForUpdate(logger)

	assert.NoError(t, err)
	assert.False(t, isSufficient)
}

func TestIsDiskSpaceSufficientForUpdateWithDiskSpaceLoadFail(t *testing.T) {
	getDiskSpaceInfo = func() (fileutil.DiskSpaceInfo, error) {
		return fileutil.DiskSpaceInfo{
			AvailBytes: 0,
			FreeBytes:  0,
			TotalBytes: 0,
		}, fmt.Errorf("mock error - failed to load the disk space")
	}

	util := Utility{}
	isSufficient, err := util.IsDiskSpaceSufficientForUpdate(logger)

	assert.Error(t, err)
	assert.False(t, isSufficient)
}

func TestCompareVersion(t *testing.T) {
	var res int
	var err error

	// major version 1 > major version 2
	res, err = CompareVersion("2.0.0.0", "1.0.0.0")
	assert.Nil(t, err)
	assert.Equal(t, 1, res)

	// major version 1 < major version 2
	res, err = CompareVersion("1.0.0.0", "2.0.0.0")
	assert.Nil(t, err)
	assert.Equal(t, -1, res)

	// minor version 1 > minor version 2
	res, err = CompareVersion("2.1.0.0", "2.0.0.0")
	assert.Nil(t, err)
	assert.Equal(t, 1, res)

	// minor version 1 < minor version 2
	res, err = CompareVersion("2.0.0.0", "2.1.0.0")
	assert.Nil(t, err)
	assert.Equal(t, -1, res)

	// build version 1 > build version 2
	res, err = CompareVersion("2.1.10.0", "2.1.5.0")
	assert.Nil(t, err)
	assert.Equal(t, 1, res)

	// build version 1 < build version 2
	res, err = CompareVersion("2.1.3.0", "2.1.12.0")
	assert.Nil(t, err)
	assert.Equal(t, -1, res)

	// patch version 1 > patch version 2
	res, err = CompareVersion("2.1.10.100", "2.1.10.50")
	assert.Nil(t, err)
	assert.Equal(t, 1, res)

	// patch version 1 < patch version 2
	res, err = CompareVersion("2.1.10.100", "2.1.10.1000")
	assert.Nil(t, err)
	assert.Equal(t, -1, res)

	// version 1 == version 2
	res, err = CompareVersion("2.5.7.8", "2.5.7.8")
	assert.Nil(t, err)
	assert.Equal(t, 0, res)

	// version 1 contains invalid characters
	res, err = CompareVersion("2.foo.7.8", "2.5.7.8")
	assert.NotNil(t, err)

	// version 2 contains invalid characters
	res, err = CompareVersion("2.5.7.8", "2.5.7.bar")
	assert.NotNil(t, err)

	// versions contains wrong format
	res, err = CompareVersion("2.5.7.8.9", "2.5.7.8.9")
	assert.NotNil(t, err)

}
