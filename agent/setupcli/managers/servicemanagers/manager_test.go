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

package servicemanagers_test

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/servicemanagers"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/servicemanagers/mocks"
	"github.com/stretchr/testify/assert"
)

var logger = log.NewMockLog()

func TestStopAgent_ErrorStopAgent(t *testing.T) {
	managerMock := &mocks.IServiceManager{}

	managerMock.On("GetName").Return("MockName").Once()
	managerMock.On("StopAgent").Return(fmt.Errorf("FailedStopAgent")).Times(4)
	err := servicemanagers.StopAgent(managerMock, logger)
	assert.Error(t, err)
	assert.Contains(t, "retries exhausted", err.Error())
}

func TestStopAgent_ErrorVerifyStopAgent(t *testing.T) {
	managerMock := &mocks.IServiceManager{}

	managerMock.On("GetName").Return("MockName").Once()
	managerMock.On("StopAgent").Return(nil).Times(4)
	managerMock.On("GetAgentStatus").Return(common.UndefinedStatus, fmt.Errorf("FailedGetStatus")).Times(4)
	err := servicemanagers.StopAgent(managerMock, logger)
	assert.Error(t, err)
	assert.Contains(t, "retries exhausted", err.Error())
}

func TestStopAgent_ErrorStopAgentOnce_ErrorVerifyStopAgentOnce_Success(t *testing.T) {
	managerMock := &mocks.IServiceManager{}

	managerMock.On("GetName").Return("MockName").Once()
	managerMock.On("StopAgent").Return(fmt.Errorf("FailedStop")).Once()
	managerMock.On("StopAgent").Return(nil).Twice()

	managerMock.On("GetAgentStatus").Return(common.UndefinedStatus, fmt.Errorf("FailedGetStatus")).Once()
	managerMock.On("GetAgentStatus").Return(common.Stopped, nil).Once()

	err := servicemanagers.StopAgent(managerMock, logger)
	assert.NoError(t, err)
}

func TestStartAgent_ErrorStartAgent(t *testing.T) {
	managerMock := &mocks.IServiceManager{}

	managerMock.On("GetName").Return("MockName").Once()
	managerMock.On("StartAgent").Return(fmt.Errorf("FailedStartAgent")).Times(4)
	err := servicemanagers.StartAgent(managerMock, logger)
	assert.Error(t, err)
	assert.Contains(t, "retries exhausted", err.Error())
}

func TestStartAgent_ErrorVerifyStartAgent(t *testing.T) {
	managerMock := &mocks.IServiceManager{}

	managerMock.On("GetName").Return("MockName").Once()
	managerMock.On("StartAgent").Return(nil).Times(4)
	managerMock.On("GetAgentStatus").Return(common.UndefinedStatus, fmt.Errorf("FailedGetStatus")).Times(4)
	err := servicemanagers.StartAgent(managerMock, logger)
	assert.Error(t, err)
	assert.Contains(t, "retries exhausted", err.Error())
}

func TestStartAgent_ErrorStartAgentOnce_ErrorVerifyStartAgentOnce_Success(t *testing.T) {
	managerMock := &mocks.IServiceManager{}

	managerMock.On("GetName").Return("MockName").Once()
	managerMock.On("StartAgent").Return(fmt.Errorf("FailedStart")).Once()
	managerMock.On("StartAgent").Return(nil).Twice()

	managerMock.On("GetAgentStatus").Return(common.UndefinedStatus, fmt.Errorf("FailedGetStatus")).Once()
	managerMock.On("GetAgentStatus").Return(common.Running, nil).Once()

	err := servicemanagers.StartAgent(managerMock, logger)
	assert.NoError(t, err)
}
