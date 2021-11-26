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

package managers

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/packagemanagers"
	pmMock "github.com/aws/amazon-ssm-agent/agent/setupcli/managers/packagemanagers/mocks"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/servicemanagers"
	smMock "github.com/aws/amazon-ssm-agent/agent/setupcli/managers/servicemanagers/mocks"
	"github.com/stretchr/testify/assert"
)

func storeMockedFunctions() func() {
	getServiceManagerStorage := getServiceManager
	getPackageManagerStorage := getPackageManager
	getAllPackageManagersStorage := getAllPackageManagers

	return func() {
		getServiceManager = getServiceManagerStorage
		getPackageManager = getPackageManagerStorage
		getAllPackageManagers = getAllPackageManagersStorage
	}
}

func setPackageManagerCache(manager packagemanagers.PackageManager) func() {
	selectedPackageManagerCacheStore := selectedPackageManagerCache
	selectedPackageManagerCache = manager
	return func() {
		selectedPackageManagerCache = selectedPackageManagerCacheStore
	}
}

func setServiceManagerCache(manager servicemanagers.ServiceManager) func() {
	selectedServiceManagerCacheStore := selectedServiceManagerCache
	selectedServiceManagerCache = manager
	return func() {
		selectedServiceManagerCache = selectedServiceManagerCacheStore
	}
}

var logger = log.NewMockLog()

func TestGetPackageManager_Cached(t *testing.T) {
	replaceMocked := storeMockedFunctions()
	defer replaceMocked()

	restorePM := setPackageManagerCache(packagemanagers.Snap)
	defer restorePM()

	getPackageManager = func(managerType packagemanagers.PackageManager) (packagemanagers.IPackageManager, bool) {
		assert.Equal(t, packagemanagers.Snap, managerType)

		mockManager := &pmMock.IPackageManager{}
		return mockManager, true
	}

	_, err := GetPackageManager(logger)
	assert.NoError(t, err)
}

func TestGetServiceManager_Cached(t *testing.T) {
	replaceMocked := storeMockedFunctions()
	defer replaceMocked()

	restoreSM := setServiceManagerCache(servicemanagers.Snap)
	defer restoreSM()

	restorePM := setPackageManagerCache(packagemanagers.Snap)
	defer restorePM()

	getServiceManager = func(managerType servicemanagers.ServiceManager) (servicemanagers.IServiceManager, bool) {
		assert.Equal(t, servicemanagers.Snap, managerType)

		mockManager := &smMock.IServiceManager{}
		return mockManager, true
	}

	_, err := GetServiceManager(logger)
	assert.NoError(t, err)
}

func TestGetServiceManager_PackageManagerUndefined(t *testing.T) {
	replaceMocked := storeMockedFunctions()
	defer replaceMocked()

	restoreSM := setServiceManagerCache(servicemanagers.Snap)
	defer restoreSM()

	getServiceManager = func(managerType servicemanagers.ServiceManager) (servicemanagers.IServiceManager, bool) {
		assert.Equal(t, servicemanagers.Snap, managerType)

		mockManager := &smMock.IServiceManager{}
		return mockManager, true
	}

	defer func() {
		if err := recover(); err == nil {
			assert.Fail(t, "Expected panic because package manager is not available")
		}
	}()

	GetServiceManager(logger)
	assert.Fail(t, "Should not reach here because package manager is not available")
}

func TestGetServiceManager_FailedGetServiceManagerInner(t *testing.T) {
	replaceMocked := storeMockedFunctions()
	defer replaceMocked()

	restoreSM := setServiceManagerCache(servicemanagers.Undefined)
	defer restoreSM()

	restorePM := setPackageManagerCache(packagemanagers.Snap)
	defer restorePM()

	getPackageManager = func(managerType packagemanagers.PackageManager) (packagemanagers.IPackageManager, bool) {

		mockManager := &pmMock.IPackageManager{}
		mockManager.On("GetSupportedServiceManagers").Return([]servicemanagers.ServiceManager{servicemanagers.SystemCtl, servicemanagers.Upstart})

		return mockManager, true
	}

	getServiceManager = func(managerType servicemanagers.ServiceManager) (servicemanagers.IServiceManager, bool) {
		return nil, false
	}

	defer func() {
		if err := recover(); err == nil {
			assert.Fail(t, "Expected panic because package manager returned undefined service manager")
		}
	}()

	GetServiceManager(logger)
	assert.Fail(t, "Should not reach here package manager returned undefined service manager")
}

func TestSetServiceManager_FailedGetAgentStatus(t *testing.T) {
	replaceMocked := storeMockedFunctions()
	defer replaceMocked()

	restoreSM := setServiceManagerCache(servicemanagers.Undefined)
	defer restoreSM()

	restorePM := setPackageManagerCache(packagemanagers.Snap)
	defer restorePM()

	getPackageManager = func(managerType packagemanagers.PackageManager) (packagemanagers.IPackageManager, bool) {

		mockManager := &pmMock.IPackageManager{}
		mockManager.On("GetSupportedServiceManagers").Return([]servicemanagers.ServiceManager{servicemanagers.SystemCtl})

		return mockManager, true
	}

	getServiceManager = func(managerType servicemanagers.ServiceManager) (servicemanagers.IServiceManager, bool) {
		assert.Equal(t, managerType, servicemanagers.SystemCtl)

		mockManager := &smMock.IServiceManager{}
		mockManager.On("IsManagerEnvironment").Return(false)
		mockManager.On("GetName").Return("SomeName")
		return mockManager, true
	}

	err := setServiceManager(logger)
	assert.NoError(t, err)
	assert.Equal(t, servicemanagers.Undefined, selectedServiceManagerCache)
}

func TestSetServiceManager_IsNotManagerEnvironment(t *testing.T) {
	replaceMocked := storeMockedFunctions()
	defer replaceMocked()

	restoreSM := setServiceManagerCache(servicemanagers.Undefined)
	defer restoreSM()

	restorePM := setPackageManagerCache(packagemanagers.Snap)
	defer restorePM()

	getPackageManager = func(managerType packagemanagers.PackageManager) (packagemanagers.IPackageManager, bool) {

		mockManager := &pmMock.IPackageManager{}
		mockManager.On("GetSupportedServiceManagers").Return([]servicemanagers.ServiceManager{servicemanagers.SystemCtl})

		return mockManager, true
	}

	getServiceManager = func(managerType servicemanagers.ServiceManager) (servicemanagers.IServiceManager, bool) {
		assert.Equal(t, managerType, servicemanagers.SystemCtl)

		mockManager := &smMock.IServiceManager{}
		mockManager.On("IsManagerEnvironment").Return(false)
		mockManager.On("GetName").Return("SomeName")
		return mockManager, true
	}

	err := setServiceManager(logger)
	assert.NoError(t, err)
	assert.Equal(t, servicemanagers.Undefined, selectedServiceManagerCache)
}

func TestSetServiceManager_IsManagerEnvironment(t *testing.T) {
	replaceMocked := storeMockedFunctions()
	defer replaceMocked()

	restoreSM := setServiceManagerCache(servicemanagers.Undefined)
	defer restoreSM()

	restorePM := setPackageManagerCache(packagemanagers.Snap)
	defer restorePM()

	getPackageManager = func(managerType packagemanagers.PackageManager) (packagemanagers.IPackageManager, bool) {
		mockManager := &pmMock.IPackageManager{}
		mockManager.On("GetSupportedServiceManagers").Return([]servicemanagers.ServiceManager{servicemanagers.SystemCtl})

		return mockManager, true
	}

	getServiceManager = func(managerType servicemanagers.ServiceManager) (servicemanagers.IServiceManager, bool) {
		assert.Equal(t, managerType, servicemanagers.SystemCtl)

		mockManager := &smMock.IServiceManager{}
		mockManager.On("IsManagerEnvironment").Return(true)
		mockManager.On("GetName").Return("SomeName")
		return mockManager, true
	}

	err := setServiceManager(logger)
	assert.NoError(t, err)
	assert.Equal(t, servicemanagers.SystemCtl, selectedServiceManagerCache)
}

func TestSetPackageManager_NoAvailablePackageManagers(t *testing.T) {
	replaceMocked := storeMockedFunctions()
	defer replaceMocked()

	restoreSM := setServiceManagerCache(servicemanagers.Undefined)
	defer restoreSM()

	restorePM := setPackageManagerCache(packagemanagers.Undefined)
	defer restorePM()

	getAllPackageManagers = func() []packagemanagers.IPackageManager {
		return []packagemanagers.IPackageManager{}
	}

	err := setPackageManager(logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no supported package manager found")
	assert.Equal(t, packagemanagers.Undefined, selectedPackageManagerCache)
}

func TestSetPackageManager_NoPackageManagerEnvironment(t *testing.T) {
	replaceMocked := storeMockedFunctions()
	defer replaceMocked()

	restoreSM := setServiceManagerCache(servicemanagers.Undefined)
	defer restoreSM()

	restorePM := setPackageManagerCache(packagemanagers.Undefined)
	defer restorePM()

	getAllPackageManagers = func() []packagemanagers.IPackageManager {
		mockManager := &pmMock.IPackageManager{}
		mockManager.On("IsManagerEnvironment").Return(false)
		mockManager.On("GetName").Return("SomeManagerName")

		return []packagemanagers.IPackageManager{mockManager}
	}

	err := setPackageManager(logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no supported package manager found")
	assert.Equal(t, packagemanagers.Undefined, selectedPackageManagerCache)
}

func TestSetPackageManager_ErrorCheckingIfAgentInstalled(t *testing.T) {
	replaceMocked := storeMockedFunctions()
	defer replaceMocked()

	restoreSM := setServiceManagerCache(servicemanagers.Undefined)
	defer restoreSM()

	restorePM := setPackageManagerCache(packagemanagers.Undefined)
	defer restorePM()

	getAllPackageManagers = func() []packagemanagers.IPackageManager {
		mockManager := &pmMock.IPackageManager{}
		mockManager.On("IsManagerEnvironment").Return(true)
		mockManager.On("IsAgentInstalled").Return(false, fmt.Errorf("SomeError"))
		mockManager.On("GetName").Return("SomeManagerName")
		mockManager.On("GetType").Return(packagemanagers.Rpm)

		return []packagemanagers.IPackageManager{mockManager}
	}

	err := setPackageManager(logger)
	assert.NoError(t, err)
	assert.Equal(t, packagemanagers.Rpm, selectedPackageManagerCache)
}

func TestSetPackageManager_SelectFirstManagerWhereAgentInstalled(t *testing.T) {
	replaceMocked := storeMockedFunctions()
	defer replaceMocked()

	restoreSM := setServiceManagerCache(servicemanagers.Undefined)
	defer restoreSM()

	restorePM := setPackageManagerCache(packagemanagers.Undefined)
	defer restorePM()

	getAllPackageManagers = func() []packagemanagers.IPackageManager {
		mockManagerNotInstalled := &pmMock.IPackageManager{}
		mockManagerNotInstalled.On("IsManagerEnvironment").Return(true)
		mockManagerNotInstalled.On("IsAgentInstalled").Return(false, nil)
		mockManagerNotInstalled.On("GetName").Return("SNAP")
		mockManagerNotInstalled.On("GetType").Return(packagemanagers.Snap)

		mockManagerInstalled1 := &pmMock.IPackageManager{}
		mockManagerInstalled1.On("IsManagerEnvironment").Return(true)
		mockManagerInstalled1.On("IsAgentInstalled").Return(true, nil)
		mockManagerInstalled1.On("GetName").Return("DPKG")
		mockManagerInstalled1.On("GetType").Return(packagemanagers.Dpkg)

		mockManagerInstalled2 := &pmMock.IPackageManager{}
		mockManagerInstalled2.On("IsManagerEnvironment").Return(true)
		// Even though true is set here, we don't select RPM because Dpkg is earlier in the list and is marked as installed
		mockManagerInstalled2.On("IsAgentInstalled").Return(true, nil)
		mockManagerInstalled2.On("GetName").Return("RPM")
		mockManagerInstalled2.On("GetType").Return(packagemanagers.Rpm)

		return []packagemanagers.IPackageManager{mockManagerNotInstalled, mockManagerInstalled1, mockManagerInstalled2}
	}

	err := setPackageManager(logger)
	assert.NoError(t, err)
	assert.Equal(t, packagemanagers.Dpkg, selectedPackageManagerCache)
}
