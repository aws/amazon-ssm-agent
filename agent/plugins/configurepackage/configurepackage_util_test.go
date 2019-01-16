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
// test_configurepackage contains stub implementations
package configurepackage

import (
	"errors"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	iohandlermocks "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler/mock"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/facade"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/installer"
	installerMock "github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/installer/mock"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/localpackages"
	repoMock "github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/localpackages/mock"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/packageservice"
	serviceMock "github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/packageservice/mock"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/trace"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/mock"
)

func repoInstallMock(pluginInformation *ConfigurePackagePluginInput, installerMock installer.Installer) *repoMock.MockedRepository {
	mockRepo := repoMock.MockedRepository{}
	mockRepo.On("GetInstalledVersion", mock.Anything, mock.Anything).Return("")
	mockRepo.On("GetInstallState", mock.Anything, mock.Anything).Return(localpackages.None, "")
	mockRepo.On("ValidatePackage", mock.Anything, mock.Anything, pluginInformation.Version).Return(nil)
	mockRepo.On("SetInstallState", mock.Anything, mock.Anything, pluginInformation.Version, mock.Anything).Return(nil)
	mockRepo.On("GetInstaller", mock.Anything, mock.Anything, mock.Anything, pluginInformation.Version).Return(installerMock)
	mockRepo.On("LockPackage", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockRepo.On("UnlockPackage", mock.Anything, mock.Anything).Return()
	mockRepo.On("LoadTraces", mock.Anything, mock.Anything).Return(nil)
	return &mockRepo
}

func repoInstallMock_WithValidatePackageError(pluginInformation *ConfigurePackagePluginInput, installerMock installer.Installer) *repoMock.MockedRepository {
	mockRepo := repoMock.MockedRepository{}
	mockRepo.On("GetInstalledVersion", mock.Anything, mock.Anything).Return("")
	mockRepo.On("GetInstallState", mock.Anything, mock.Anything).Return(localpackages.None, "")
	mockRepo.On("ValidatePackage", mock.Anything, mock.Anything, pluginInformation.Version).Return(errors.New("There's an error"))
	mockRepo.On("SetInstallState", mock.Anything, mock.Anything, pluginInformation.Version, mock.Anything).Return(nil)
	mockRepo.On("GetInstaller", mock.Anything, mock.Anything, mock.Anything, pluginInformation.Version).Return(installerMock)
	mockRepo.On("LockPackage", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockRepo.On("UnlockPackage", mock.Anything, mock.Anything).Return()
	mockRepo.On("LoadTraces", mock.Anything, mock.Anything).Return(nil)
	return &mockRepo
}

func repoAlreadyInstalledMock(pluginInformation *ConfigurePackagePluginInput, installerMock installer.Installer) *repoMock.MockedRepository {
	mockRepo := repoMock.MockedRepository{}
	mockRepo.On("GetInstalledVersion", mock.Anything, mock.Anything).Return("0.0.1")
	mockRepo.On("GetInstallState", mock.Anything, mock.Anything).Return(localpackages.Installed, "")
	mockRepo.On("ValidatePackage", mock.Anything, mock.Anything, pluginInformation.Version).Return(nil)
	mockRepo.On("SetInstallState", mock.Anything, mock.Anything, pluginInformation.Version, mock.Anything).Return(nil)
	mockRepo.On("GetInstaller", mock.Anything, mock.Anything, mock.Anything, pluginInformation.Version).Return(installerMock)
	mockRepo.On("LockPackage", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockRepo.On("UnlockPackage", mock.Anything, mock.Anything).Return()
	return &mockRepo
}

func repoUpgradeMock(pluginInformation *ConfigurePackagePluginInput, installerMock installer.Installer) *repoMock.MockedRepository {
	mockRepo := repoMock.MockedRepository{}
	mockRepo.On("GetInstalledVersion", mock.Anything, mock.Anything).Return("0.0.1")
	mockRepo.On("GetInstallState", mock.Anything, mock.Anything).Return(localpackages.Installed, "")
	mockRepo.On("ValidatePackage", mock.Anything, mock.Anything, "0.0.1").Return(nil)
	mockRepo.On("ValidatePackage", mock.Anything, mock.Anything, "0.0.2").Return(nil)
	mockRepo.On("SetInstallState", mock.Anything, mock.Anything, "0.0.2", mock.Anything).Return(nil)
	mockRepo.On("GetInstaller", mock.Anything, mock.Anything, mock.Anything, "0.0.1").Return(installerMock)
	mockRepo.On("GetInstaller", mock.Anything, mock.Anything, mock.Anything, "0.0.2").Return(installerMock)
	mockRepo.On("LockPackage", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockRepo.On("UnlockPackage", mock.Anything, mock.Anything).Return()
	return &mockRepo
}

func repoUninstallMock(pluginInformation *ConfigurePackagePluginInput, installerMock installer.Installer) *repoMock.MockedRepository {
	mockRepo := repoMock.MockedRepository{}
	mockRepo.On("GetInstalledVersion", mock.Anything, mock.Anything).Return("0.0.1")
	mockRepo.On("GetInstallState", mock.Anything, mock.Anything).Return(localpackages.Installed, "")
	mockRepo.On("ValidatePackage", mock.Anything, mock.Anything, "0.0.1").Return(nil).Once()
	mockRepo.On("GetInstaller", mock.Anything, mock.Anything, mock.Anything, "0.0.1").Return(installerMock)
	mockRepo.On("LockPackage", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockRepo.On("UnlockPackage", mock.Anything, mock.Anything).Return()
	return &mockRepo
}

func repoInstallMock_ReadWriteManifest(pluginInformation *ConfigurePackagePluginInput, installerMock installer.Installer, version string, action string) *repoMock.MockedRepository {
	mockRepo := repoMock.MockedRepository{}
	mockRepo.On("ReadManifest", pluginInformation.Name, version).Return([]byte(""), nil)
	mockRepo.On("WriteManifest", pluginInformation.Name, version, mock.Anything).Return(nil)
	mockRepo.On("GetInstalledVersion", mock.Anything, pluginInformation.Name).Return("")
	mockRepo.On("GetInstallState", mock.Anything, pluginInformation.Name).Return(localpackages.None, "")
	mockRepo.On("UnlockPackage", mock.Anything, mock.Anything).Return().Once()
	mockRepo.On("LoadTraces", mock.Anything, mock.Anything).Return(nil)

	if action == InstallAction {
		mockRepo.On("LockPackage", mock.Anything, pluginInformation.Name, "Install").Return(nil).Once()
		mockRepo.On("ValidatePackage", mock.Anything, pluginInformation.Name, version).Return(nil)
		mockRepo.On("GetInstaller", mock.Anything, mock.Anything, pluginInformation.Name, version).Return(installerMock)
		mockRepo.On("SetInstallState", mock.Anything, pluginInformation.Name, version, mock.Anything).Return(nil)
	} else {
		mockRepo.On("LockPackage", mock.Anything, pluginInformation.Name, "Uninstall").Return(nil).Once()

	}
	return &mockRepo
}

func repoInstallMock_ReadWriteManifestHash(pluginInformation *ConfigurePackagePluginInput, installerMock installer.Installer, version string, installedDocVersion string, newpkgDocVersion string, action string) *repoMock.MockedRepository {
	mockRepo := repoMock.MockedRepository{}
	fakeHash_returned := "fwiu49283410rnjkfnksmncsogu0dh7834b1jbnmslg87bjdeb8g7e"
	mockRepo.On("ReadManifestHash", pluginInformation.Name, installedDocVersion).Return([]byte(fakeHash_returned), nil)
	mockRepo.On("WriteManifestHash", pluginInformation.Name, newpkgDocVersion, mock.Anything).Return(nil)
	mockRepo.On("ReadManifest", pluginInformation.Name, newpkgDocVersion).Return([]byte(""), nil)
	mockRepo.On("WriteManifest", pluginInformation.Name, newpkgDocVersion, mock.Anything).Return(nil)
	mockRepo.On("GetInstalledVersion", mock.Anything, pluginInformation.Name).Return("")
	mockRepo.On("GetInstallState", mock.Anything, pluginInformation.Name).Return(localpackages.None, "")
	mockRepo.On("UnlockPackage", mock.Anything, mock.Anything).Return().Once()
	mockRepo.On("LoadTraces", mock.Anything, mock.Anything).Return(nil)

	if action == InstallAction {
		mockRepo.On("LockPackage", mock.Anything, pluginInformation.Name, "Install").Return(nil).Once()
		mockRepo.On("ValidatePackage", mock.Anything, pluginInformation.Name, version).Return(nil)
		mockRepo.On("GetInstaller", mock.Anything, mock.Anything, pluginInformation.Name, version).Return(installerMock)
		mockRepo.On("SetInstallState", mock.Anything, pluginInformation.Name, version, mock.Anything).Return(nil)
	} else {
		mockRepo.On("LockPackage", mock.Anything, pluginInformation.Name, "Uninstall").Return(nil).Once()

	}
	return &mockRepo
}
func pluginOutputWithStatus(status contracts.ResultStatus) contracts.PluginOutputter {
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")
	output := &trace.PluginOutputTrace{Tracer: tracer}
	output.SetStatus(status)
	return output
}

func installerSuccessMock(packageName string, version string) *installerMock.Mock {
	mockInst := installerMock.Mock{}
	mockInst.On("Install", mock.Anything).Return(pluginOutputWithStatus(contracts.ResultStatusSuccess)).Once()
	mockInst.On("Validate", mock.Anything).Return(pluginOutputWithStatus(contracts.ResultStatusSuccess)).Once()
	mockInst.On("PackageName").Return(packageName)
	mockInst.On("Version").Return(version)
	return &mockInst
}

func installerSuccessMock_Install(packageName string, version string, action string) *installerMock.Mock {
	mockInst := installerMock.Mock{}
	if action == InstallAction {
		mockInst.On("Install", mock.Anything).Return(pluginOutputWithStatus(contracts.ResultStatusSuccess)).Once()
		mockInst.On("Validate", mock.Anything).Return(pluginOutputWithStatus(contracts.ResultStatusSuccess)).Once()
		mockInst.On("PackageName").Return(packageName)
		mockInst.On("Version").Return(version)
	}
	return &mockInst
}

func installerRebootMock(packageName string, version string) *installerMock.Mock {
	mockInst := installerMock.Mock{}
	mockInst.On("Install", mock.Anything).Return(pluginOutputWithStatus(contracts.ResultStatusSuccessAndReboot)).Once()
	mockInst.On("PackageName").Return(packageName)
	mockInst.On("Version").Return(version)
	return &mockInst
}

func installerFailedMock(packageName string, version string) *installerMock.Mock {
	mockInst := installerMock.Mock{}
	mockInst.On("Install", mock.Anything).Return(pluginOutputWithStatus(contracts.ResultStatusFailed)).Once()
	mockInst.On("PackageName").Return(packageName)
	mockInst.On("Version").Return(version)
	return &mockInst
}

func installerInvalidMock(packageName string, version string) *installerMock.Mock {
	mockInst := installerMock.Mock{}
	mockInst.On("Install", mock.Anything).Return(pluginOutputWithStatus(contracts.ResultStatusSuccess)).Once()
	mockInst.On("Validate", mock.Anything).Return(pluginOutputWithStatus(contracts.ResultStatusFailed)).Once()
	mockInst.On("PackageName").Return(packageName)
	mockInst.On("Version").Return(version)
	return &mockInst
}

func uninstallerSuccessMock(packageName string, version string) *installerMock.Mock {
	mockInst := installerMock.Mock{}
	mockInst.On("Uninstall", mock.Anything).Return(pluginOutputWithStatus(contracts.ResultStatusSuccess)).Once()
	mockInst.On("PackageName").Return(packageName)
	mockInst.On("Version").Return(version)
	return &mockInst
}

func uninstallerRebootMock(packageName string, version string) *installerMock.Mock {
	mockInst := installerMock.Mock{}
	mockInst.On("Uninstall", mock.Anything).Return(pluginOutputWithStatus(contracts.ResultStatusSuccessAndReboot)).Once()
	mockInst.On("PackageName").Return(packageName)
	mockInst.On("Version").Return(version)
	return &mockInst
}

func uninstallerFailedMock(packageName string, version string) *installerMock.Mock {
	mockInst := installerMock.Mock{}
	mockInst.On("Uninstall", mock.Anything).Return(pluginOutputWithStatus(contracts.ResultStatusFailed)).Once()
	mockInst.On("PackageName").Return(packageName)
	mockInst.On("Version").Return(version)
	return &mockInst
}

func installerFailedWithRollbackMock(packageName string, version string) *installerMock.Mock {
	mockInst := installerMock.Mock{}
	mockInst.On("Install", mock.Anything).Return(pluginOutputWithStatus(contracts.ResultStatusFailed)).Once()
	mockInst.On("Uninstall", mock.Anything).Return(pluginOutputWithStatus(contracts.ResultStatusSuccess)).Once()
	mockInst.On("PackageName").Return(packageName)
	mockInst.On("Version").Return(version)
	return &mockInst
}

func uninstallerSuccessWithRollbackMock(packageName string, version string) *installerMock.Mock {
	mockInst := installerMock.Mock{}
	mockInst.On("Uninstall", mock.Anything).Return(pluginOutputWithStatus(contracts.ResultStatusSuccess)).Once()
	mockInst.On("Install", mock.Anything).Return(pluginOutputWithStatus(contracts.ResultStatusSuccess)).Once()
	mockInst.On("Validate", mock.Anything).Return(pluginOutputWithStatus(contracts.ResultStatusSuccess)).Once()
	mockInst.On("PackageName").Return(packageName)
	mockInst.On("Version").Return(version)
	return &mockInst
}

func uninstallerSuccessWithFailedRollbackMock(packageName string, version string) *installerMock.Mock {
	mockInst := installerMock.Mock{}
	mockInst.On("Uninstall", mock.Anything).Return(pluginOutputWithStatus(contracts.ResultStatusSuccess)).Once()
	mockInst.On("Install", mock.Anything).Return(pluginOutputWithStatus(contracts.ResultStatusFailed)).Once()
	mockInst.On("PackageName").Return(packageName)
	mockInst.On("Version").Return(version)
	return &mockInst
}

func installerNameVersionOnlyMock(packageName string, version string) *installerMock.Mock {
	mockInst := installerMock.Mock{}
	mockInst.On("PackageName").Return(packageName)
	mockInst.On("Version").Return(version)
	return &mockInst
}

func installerNotCalledMock() *installerMock.Mock {
	return &installerMock.Mock{}
}

func selectMockService(service packageservice.PackageService) func(tracer trace.Tracer, input *ConfigurePackagePluginInput, localrepo localpackages.Repository, appCfg *appconfig.SsmagentConfig, bwfacade facade.BirdwatcherFacade, isDocumentService *bool) (packageservice.PackageService, error) {
	return func(tracer trace.Tracer, input *ConfigurePackagePluginInput, localrepo localpackages.Repository, appCfg *appconfig.SsmagentConfig, bwfacade facade.BirdwatcherFacade, isDocumentService *bool) (packageservice.PackageService, error) {
		return service, nil
	}
}

func serviceSuccessMock() *serviceMock.Mock {
	mockService := serviceMock.Mock{}
	mockService.On("GetPackageArnAndVersion", mock.Anything, mock.Anything).Return("packageArn", "0.0.1")
	mockService.On("DownloadManifest", mock.Anything, mock.Anything, mock.Anything).Return("packageArn", "0.0.1", false, nil)
	mockService.On("ReportResult", mock.Anything, mock.Anything).Return(nil)
	return &mockService
}

func serviceSameManifestCacheMock() *serviceMock.Mock {
	mockService := serviceMock.Mock{}
	mockService.On("GetPackageArnAndVersion", mock.Anything, mock.Anything).Return("packageArn", "0.0.1")
	mockService.On("DownloadManifest", mock.Anything, mock.Anything, mock.Anything).Return("packageArn", "0.0.1", true, nil)
	mockService.On("ReportResult", mock.Anything, mock.Anything).Return(nil)
	return &mockService
}

func serviceFailedMock() *serviceMock.Mock {
	mockService := serviceMock.Mock{}
	mockService.On("GetPackageArnAndVersion", mock.Anything, mock.Anything).Return("packageArn", "0.0.1")
	mockService.On("DownloadManifest", mock.Anything, mock.Anything, mock.Anything).Return("", "", false, errors.New("testerror"))
	return &mockService
}

func serviceRebootMock() *serviceMock.Mock {
	return &serviceMock.Mock{}
}

func serviceUpgradeMock() *serviceMock.Mock {
	mockService := serviceMock.Mock{}
	mockService.On("GetPackageArnAndVersion", mock.Anything, mock.Anything).Return("packageArn", "0.0.1")
	mockService.On("DownloadManifest", mock.Anything, mock.Anything, "latest").Return("packageArn", "0.0.2", false, nil)
	mockService.On("DownloadArtifact", mock.Anything, mock.Anything, "0.0.2").Return("/temp/0.0.2", nil)
	mockService.On("ReportResult", mock.Anything, mock.Anything).Return(nil)
	return &mockService
}

func createMockCancelFlag() task.CancelFlag {
	mockCancelFlag := new(task.MockCancelFlag)
	// Setup mocks
	mockCancelFlag.On("Canceled").Return(false)
	mockCancelFlag.On("ShutDown").Return(false)
	mockCancelFlag.On("Wait").Return(false).After(100 * time.Millisecond)

	return mockCancelFlag
}

func createMockIOHandler() iohandler.IOHandler {
	mockIOHandler := new(iohandlermocks.MockIOHandler)

	mockIOHandler.On("SetExitCode", mock.Anything).Return()
	mockIOHandler.On("SetStatus", mock.Anything).Return()
	mockIOHandler.On("AppendInfo", mock.Anything).Return()
	mockIOHandler.On("AppendError", mock.Anything).Return()

	return mockIOHandler
}

func createMockIOHandlerStruct(errorResponse string) *iohandlermocks.MockIOHandler {
	mockIOHandler := iohandlermocks.MockIOHandler{}

	mockIOHandler.On("SetExitCode", mock.Anything).Return()
	mockIOHandler.On("SetStatus", mock.Anything).Return()
	mockIOHandler.On("AppendInfo", mock.Anything).Return()
	mockIOHandler.On("AppendError", errorResponse).Return()

	return &mockIOHandler
}

type ConfigurePackageStubs struct {
	// individual stub functions or interfaces go here with a temp variable for the original version
	fileSysDepStub fileSysDep
	fileSysDepOrig fileSysDep
	stubsSet       bool
}

// Set replaces dependencies with stub versions and saves the original version.
// it should always be followed by defer Clear()
func (m *ConfigurePackageStubs) Set() {
	if m.fileSysDepStub != nil {
		m.fileSysDepOrig = filesysdep
		filesysdep = m.fileSysDepStub
	}
	m.stubsSet = true
}

// Clear resets dependencies to their original values.
func (m *ConfigurePackageStubs) Clear() {
	if m.fileSysDepStub != nil {
		filesysdep = m.fileSysDepOrig
	}
	m.stubsSet = false
}

func setSuccessStubs() *ConfigurePackageStubs {
	stubs := &ConfigurePackageStubs{fileSysDepStub: &FileSysDepStub{}}
	stubs.Set()
	return stubs
}

type FileSysDepStub struct {
	makeFileError   error
	uncompressError error
	removeError     error
	writeError      error
}

func (m *FileSysDepStub) MakeDirExecute(destinationDir string) (err error) {
	return m.makeFileError
}

func (m *FileSysDepStub) Uncompress(src, dest string) error {
	return m.uncompressError
}

func (m *FileSysDepStub) RemoveAll(path string) error {
	return m.removeError
}

func (m *FileSysDepStub) WriteFile(filename string, content string) error {
	return m.writeError
}
