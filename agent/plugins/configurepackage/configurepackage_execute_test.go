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

package configurepackage

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/localpackages"
	repository_mock "github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/localpackages/mock"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/trace"
	"github.com/stretchr/testify/mock"
)

func TestInstallNew(t *testing.T) {
	installerMock := installerSuccessMock("SsmTest", "0.0.1")
	repoMock := &repository_mock.MockedRepository{}
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.1", localpackages.Installing).Return(nil)
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.1", localpackages.Installed).Return(nil)
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")
	output := &trace.PluginOutputTrace{Tracer: tracer}

	executeConfigurePackage(tracer, contextMock, repoMock, installerMock, nil, localpackages.New, output)

	installerMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
}

func TestUpgrade(t *testing.T) {
	uninstallerMock := uninstallerSuccessMock("SsmTest", "0.0.1")
	installerMock := installerSuccessMock("SsmTest", "0.0.2")
	repoMock := &repository_mock.MockedRepository{}
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.1", localpackages.Upgrading).Return(nil)
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.2", localpackages.Installing).Return(nil)
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.2", localpackages.Installed).Return(nil)
	repoMock.On("RemovePackage", mock.Anything, "SsmTest", "0.0.1").Return(nil)
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")
	output := &trace.PluginOutputTrace{Tracer: tracer}

	executeConfigurePackage(tracer, contextMock, repoMock, installerMock, uninstallerMock, localpackages.Installed, output)

	installerMock.AssertExpectations(t)
	uninstallerMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
}

func TestUpgradeFailedUninstall(t *testing.T) {
	uninstallerMock := uninstallerFailedMock("SsmTest", "0.0.1")
	installerMock := installerSuccessMock("SsmTest", "0.0.2")
	repoMock := &repository_mock.MockedRepository{}
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.1", localpackages.Upgrading).Return(nil)
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.2", localpackages.Installing).Return(nil)
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.2", localpackages.Installed).Return(nil)
	repoMock.On("RemovePackage", mock.Anything, "SsmTest", "0.0.1").Return(nil)
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")
	output := &trace.PluginOutputTrace{Tracer: tracer}

	executeConfigurePackage(tracer, contextMock, repoMock, installerMock, uninstallerMock, localpackages.Installed, output)

	installerMock.AssertExpectations(t)
	uninstallerMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
}

func TestUninstall(t *testing.T) {
	uninstallerMock := uninstallerSuccessMock("SsmTest", "0.0.1")
	repoMock := &repository_mock.MockedRepository{}
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.1", localpackages.Uninstalling).Return(nil)
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.1", localpackages.None).Return(nil)
	repoMock.On("RemovePackage", mock.Anything, "SsmTest", "0.0.1").Return(nil)
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")
	output := &trace.PluginOutputTrace{Tracer: tracer}

	executeConfigurePackage(tracer, contextMock, repoMock, nil, uninstallerMock, localpackages.Installed, output)

	uninstallerMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
}

func TestInstall_FailedInstall(t *testing.T) {
	installerMock := installerFailedMock("SsmTest", "0.0.1")
	repoMock := &repository_mock.MockedRepository{}
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.1", localpackages.Installing).Return(nil)
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.1", localpackages.Failed).Return(nil)
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")
	output := &trace.PluginOutputTrace{Tracer: tracer}

	executeConfigurePackage(tracer, contextMock, repoMock, installerMock, nil, localpackages.New, output)

	installerMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
}

func TestInstall_FailedValidate(t *testing.T) {
	installerMock := installerInvalidMock("SsmTest", "0.0.1")
	repoMock := &repository_mock.MockedRepository{}
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.1", localpackages.Installing).Return(nil)
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.1", localpackages.Failed).Return(nil)
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")
	output := &trace.PluginOutputTrace{Tracer: tracer}

	executeConfigurePackage(tracer, contextMock, repoMock, installerMock, nil, localpackages.New, output)

	installerMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
}

func TestUninstall_Failed(t *testing.T) {
	uninstallerMock := uninstallerFailedMock("SsmTest", "0.0.1")
	repoMock := &repository_mock.MockedRepository{}
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.1", localpackages.Uninstalling).Return(nil)
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.1", localpackages.Failed).Return(nil)
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")
	output := &trace.PluginOutputTrace{Tracer: tracer}

	executeConfigurePackage(tracer, contextMock, repoMock, nil, uninstallerMock, localpackages.Installed, output)

	uninstallerMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
}

func TestRollback(t *testing.T) {
	uninstallerMock := uninstallerSuccessWithRollbackMock("SsmTest", "0.0.1")
	installerMock := installerFailedWithRollbackMock("SsmTest", "0.0.2")
	repoMock := &repository_mock.MockedRepository{}
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.1", localpackages.Upgrading).Return(nil)
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.2", localpackages.Installing).Return(nil)
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.2", localpackages.RollbackUninstall).Return(nil)
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.1", localpackages.RollbackInstall).Return(nil)
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.1", localpackages.Installed).Return(nil)
	repoMock.On("RemovePackage", mock.Anything, "SsmTest", "0.0.2").Return(nil)
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")
	output := &trace.PluginOutputTrace{Tracer: tracer}

	executeConfigurePackage(tracer, contextMock, repoMock, installerMock, uninstallerMock, localpackages.Installed, output)

	installerMock.AssertExpectations(t)
	uninstallerMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
}

func TestRollbackFailed(t *testing.T) {
	uninstallerMock := uninstallerSuccessWithFailedRollbackMock("SsmTest", "0.0.1")
	installerMock := installerFailedWithRollbackMock("SsmTest", "0.0.2")
	repoMock := &repository_mock.MockedRepository{}
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.1", localpackages.Upgrading).Return(nil)
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.2", localpackages.Installing).Return(nil)
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.2", localpackages.RollbackUninstall).Return(nil)
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.1", localpackages.RollbackInstall).Return(nil)
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.1", localpackages.Failed).Return(nil)
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")
	output := &trace.PluginOutputTrace{Tracer: tracer}

	executeConfigurePackage(tracer, contextMock, repoMock, installerMock, uninstallerMock, localpackages.Installed, output)

	installerMock.AssertExpectations(t)
	uninstallerMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
}

func TestUninstallReboot(t *testing.T) {
	uninstallerMock := uninstallerRebootMock("SsmTest", "0.0.1")
	repoMock := &repository_mock.MockedRepository{}
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.1", localpackages.Uninstalling).Return(nil)
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")
	output := &trace.PluginOutputTrace{Tracer: tracer}

	executeConfigurePackage(tracer, contextMock, repoMock, nil, uninstallerMock, localpackages.Installed, output)

	uninstallerMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
}

func TestUninstallAfterReboot(t *testing.T) {
	uninstallerMock := uninstallerSuccessMock("SsmTest", "0.0.1")
	repoMock := &repository_mock.MockedRepository{}
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.1", localpackages.Uninstalling).Return(nil)
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.1", localpackages.None).Return(nil)
	repoMock.On("RemovePackage", mock.Anything, "SsmTest", "0.0.1").Return(nil)
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")
	output := &trace.PluginOutputTrace{Tracer: tracer}

	executeConfigurePackage(tracer, contextMock, repoMock, nil, uninstallerMock, localpackages.Uninstalling, output)

	uninstallerMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
}

func TestInstallReboot(t *testing.T) {
	installerMock := installerRebootMock("SsmTest", "0.0.1")
	repoMock := &repository_mock.MockedRepository{}
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.1", localpackages.Installing).Return(nil)
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")
	output := &trace.PluginOutputTrace{Tracer: tracer}

	executeConfigurePackage(tracer, contextMock, repoMock, installerMock, nil, localpackages.New, output)

	installerMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
}

func TestInstallAfterReboot(t *testing.T) {
	installerMock := installerSuccessMock("SsmTest", "0.0.1")
	repoMock := &repository_mock.MockedRepository{}
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.1", localpackages.Installing).Return(nil)
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.1", localpackages.Installed).Return(nil)
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")
	output := &trace.PluginOutputTrace{Tracer: tracer}

	executeConfigurePackage(tracer, contextMock, repoMock, installerMock, nil, localpackages.Installing, output)

	installerMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
}

func TestUpgradeAfterUninstallReboot(t *testing.T) {
	uninstallerMock := uninstallerSuccessMock("SsmTest", "0.0.1")
	installerMock := installerSuccessMock("SsmTest", "0.0.2")
	repoMock := &repository_mock.MockedRepository{}
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.1", localpackages.Upgrading).Return(nil)
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.2", localpackages.Installing).Return(nil)
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.2", localpackages.Installed).Return(nil)
	repoMock.On("RemovePackage", mock.Anything, "SsmTest", "0.0.1").Return(nil)
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")
	output := &trace.PluginOutputTrace{Tracer: tracer}

	executeConfigurePackage(tracer, contextMock, repoMock, installerMock, uninstallerMock, localpackages.Uninstalling, output)

	installerMock.AssertExpectations(t)
	uninstallerMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
}

func TestUpgradeAfterInstallReboot(t *testing.T) {
	uninstallerMock := installerNameVersionOnlyMock("SsmTest", "0.0.1")
	installerMock := installerSuccessMock("SsmTest", "0.0.2")
	repoMock := &repository_mock.MockedRepository{}
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.2", localpackages.Installing).Return(nil)
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.2", localpackages.Installed).Return(nil)
	repoMock.On("RemovePackage", mock.Anything, "SsmTest", "0.0.1").Return(nil)
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")
	output := &trace.PluginOutputTrace{Tracer: tracer}

	executeConfigurePackage(tracer, contextMock, repoMock, installerMock, uninstallerMock, localpackages.Installing, output)

	installerMock.AssertExpectations(t)
	uninstallerMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
}

func TestRollbackAfterUninstallReboot(t *testing.T) {
	uninstallerMock := installerSuccessMock("SsmTest", "0.0.1")
	installerMock := uninstallerSuccessMock("SsmTest", "0.0.2")
	repoMock := &repository_mock.MockedRepository{}
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.2", localpackages.RollbackUninstall).Return(nil)
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.1", localpackages.RollbackInstall).Return(nil)
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.1", localpackages.Installed).Return(nil)
	repoMock.On("RemovePackage", mock.Anything, "SsmTest", "0.0.2").Return(nil)
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")
	output := &trace.PluginOutputTrace{Tracer: tracer}

	executeConfigurePackage(tracer, contextMock, repoMock, installerMock, uninstallerMock, localpackages.RollbackUninstall, output)

	installerMock.AssertExpectations(t)
	uninstallerMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
}

func TestRollbackAfterInstallReboot(t *testing.T) {
	uninstallerMock := installerSuccessMock("SsmTest", "0.0.1")
	installerMock := installerNameVersionOnlyMock("SsmTest", "0.0.2")
	repoMock := &repository_mock.MockedRepository{}
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.1", localpackages.RollbackInstall).Return(nil)
	repoMock.On("SetInstallState", mock.Anything, "SsmTest", "0.0.1", localpackages.Installed).Return(nil)
	repoMock.On("RemovePackage", mock.Anything, "SsmTest", "0.0.2").Return(nil)
	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")
	output := &trace.PluginOutputTrace{Tracer: tracer}

	executeConfigurePackage(tracer, contextMock, repoMock, installerMock, uninstallerMock, localpackages.RollbackInstall, output)

	installerMock.AssertExpectations(t)
	uninstallerMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
}
