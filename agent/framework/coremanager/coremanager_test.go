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
	"sync"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/agentlogstocloudwatch/cloudwatchlogspublisher"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	moduleMock "github.com/aws/amazon-ssm-agent/agent/contracts/mocks"
	"github.com/aws/amazon-ssm-agent/agent/framework/coremodules"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/rebooter"
	rebootMock "github.com/aws/amazon-ssm-agent/agent/rebooter/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// Setting up the CoreManagerTestSuite struct
type CoreManagerTestSuite struct {
	suite.Suite
	contextMock *context.Mock
	logMock     *log.Mock
	coreManager ICoreManager
	moduleMock  *moduleMock.ICoreModule
	rebootMock  *rebootMock.IRebootType
}

// Initialize mock struct and objects in test suite.
func (suite *CoreManagerTestSuite) SetupTest() {
	logMock := log.NewMockLog()
	contextMock := context.NewMockDefault()
	coreModulesMock := make(coremodules.ModuleRegistry, 1)
	moduleMock := new(moduleMock.ICoreModule)
	coreModulesMock[0] = moduleMock
	rebootMock := new(rebootMock.IRebootType)
	cloudwatchPublisher := &cloudwatchlogspublisher.CloudWatchPublisher{}

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
	suite.moduleMock.On("ModuleRequestStop", mock.Anything).Return(nil)
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
	suite.moduleMock.AssertNotCalled(suite.T(), "ModuleRequestStop", mock.Anything)
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
	go func(wgc *sync.WaitGroup) {
		wgc.Add(1)
		defer wgc.Done()
		suite.coreManager.Start()
		// coremanager start launch a new go routine in stopModules function, sleep one second to wait the routine launched
		time.Sleep(100 * time.Millisecond)
		suite.moduleMock.AssertNotCalled(suite.T(), "ModuleName")
		suite.moduleMock.AssertNotCalled(suite.T(), "ModuleRequestStop", mock.Anything)
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
	go func(wgc *sync.WaitGroup) {
		wgc.Add(1)
		defer wgc.Done()
		suite.coreManager.Start()
		time.Sleep(100 * time.Millisecond)
		suite.moduleMock.AssertCalled(suite.T(), "ModuleRequestStop", mock.Anything)
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
	suite.moduleMock.AssertCalled(suite.T(), "ModuleRequestStop", contracts.StopTypeHardStop)
	suite.moduleMock.AssertNotCalled(suite.T(), "ModuleRequestStop", contracts.StopTypeSoftStop)
	suite.moduleMock.AssertNotCalled(suite.T(), "ModuleName")
}

func TestCoreManagerTestSuite(t *testing.T) {
	suite.Run(t, new(CoreManagerTestSuite))
}
