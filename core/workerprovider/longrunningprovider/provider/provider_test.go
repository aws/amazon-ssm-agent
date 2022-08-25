// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package provider implements logic for allowing interaction with worker processes
package provider

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	identityMocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/aws/amazon-ssm-agent/common/message"
	contextmocks "github.com/aws/amazon-ssm-agent/core/app/context/mocks"
	"github.com/aws/amazon-ssm-agent/core/executor"
	executormocks "github.com/aws/amazon-ssm-agent/core/executor/mocks"
	"github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type WorkerProviderTestSuite struct {
	suite.Suite
	mockLog  log.T
	process  *model.Process
	provider *WorkerProvider
	exec     *executormocks.IExecutor
	config   *model.WorkerConfig
	context  *contextmocks.ICoreAgentContext
}

func (suite *WorkerProviderTestSuite) SetupTest() {
	coreContext := &contextmocks.ICoreAgentContext{}

	agentIdentity := &identityMocks.IAgentIdentity{}
	agentIdentity.On("InstanceID").Return("i-1203030", nil)

	mockLog := log.NewMockLog()
	suite.mockLog = mockLog
	suite.exec = &executormocks.IExecutor{}
	suite.config = &model.WorkerConfig{
		Name: model.SSMAgentWorkerName,
	}

	coreContext.On("Identity").Return(agentIdentity)
	coreContext.On("With", mock.Anything).Return(coreContext)
	coreContext.On("Log").Return(mockLog)

	suite.provider = NewWorkerProvider(coreContext, suite.exec)
	suite.provider.exec = suite.exec
	suite.process = &model.Process{
		Pid:    1000,
		Status: model.Active,
	}
}

// Execute the test suite
func TestWorkerProviderTestSuite(t *testing.T) {
	suite.Run(t, new(WorkerProviderTestSuite))
}

func (suite *WorkerProviderTestSuite) TestStart_NewWorker() {
	var pingResults []*message.Message
	var processes []executor.OsProcess
	configs := make(map[string]*model.WorkerConfig)
	configs[suite.config.Name] = suite.config

	suite.exec.On("Start", suite.config).Return(suite.process, nil)
	suite.exec.On("Processes").Return(processes, nil)

	suite.provider.Start(configs, pingResults)
	suite.exec.AssertExpectations(suite.T())

	assert.Equal(suite.T(), len(suite.provider.workerPool), 1)
	worker := suite.provider.workerPool[model.SSMAgentWorkerName]
	assert.Equal(suite.T(), len(worker.Processes), 1)
	assert.Equal(suite.T(), worker.Processes[1000].Pid, 1000)
	assert.Equal(suite.T(), worker.Processes[1000].Status, model.Active)
}

func (suite *WorkerProviderTestSuite) TestStartProcess_NoWorkerConfig() {
	var emptyPingResults []*message.Message
	var emptyProcesses []executor.OsProcess
	emptyConfig := make(map[string]*model.WorkerConfig)

	suite.exec.On("Processes").Return(emptyProcesses, nil)

	suite.provider.Start(emptyConfig, emptyPingResults)
	suite.exec.AssertExpectations(suite.T())

	assert.Equal(suite.T(), len(suite.provider.workerPool), 0)
}

func (suite *WorkerProviderTestSuite) TestStartProcess_WorkingIsRunning_HealthPing() {
	var pingResults []*message.Message
	var emptyProcesses []executor.OsProcess

	configs := make(map[string]*model.WorkerConfig)
	configs[suite.config.Name] = suite.config

	msg, _ := message.CreateTerminateWorkerResult(model.SSMAgentWorkerName, message.LongRunning, 1, true)
	pingResults = append(pingResults, msg)

	suite.exec.On("Processes").Return(emptyProcesses, nil)
	suite.exec.On("Kill", mock.Anything).Return(nil)
	suite.exec.On("Start", suite.config).Return(suite.process, nil)

	suite.provider.Start(configs, pingResults)
	suite.exec.AssertExpectations(suite.T())

	assert.Equal(suite.T(), len(suite.provider.workerPool), 1)
	worker := suite.provider.workerPool[model.SSMAgentWorkerName]
	assert.Equal(suite.T(), len(worker.Processes), 1)
	assert.Equal(suite.T(), worker.Processes[1000].Pid, 1000)
	assert.Equal(suite.T(), worker.Processes[1000].Status, model.Active)
}

func (suite *WorkerProviderTestSuite) TestKillAllWorkerProcesses_Success() {

	successPid := 10
	failurePid := 11
	suite.provider.workerPool[model.SSMAgentWorkerName] = &model.Worker{
		Name:      model.SSMAgentWorkerName,
		Config:    &model.WorkerConfig{},
		Processes: make(map[int]*model.Process),
	}

	suite.provider.workerPool[model.SSMAgentWorkerName].Processes[10] = &model.Process{
		Pid:    successPid,
		Status: model.Unknown,
	}

	suite.provider.workerPool[model.SSMAgentWorkerName].Processes[11] = &model.Process{
		Pid:    failurePid,
		Status: model.Active,
	}

	suite.exec.On("Kill", mock.Anything).Return(nil)

	suite.provider.KillAllWorkerProcesses()
	suite.exec.AssertExpectations(suite.T())

	assert.Equal(suite.T(), len(suite.provider.workerPool), 1)
	worker := suite.provider.workerPool[model.SSMAgentWorkerName]
	assert.Equal(suite.T(), len(worker.Processes), 0)
}

func (suite *WorkerProviderTestSuite) TestKillAllWorkerProcesses_Failure() {

	successPid := 10
	failurePid := 11
	suite.provider.workerPool[model.SSMAgentWorkerName] = &model.Worker{
		Name:      model.SSMAgentWorkerName,
		Config:    &model.WorkerConfig{},
		Processes: make(map[int]*model.Process),
	}

	suite.provider.workerPool[model.SSMAgentWorkerName].Processes[10] = &model.Process{
		Pid:    successPid,
		Status: model.Unknown,
	}

	suite.provider.workerPool[model.SSMAgentWorkerName].Processes[11] = &model.Process{
		Pid:    failurePid,
		Status: model.Active,
	}

	suite.exec.On("Kill", successPid).Return(nil)
	suite.exec.On("Kill", failurePid).Return(fmt.Errorf("SomeError"))

	suite.provider.KillAllWorkerProcesses()
	suite.exec.AssertExpectations(suite.T())

	assert.Equal(suite.T(), len(suite.provider.workerPool), 1)
	worker := suite.provider.workerPool[model.SSMAgentWorkerName]
	assert.Equal(suite.T(), len(worker.Processes), 1)
	assert.Equal(suite.T(), worker.Processes[failurePid].Pid, failurePid)
	assert.Equal(suite.T(), worker.Processes[failurePid].Status, model.Active)
}
