// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
package coremanager

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/agentlogstocloudwatch/cloudwatchlogspublisher"
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	moduleMock "github.com/aws/amazon-ssm-agent/agent/contracts/mocks"
	"github.com/aws/amazon-ssm-agent/agent/framework/coremodules"
	"github.com/aws/amazon-ssm-agent/agent/mocks/context"
	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/agent/rebooter"
	rebootMock "github.com/aws/amazon-ssm-agent/agent/rebooter/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// Setting up the CoreManagerTestSuite struct
type CoreManagerTestSuite struct {
	suite.Suite
	contextMock         *context.Mock
	logMock             *log.Mock
	coreManager         ICoreManager
	moduleMock          *moduleMock.ICoreModuleWrapper
	rebootMock          *rebootMock.IRebootType
	coreModulesMock     coremodules.ModuleRegistry
	cloudwatchPublisher *cloudwatchlogspublisher.CloudWatchPublisher
}

// Initialize mock struct and objects in test suite.
func (suite *CoreManagerTestSuite) SetupTest() {
	logMock := log.NewMockLog()
	contextMock := context.NewMockDefault()
	coreModulesMock := make(coremodules.ModuleRegistry, 1)
	moduleMock := new(moduleMock.ICoreModuleWrapper)
	coreModulesMock[0] = moduleMock
	rebootMock := new(rebootMock.IRebootType)
	cloudwatchPublisher := cloudwatchlogspublisher.NewCloudWatchPublisher(contextMock)

	suite.contextMock = contextMock
	suite.logMock = logMock
	suite.moduleMock = moduleMock
	suite.rebootMock = rebootMock
	cm := &CoreManager{
		context:             contextMock,
		coreModules:         coreModulesMock,
		cloudwatchPublisher: cloudwatchPublisher,
		rebooter:            rebootMock,
	}
	suite.coreManager = cm
	suite.moduleMock.On("ModuleStop", mock.Anything).Return(nil)
	suite.moduleMock.On("ModuleExecute", mock.Anything).Return(nil)
	suite.moduleMock.On("ModuleName").Return("TestExecuteModule")
	suite.rebootMock.On("RebootMachine", mock.Anything).Return(nil)
}

// Testing the coremanager API without sending any signal.
func (suite *CoreManagerTestSuite) TestCoremanager_Start_WithoutSignal() {
	ch := make(chan rebooter.RebootType)
	suite.rebootMock.On("GetChannel").Return(ch)
	suite.coreManager.Start()
	close(ch)
	suite.rebootMock.AssertNotCalled(suite.T(), "RebootMachine", mock.Anything)
	suite.moduleMock.AssertNotCalled(suite.T(), "ModuleStop")
}

// CoreManager Start api test, send the update signal
// This function mainly tested the Start() Api in CoreManager.
// During the Start() function, it create a new goroutine for monitoring reboot signal.
// The test function will send a RebootRequestTypeUpdate signal and check whether the
// corresponding method has bee called
func (suite *CoreManagerTestSuite) TestCoreManager_Start_Update() {
	ch := make(chan rebooter.RebootType)
	suite.rebootMock.On("GetChannel").Return(ch)

	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func(wgc *sync.WaitGroup) {
		defer wgc.Done()
		suite.coreManager.Start()
		// coremanager start launch a new go routine in stopModules function, sleep one second to wait the routine launched
		time.Sleep(100 * time.Millisecond)
		suite.moduleMock.AssertNotCalled(suite.T(), "ModuleName")
		suite.moduleMock.AssertNotCalled(suite.T(), "ModuleStop", mock.Anything)
		suite.moduleMock.AssertCalled(suite.T(), "ModuleExecute", mock.Anything)
		suite.rebootMock.AssertCalled(suite.T(), "GetChannel")
		suite.rebootMock.AssertNotCalled(suite.T(), "RebootMachine", mock.Anything)
	}(wg)
	// Send update type signal to the channel
	ch <- rebooter.RebootRequestTypeUpdate
	close(ch)
	wg.Wait()
}

// CoreManager Start api test, send the reboot signal
func (suite *CoreManagerTestSuite) TestCoreManager_Start_Reboot() {
	ch := make(chan rebooter.RebootType)
	suite.rebootMock.On("GetChannel").Return(ch)

	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func(wgc *sync.WaitGroup) {
		defer wgc.Done()
		suite.coreManager.Start()
		time.Sleep(100 * time.Millisecond)
		suite.moduleMock.AssertCalled(suite.T(), "ModuleStop", mock.Anything)
		suite.rebootMock.AssertCalled(suite.T(), "GetChannel")
		suite.rebootMock.AssertCalled(suite.T(), "RebootMachine", mock.Anything)
	}(wg)
	// Send reboot signal to the channel and close it
	ch <- rebooter.RebootRequestTypeReboot
	close(ch)
	wg.Wait()
}

// Unit testing function for Stop() method
func (suite *CoreManagerTestSuite) TestCoreManager_Stop() {
	suite.coreManager.Stop()
	suite.moduleMock.AssertCalled(suite.T(), "ModuleStop", mock.Anything)
	suite.moduleMock.AssertNotCalled(suite.T(), "ModuleName")
}

func TestCoreManagerTestSuite(t *testing.T) {
	suite.Run(t, new(CoreManagerTestSuite))
}

func (suite *CoreManagerTestSuite) TestInitializeBookkeepingLocations() {

	tests := []struct {
		name            string
		shortInstanceID string
		setupFn         func(string) error
		expectedResult  bool
		expectedDirs    []string // List of directories that should exist
	}{
		{
			name:            "Success case - all directories created",
			shortInstanceID: "test-instance",
			setupFn:         func(string) error { return nil },
			expectedResult:  true,
			expectedDirs: []string{
				filepath.Join("test-instance", appconfig.DefaultDocumentRootDirName, appconfig.DefaultLocationOfState, appconfig.DefaultLocationOfPending),
				filepath.Join("test-instance", appconfig.DefaultDocumentRootDirName, appconfig.DefaultLocationOfState, appconfig.DefaultLocationOfCurrent),
				filepath.Join("test-instance", appconfig.DefaultDocumentRootDirName, appconfig.DefaultLocationOfState, appconfig.DefaultLocationOfCorrupt),
				filepath.Join("test-instance", appconfig.LongRunningPluginsLocation, appconfig.LongRunningPluginDataStoreLocation),
				filepath.Join("test-instance", appconfig.RepliesRootDirName),
				filepath.Join("test-instance", appconfig.RepliesMGSRootDirName),
				filepath.Join("test-instance", appconfig.LongRunningPluginsLocation, appconfig.LongRunningPluginsHealthCheck),
				filepath.Join("test-instance", appconfig.InventoryRootDirName),
				filepath.Join("test-instance", appconfig.InventoryRootDirName, appconfig.CustomInventoryRootDirName),
				filepath.Join("test-instance", appconfig.InventoryRootDirName, appconfig.FileInventoryRootDirName),
				filepath.Join("test-instance", appconfig.InventoryRootDirName, appconfig.RoleInventoryRootDirName),
			},
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			tempDir, err := os.MkdirTemp("", "test-bookkeeping-*")
			if err != nil {
				suite.T().Fatalf("Failed to create temp directory: %v", err)
			}
			defer os.RemoveAll(tempDir) // Clean up after test
			// Create a temporary directory for testing

			// Store original paths
			originalDataStorePath := appconfig.DefaultDataStorePath

			// Override paths for testing
			appconfig.DefaultDataStorePath = tempDir

			// Restore original paths after test
			defer func() {
				appconfig.DefaultDataStorePath = originalDataStorePath
			}()

			if err := tt.setupFn(tempDir); err != nil {
				suite.T().Fatalf("Failed to setup test: %v", err)
			}

			// Run the function
			result := initializeBookkeepingLocations(suite.logMock, tt.shortInstanceID)

			// Verify result
			suite.Equal(tt.expectedResult, result)

			// Verify directories
			for _, expectedDir := range tt.expectedDirs {
				fullPath := filepath.Join(tempDir, expectedDir)
				exists, err := directoryExists(fullPath)
				suite.NoError(err)
				suite.True(exists, "Directory should exist: %s", fullPath)
			}
		})
	}
}

// Helper function to check if directory exists
func directoryExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return info.IsDir(), nil
}
