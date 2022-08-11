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

	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/mocks"
	identityMocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	"github.com/aws/amazon-ssm-agent/core/executor"
	"github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider/model"
	"github.com/stretchr/testify/assert"
)

var logger = log.NewMockLog()

type testProcess struct {
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

	util := Utility{
		Context: context.NewMockDefault(),
	}

	result, _ := util.CreateUpdateDownloadFolder()
	assert.Contains(t, result, "update")
}

func TestCreateUpdateDownloadFolderFailed(t *testing.T) {
	mkDirAll = func(path string, perm os.FileMode) error {
		return fmt.Errorf("Folder cannot be created")
	}

	util := Utility{
		Context: context.NewMockDefault(),
	}
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

func TestConvertToUpdateErrorCode_MulipleCases(t *testing.T) {
	output := ConvertToUpdateErrorCode(string(updateconstants.ErrorCreateUpdateFolder), "_", "Test")
	assert.Equal(t, string(output), "ErrorCreateUpdateFolder_Test")
	output = ConvertToUpdateErrorCode(string(updateconstants.ErrorCreateUpdateFolder))
	assert.Equal(t, string(output), "ErrorCreateUpdateFolder")
	output = ConvertToUpdateErrorCode("Test")
	assert.Equal(t, string(output), "Test")
	output = ConvertToUpdateErrorCode()
	assert.Equal(t, string(output), "")
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
	assert.Contains(t, result, updateconstants.UpdateContextFileName)
}

func TestUpdateOutputDirectory(t *testing.T) {
	result := UpdateOutputDirectory(appconfig.UpdaterArtifactsRoot)
	assert.Equal(t, strings.Contains(result, updateconstants.DefaultOutputFolder), true)
}

func TestUpdateStandOutPath(t *testing.T) {
	testCases := []struct {
		filename         string
		expectedFileName string
	}{
		{"std.out", "std.out"},
		{"", updateconstants.DefaultStandOut},
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
		{"", updateconstants.DefaultStandErr},
	}

	for _, test := range testCases {
		result := UpdateStdErrPath(appconfig.UpdaterArtifactsRoot, test.filename)
		assert.Contains(t, result, test.expectedFileName)
	}
}

func TestUpdatePluginResultFilePath(t *testing.T) {
	result := UpdatePluginResultFilePath(appconfig.UpdaterArtifactsRoot)
	assert.Contains(t, result, updateconstants.UpdatePluginResultFileName)
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
		assert.Contains(t, result, updateconstants.Updater)
	}
}

func TestInstallerFilePath(t *testing.T) {
	randomInstaller := "someinstaller.sh"
	testCases := []struct {
		pkgname string
		version string
	}{
		{"amazon-ssm-agent", "1.0.0.0"},
		{"amazon-ssm-agent-updater", "2.0.0.0"},
	}

	for _, test := range testCases {
		result := InstallerFilePath(appconfig.UpdaterArtifactsRoot, test.pkgname, test.version, randomInstaller)
		assert.Contains(t, result, test.pkgname)
		assert.Contains(t, result, test.version)
		assert.Contains(t, result, randomInstaller)
	}
}

func TestUnInstallerFilePath(t *testing.T) {
	randomUnInstaller := "someuninstaller.sh"
	testCases := []struct {
		pkgname string
		version string
	}{
		{"amazon-ssm-agent", "1.0.0.0"},
		{"amazon-ssm-agent-updater", "2.0.0.0"},
	}

	for _, test := range testCases {
		result := UnInstallerFilePath(appconfig.UpdaterArtifactsRoot, test.pkgname, test.version, randomUnInstaller)
		assert.Contains(t, result, test.pkgname)
		assert.Contains(t, result, test.version)
		assert.Contains(t, result, randomUnInstaller)
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

	util := Utility{
		Context: context.NewMockDefault(),
	}

	for _, test := range testCases {
		_, _, err := util.ExeCommand(logger,
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
			AvailBytes: updateconstants.MinimumDiskSpaceForUpdate,
			FreeBytes:  0,
			TotalBytes: 0,
		}, nil
	}

	util := Utility{
		Context: context.NewMockDefault(),
	}
	isSufficient, err := util.IsDiskSpaceSufficientForUpdate(logger)

	assert.NoError(t, err)
	assert.True(t, isSufficient)
}

func TestIsDiskSpaceSufficientForUpdateWithInsufficientSpace(t *testing.T) {
	getDiskSpaceInfo = func() (fileutil.DiskSpaceInfo, error) {
		return fileutil.DiskSpaceInfo{
			AvailBytes: updateconstants.MinimumDiskSpaceForUpdate - 1,
			FreeBytes:  0,
			TotalBytes: 0,
		}, nil
	}

	util := Utility{
		Context: context.NewMockDefault(),
	}
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

	util := Utility{
		Context: context.NewMockDefault(),
	}
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

func TestGetManifestURLFromSourceUrl(t *testing.T) {
	// Empty URL
	url, err := GetManifestURLFromSourceUrl("")
	assert.NotNil(t, err)
	assert.Equal(t, "", url)

	// Invalid URL
	url, err = GetManifestURLFromSourceUrl("InvalidUrl")
	assert.NotNil(t, err)
	assert.Equal(t, "", url)

	// Valid s3 link bucket in URL
	url, err = GetManifestURLFromSourceUrl("https://bucket.s3.region.amazonaws.com/amazon-ssm-agent/version/amazon-ssm-agent.tar.gz")
	assert.Nil(t, err)
	assert.Equal(t, "https://bucket.s3.region.amazonaws.com/ssm-agent-manifest.json", url)

	// Valid s3 link bucket in Path
	url, err = GetManifestURLFromSourceUrl("https://s3.region.amazonaws.com/bucket/amazon-ssm-agent/version/amazon-ssm-agent.tar.gz")
	assert.Nil(t, err)
	assert.Equal(t, "https://s3.region.amazonaws.com/bucket/ssm-agent-manifest.json", url)

	// Valid s3 link bucket in URL - china
	url, err = GetManifestURLFromSourceUrl("https://bucket.s3.region.amazonaws.com.cn/amazon-ssm-agent/version/amazon-ssm-agent.tar.gz")
	assert.Nil(t, err)
	assert.Equal(t, "https://bucket.s3.region.amazonaws.com.cn/ssm-agent-manifest.json", url)

	// Valid s3 link bucket in Path - china
	url, err = GetManifestURLFromSourceUrl("https://s3.region.amazonaws.com.cn/bucket/amazon-ssm-agent/version/amazon-ssm-agent.tar.gz")
	assert.Nil(t, err)
	assert.Equal(t, "https://s3.region.amazonaws.com.cn/bucket/ssm-agent-manifest.json", url)

	// Valid s3 link but not expected path
	url, err = GetManifestURLFromSourceUrl("https://s3.region.amazonaws.com/bucket")
	assert.NotNil(t, err)
	assert.Equal(t, "", url)
}

func TestIsV1UpdatePlugin(t *testing.T) {
	// lower than
	assert.True(t, IsV1UpdatePlugin("3.0.881.0"))

	// equal than
	assert.True(t, IsV1UpdatePlugin("3.0.882.0"))

	// greater than
	assert.False(t, IsV1UpdatePlugin("3.0.883.0"))

	// invalid source
	assert.False(t, IsV1UpdatePlugin("SomeInvalidVersion"))
}

func TestIsIdentityRuntimeConfigSupported(t *testing.T) {
	// lower than
	assert.False(t, IsIdentityRuntimeConfigSupported("3.0.281.0"))
	assert.False(t, IsIdentityRuntimeConfigSupported("1.1.281.0"))
	assert.False(t, IsIdentityRuntimeConfigSupported("3.1.281.0"))

	// equal than
	assert.False(t, IsIdentityRuntimeConfigSupported("3.1.282.0"))

	// greater than
	assert.True(t, IsIdentityRuntimeConfigSupported("3.1.282.1"))
	assert.True(t, IsIdentityRuntimeConfigSupported("3.1.1111.0"))
	assert.True(t, IsIdentityRuntimeConfigSupported("3.1.283.0"))
	assert.True(t, IsIdentityRuntimeConfigSupported("3.2.283.0"))

	// invalid source
	assert.False(t, IsIdentityRuntimeConfigSupported("SomeInvalidVersion"))
}

func TestUtility_setShareCredsEnvironment_SetsCommandAWSEnvironmentVariables_WhenIdentitySharesCredentials(t *testing.T) {
	ctx := new(context.Mock)
	agentIdentity := identityMocks.NewDefaultMockAgentIdentity()
	ctx.On("Identity").Return(agentIdentity)
	ctx.On("Log").Return(log.NewMockLog())

	remoteProvier := &mocks.IRemoteProvider{}
	remoteProvier.On("SharesCredentials").Return(true)
	expectedShareProfile := "SomeShareFileLocation"
	expectedShareFile := "SomeShareFileLocation"
	remoteProvier.On("ShareProfile").Return(expectedShareProfile)
	remoteProvier.On("ShareFile").Return(expectedShareFile)
	getRemoteProvider = func(agentIdentity identity.IAgentIdentity) (credentialproviders.IRemoteProvider, bool) {
		return remoteProvier, true
	}

	utility := &Utility{
		Context: ctx,
	}

	command := &exec.Cmd{}
	utility.setCommandEnvironmentVariables(command)

	expectedProfileVar := fmt.Sprintf("AWS_PROFILE=%s", expectedShareProfile)
	expectedSharedFileVar := fmt.Sprintf("AWS_SHARED_CREDENTIALS_FILE=%s", expectedShareFile)

	assert.Contains(t, command.Env, expectedProfileVar, "AWS_PROFILE environment variable not set")
	assert.Contains(t, command.Env, expectedSharedFileVar, "AWS_SHARED_CREDENTIALS_FILE environment variable not set")
}

func (p *testProcess) Start(*model.WorkerConfig) (*model.Process, error) { return nil, nil }

func (p *testProcess) Kill(pid int) error { return nil }

func (p *testProcess) Processes() ([]executor.OsProcess, error) {
	var allProcess []executor.OsProcess
	var process = executor.OsProcess{
		Pid:        1,
		Executable: model.SSMAgentWorkerBinaryName,
	}
	allProcess = append(allProcess, process)
	return allProcess, nil
}
