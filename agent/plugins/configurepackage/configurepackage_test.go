// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package configurepackage implements the ConfigurePackage plugin.
package configurepackage

import (
	"strings"
	"sync"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/installer/mock"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/localpackages"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/localpackages/mock"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/packageservice"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/packageservice/mock"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var loggerMock = log.NewMockLog()
var contextMock context.T = context.NewMockDefault()

func TestRunUpgrade(t *testing.T) {
	plugin := &Plugin{}
	pluginInformation := createStubPluginInputInstall()

	managerMock := ConfigPackageSuccessMock("/foo", "1.0.0", "0.5.6", contracts.ResultStatusSuccess, contracts.ResultStatusSuccess, contracts.ResultStatusSuccess)
	output := runConfigurePackage(plugin, contextMock, managerMock, pluginInformation)

	assert.Equal(t, output.ExitCode, 0)
	assert.Contains(t, output.Stdout, "Successfully installed")
	managerMock.AssertCalled(t, "runUninstallPackagePre", "PVDriver", "0.5.6", mock.Anything, mock.Anything)
	managerMock.AssertCalled(t, "runUninstallPackagePost", "PVDriver", "0.5.6", mock.Anything, mock.Anything)
}

func TestRunUpgradeUninstallReboot(t *testing.T) {
	plugin := &Plugin{}
	pluginInformation := createStubPluginInputInstall()

	managerMock := ConfigPackageSuccessMock("/foo", "1.0.0", "0.5.6", contracts.ResultStatusSuccess, contracts.ResultStatusSuccessAndReboot, contracts.ResultStatusSuccess)
	output := runConfigurePackage(plugin, contextMock, managerMock, pluginInformation)

	assert.Equal(t, output.ExitCode, 0)
	managerMock.AssertCalled(t, "runUninstallPackagePre", "PVDriver", "0.5.6", mock.Anything, mock.Anything)
	managerMock.AssertNotCalled(t, "runInstallPackage")
	managerMock.AssertNotCalled(t, "runUninstallPackagePost")
}

func TestRunParallelSamePackage(t *testing.T) {
	plugin := &Plugin{}
	pluginInformation := createStubPluginInputInstall()

	managerMockFirst := ConfigPackageSuccessMock("/foo", "Wait1.0.0", "", contracts.ResultStatusSuccess, contracts.ResultStatusSuccess, contracts.ResultStatusSuccess)
	managerMockSecond := ConfigPackageSuccessMock("/foo", "1.0.0", "", contracts.ResultStatusSuccess, contracts.ResultStatusSuccess, contracts.ResultStatusSuccess)

	var outputFirst contracts.PluginOutput
	var outputSecond contracts.PluginOutput
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		outputFirst = runConfigurePackage(plugin, contextMock, managerMockFirst, pluginInformation)
	}()
	// wait until first call is at getVersionToInstall
	_ = <-managerMockFirst.waitChan
	// start second call
	outputSecond = runConfigurePackage(plugin, contextMock, managerMockSecond, pluginInformation)
	// after second call completes, allow first call to continue
	managerMockFirst.waitChan <- true
	// wait until first call is complete
	wg.Wait()

	assert.Equal(t, outputFirst.ExitCode, 0)
	assert.Equal(t, outputSecond.ExitCode, 1)
	assert.True(t, strings.Contains(outputSecond.Stderr, `Package "PVDriver" is already in the process of action "Install"`))
}

func TestExecute(t *testing.T) {
	pluginInformation := createStubPluginInputInstall()
	config := contracts.Configuration{}
	p := make([]interface{}, 1)
	p[0] = pluginInformation
	config.Properties = p
	plugin := &Plugin{}

	// set region in cache to prevent test delay because of detecion failures
	platform.SetRegion("testregion")

	runConfigOrig := runConfig

	runConfig = func(
		p *Plugin,
		context context.T,
		manager configurePackageManager,
		input *ConfigurePackagePluginInput) (out contracts.PluginOutput) {
		out = contracts.PluginOutput{}
		out.ExitCode = 1
		out.Stderr = "error"

		return out
	}
	defer func() {
		runConfig = runConfigOrig
	}()

	// TODO:MF Test result code for reboot in cases where that is expected?

	result := plugin.Execute(contextMock, config, createMockCancelFlag(), runpluginutil.PluginRunner{})

	assert.Equal(t, result.Code, 1)
	assert.Contains(t, result.Output, "error")
}

type S3PrefixTestCase struct {
	PluginID         string
	OrchestrationDir string
	BucketName       string
	PrefixIn         string
	NumInputs        int
}

func testS3Prefix(t *testing.T, testCase S3PrefixTestCase) {
	var mockPlugin pluginutil.MockDefaultPlugin
	mockPlugin = pluginutil.MockDefaultPlugin{}
	mockPlugin.On("UploadOutputToS3Bucket", mock.Anything, testCase.PluginID, testCase.OrchestrationDir, testCase.BucketName, testCase.PrefixIn, false, mock.Anything, mock.Anything, mock.Anything).Return([]string{})

	// set region in cache to prevent test delay because of detecion failures
	platform.SetRegion("testregion")

	plugin := &Plugin{}
	plugin.ExecuteUploadOutputToS3Bucket = mockPlugin.UploadOutputToS3Bucket

	config := contracts.Configuration{}
	config.OrchestrationDirectory = testCase.OrchestrationDir
	config.OutputS3BucketName = testCase.BucketName
	config.OutputS3KeyPrefix = testCase.PrefixIn
	config.PluginID = testCase.PluginID

	runConfigOrig := runConfig
	runConfig = func(
		p *Plugin,
		context context.T,
		manager configurePackageManager,
		input *ConfigurePackagePluginInput) (out contracts.PluginOutput) {
		out = contracts.PluginOutput{}
		out.ExitCode = 0

		return out
	}
	defer func() {
		runConfig = runConfigOrig
	}()
	stubs := setSuccessStubs()
	defer stubs.Clear()

	pluginInformation := createStubPluginInputInstall()
	var result contracts.PluginResult
	if testCase.NumInputs == 1 {
		var rawPluginInput interface{}
		rawPluginInput = pluginInformation
		config.Properties = rawPluginInput
		result = plugin.Execute(contextMock, config, createMockCancelFlag(), runpluginutil.PluginRunner{})
	} else {
		rawPluginInput := make([]interface{}, testCase.NumInputs)
		for i := 0; i < testCase.NumInputs; i++ {
			rawPluginInput[i] = pluginInformation
		}
		config.Properties = rawPluginInput
		result = plugin.Execute(contextMock, config, createMockCancelFlag(), runpluginutil.PluginRunner{})
	}

	assert.Equal(t, result.Code, 0)
	mockPlugin.AssertExpectations(t)
}

func TestS3PrefixSchema1_2(t *testing.T) {
	testCase := S3PrefixTestCase{
		PluginID:         "aws:configurePackage",
		OrchestrationDir: "OrchestrationDir",
		BucketName:       "Bucket",
		PrefixIn:         "Prefix",
		NumInputs:        1,
	}
	testS3Prefix(t, testCase)
}

func TestS3PrefixSchema1_2x2(t *testing.T) {
	testCase := S3PrefixTestCase{
		PluginID:         "aws:configurePackage",
		OrchestrationDir: "OrchestrationDir",
		BucketName:       "Bucket",
		PrefixIn:         "Prefix",
		NumInputs:        2,
	}
	testS3Prefix(t, testCase)
}

func TestS3PrefixSchema2_0(t *testing.T) {
	testCase := S3PrefixTestCase{
		PluginID:         "configure:Package",
		OrchestrationDir: "OrchestrationDir",
		BucketName:       "Bucket",
		PrefixIn:         "Prefix",
		NumInputs:        1,
	}
	testS3Prefix(t, testCase)
}

func TestS3PrefixSchema2_0x2(t *testing.T) {
	testCase := S3PrefixTestCase{
		PluginID:         "configure:Package",
		OrchestrationDir: "OrchestrationDir",
		BucketName:       "Bucket",
		PrefixIn:         "Prefix",
		NumInputs:        2,
	}
	testS3Prefix(t, testCase)
}

func TestInstallPackage(t *testing.T) {
	pluginInformation := createStubPluginInputInstall()

	output := &contracts.PluginOutput{}

	mockRepo := repository_mock.MockedRepository{}
	mockInst := installer_mock.Mock{}
	mockInst.On("Install", mock.Anything).Return(&contracts.PluginOutput{Status: contracts.ResultStatusSuccess}).Once()
	mockRepo.On("GetInstaller", mock.Anything, mock.Anything, mock.Anything, pluginInformation.Name, pluginInformation.Version).Return(&mockInst)

	manager := createInstanceWithRepoMock(&mockRepo)

	stubs := setSuccessStubs()
	defer stubs.Clear()

	_, err := manager.runInstallPackage(contextMock,
		pluginInformation.Name,
		pluginInformation.Version,
		output)

	assert.NoError(t, err)
}

func TestUninstallPackage(t *testing.T) {
	pluginInformation := createStubPluginInputUninstall()

	output := &contracts.PluginOutput{}

	mockRepo := repository_mock.MockedRepository{}
	mockInst := installer_mock.Mock{}
	mockInst.On("Uninstall", mock.Anything).Return(&contracts.PluginOutput{Status: contracts.ResultStatusSuccess}).Once()
	mockRepo.On("GetInstaller", mock.Anything, mock.Anything, mock.Anything, pluginInformation.Name, pluginInformation.Version).Return(&mockInst)
	mockRepo.On("RemovePackage", mock.Anything, pluginInformation.Name, pluginInformation.Version).Return(nil).Once()

	manager := createInstanceWithRepoMock(&mockRepo)

	stubs := setSuccessStubs()
	defer stubs.Clear()

	_, errPre := manager.runUninstallPackagePre(contextMock,
		pluginInformation.Name,
		pluginInformation.Version,
		output)

	assert.NoError(t, errPre)

	_, errPost := manager.runUninstallPackagePost(contextMock,
		pluginInformation.Name,
		pluginInformation.Version,
		output)

	assert.NoError(t, errPost)
}

// TO DO: Uninstall test for exe command

func TestValidateInput(t *testing.T) {
	input := ConfigurePackagePluginInput{}

	input.Version = "1.0.0"
	input.Name = "PVDriver"
	input.Action = "InvalidAction"

	result, err := validateInput(&input)

	assert.True(t, result)
	assert.NoError(t, err)
}

func TestValidateInput_Source(t *testing.T) {
	input := ConfigurePackagePluginInput{}

	input.Version = "1.0.0"
	input.Name = "PVDriver"
	input.Action = "Install"
	input.Source = "http://amazon.com"

	result, err := validateInput(&input)

	assert.False(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "source parameter is not supported")
}

func TestValidateInput_NameEmpty(t *testing.T) {
	input := ConfigurePackagePluginInput{}

	// Set version to a large number to avoid conflict of the actual package release version
	input.Version = "9000.0.0"
	input.Name = ""
	input.Action = "Install"

	result, err := validateInput(&input)

	assert.False(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty name field")
}

func TestValidateInput_NameValid(t *testing.T) {
	input := ConfigurePackagePluginInput{}
	// Set version to a large number to avoid conflict of the actual package release version
	input.Version = "9000.0.0"
	input.Action = "Install"

	validNames := []string{"a0", "_a", "_._._", "_-_", "A", "ABCDEFGHIJKLM-NOPQRSTUVWXYZ.abcdefghijklm-nopqrstuvwxyz.1234567890"}

	for _, name := range validNames {
		input.Name = name

		result, err := validateInput(&input)

		assert.True(t, result)
		assert.NoError(t, err)
	}
}

func TestValidateInput_VersionValid(t *testing.T) {
	input := ConfigurePackagePluginInput{}

	validVersions := []string{"1.0.0", "9000.0.0.0", "0.21.0", "1.2.3.4", "5.4.3.2.1", "1.2.3-a.b.c.10.d.5+beta"}

	for _, version := range validVersions {
		input.Version = version
		input.Name = "PVDriver"
		input.Action = "Install"

		result, err := validateInput(&input)

		assert.True(t, result)
		assert.NoError(t, err)
	}
}

func TestValidateInput_EmptyVersionWithInstall(t *testing.T) {
	input := ConfigurePackagePluginInput{}

	// Set version to a large number to avoid conflict of the actual package release version
	input.Version = ""
	input.Name = "PVDriver"
	input.Action = "Install"

	result, err := validateInput(&input)

	assert.True(t, result)
	assert.NoError(t, err)
}

func TestValidateInput_EmptyVersionWithUninstall(t *testing.T) {
	input := ConfigurePackagePluginInput{}

	// Set version to a large number to avoid conflict of the actual package release version
	input.Version = ""
	input.Name = "PVDriver"
	input.Action = "Uninstall"

	result, err := validateInput(&input)

	assert.True(t, result)
	assert.NoError(t, err)
}

func TestPackageLock(t *testing.T) {
	// lock Foo for Install
	err := lockPackage("Foo", "Install")
	assert.Nil(t, err)
	defer unlockPackage("Foo")

	// shouldn't be able to lock Foo, even for a different action
	err = lockPackage("Foo", "Uninstall")
	assert.NotNil(t, err)

	// lock and unlock Bar (with defer)
	err = lockAndUnlock("Bar")
	assert.Nil(t, err)

	// should be able to lock and then unlock Bar
	err = lockPackage("Bar", "Uninstall")
	assert.Nil(t, err)
	unlockPackage("Bar")

	// should be able to lock Bar
	err = lockPackage("Bar", "Uninstall")
	assert.Nil(t, err)
	defer unlockPackage("Bar")

	// lock in a goroutine with a 10ms sleep
	errorChan := make(chan error)
	go lockAndUnlockGo("Foobar", errorChan)
	err = <-errorChan // wait until the goroutine has acquired the lock
	assert.Nil(t, err)
	err = lockPackage("Foobar", "Install")
	errorChan <- err // signal the goroutine to exit
	assert.NotNil(t, err)
}

func lockAndUnlockGo(packageName string, channel chan error) {
	err := lockPackage(packageName, "Install")
	channel <- err
	_ = <-channel
	if err == nil {
		defer unlockPackage(packageName)
	}
	return
}

func lockAndUnlock(packageName string) (err error) {
	if err = lockPackage(packageName, "Install"); err != nil {
		return
	}
	defer unlockPackage(packageName)
	return
}

func createInstance() configurePackageManager {
	return &configurePackage{Configuration: contracts.Configuration{}, runner: runpluginutil.PluginRunner{}, repository: &repository_mock.MockedRepository{}, packageservice: &packageservice_mock.Mock{}}
}

func createInstanceWithRepoMock(repoMock localpackages.Repository) configurePackageManager {
	return &configurePackage{Configuration: contracts.Configuration{}, runner: runpluginutil.PluginRunner{}, repository: repoMock, packageservice: &packageservice_mock.Mock{}}
}

func createInstanceWithRepoAndDSMock(repoMock localpackages.Repository, dsMock packageservice.PackageService) configurePackageManager {
	return &configurePackage{Configuration: contracts.Configuration{}, runner: runpluginutil.PluginRunner{}, repository: repoMock, packageservice: dsMock}
}
