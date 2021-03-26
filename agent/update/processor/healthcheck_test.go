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

// Package processor contains the methods for update ssm agent.
// It also provides methods for sendReply and updateInstanceInfo
package processor

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/ssm"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	ssmService "github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/assert"
)

var logger = log.NewMockLog()

type healthCheckTestCase struct {
	InputState   UpdateState
	IsSelfUpdate bool
	InputResult  contracts.ResultStatus
	Output       string
}

func TestHealthCheck(t *testing.T) {
	// generate test cases
	testCases := []healthCheckTestCase{
		{NotStarted, false, contracts.ResultStatusNotStarted, active},
		{Initialized, false, contracts.ResultStatusInProgress, updateInitialized},
		{Staged, false, contracts.ResultStatusInProgress, updateStaged},
		{Installed, false, contracts.ResultStatusInProgress, updateInProgress},
		{Completed, false, contracts.ResultStatusSuccess, updateSucceeded},
		{Completed, true, contracts.ResultStatusSuccess, updateSucceeded + "_SelfUpdate"},
		{Completed, false, contracts.ResultStatusFailed, updateFailed},
		{Rollback, false, contracts.ResultStatusNotStarted, rollingBack},
		{RolledBack, false, contracts.ResultStatusNotStarted, rollBackCompleted},
		{TestExecution, false, contracts.ResultStatusTestFailure, testFailed},
	}

	updateDetail := createUpdateDetail(Installed)

	// run tests
	for _, tst := range testCases {
		updateDetail.State = tst.InputState
		updateDetail.Result = tst.InputResult
		updateDetail.SelfUpdate = tst.IsSelfUpdate

		// call method
		result := PrepareHealthStatus(updateDetail, "", "")

		// check results
		assert.Equal(t, result, tst.Output, "Output was %s but expected to be %s", result, tst.Output)
	}
}

func TestUpdateHealthStatusWithNonAlarmingErrorCodes(t *testing.T) {
	// generate test cases
	testCases := map[updateconstants.ErrorCode]string{
		updateconstants.ErrorUnsupportedServiceManager: fmt.Sprintf("%v_%v-%v", updateFailed, updateconstants.ErrorUnsupportedServiceManager, noAlarm),
		updateconstants.ErrorEnvironmentIssue:          fmt.Sprintf("%v_%v", updateFailed, updateconstants.ErrorEnvironmentIssue),
	}

	dummyTargetVersion := "dummyTargetVersion"
	testCasesWithTargetVersion := map[updateconstants.ErrorCode]string{
		updateconstants.ErrorUnsupportedServiceManager: fmt.Sprintf("%v_%v-%v-%v", updateFailed, updateconstants.ErrorUnsupportedServiceManager, dummyTargetVersion, noAlarm),
		updateconstants.ErrorEnvironmentIssue:          fmt.Sprintf("%v_%v-%v", updateFailed, updateconstants.ErrorEnvironmentIssue, dummyTargetVersion),
	}

	updateDetail := createUpdateDetail(Installed)

	// run tests
	for errorCode, output := range testCases {
		updateDetail.Result = contracts.ResultStatusFailed
		updateDetail.State = Completed
		result := PrepareHealthStatus(updateDetail, string(errorCode), "")
		assert.Equal(t, output, result)
	}

	// run tests
	for errorCode, output := range testCasesWithTargetVersion {
		updateDetail.Result = contracts.ResultStatusFailed
		updateDetail.State = Completed
		result := PrepareHealthStatus(updateDetail, string(errorCode), dummyTargetVersion)
		assert.Equal(t, output, result)
	}
}

func TestHealthCheckWithUpdateFailed(t *testing.T) {
	// generate test cases
	testCases := []healthCheckTestCase{
		{NotStarted, false, contracts.ResultStatusFailed, fmt.Sprintf("%v_%v", updateFailed, NotStarted)},
		{Initialized, false, contracts.ResultStatusFailed, fmt.Sprintf("%v_%v", updateFailed, Initialized)},
		{Staged, false, contracts.ResultStatusFailed, fmt.Sprintf("%v_%v", updateFailed, Staged)},
		{Installed, false, contracts.ResultStatusFailed, fmt.Sprintf("%v_%v", updateFailed, Installed)},
		{Completed, false, contracts.ResultStatusFailed, fmt.Sprintf("%v_%v", updateFailed, Completed)},
		{Rollback, false, contracts.ResultStatusFailed, fmt.Sprintf("%v_%v", updateFailed, Rollback)},
		{RolledBack, false, contracts.ResultStatusFailed, fmt.Sprintf("%v_%v", updateFailed, RolledBack)},
		{RolledBack, true, contracts.ResultStatusFailed, fmt.Sprintf("%v_%v_%v", updateFailed, "SelfUpdate", RolledBack)},
	}

	updateDetail := createUpdateDetail(Installed)

	for _, tst := range testCases {
		updateDetail.Result = tst.InputResult
		updateDetail.State = Completed
		updateDetail.SelfUpdate = tst.IsSelfUpdate

		// call method
		result := PrepareHealthStatus(updateDetail, string(tst.InputState), "")

		// check results
		assert.Equal(t, result, tst.Output)
	}
}

func TestUpdateHealthCheck(t *testing.T) {
	updateDetail := createUpdateDetail(Installed)
	service := &svcManager{}

	mockObj := ssm.NewMockDefault()
	mockObj.On(
		"UpdateInstanceInformation",
		logger,
		updateDetail.SourceVersion,
		fmt.Sprintf("%v-%v", updateInProgress, updateDetail.TargetVersion)).Return(&ssmService.UpdateInstanceInformationOutput{}, nil)

	// setup
	newSsmSvc = func(context context.T) ssm.Service {
		return mockObj
	}

	// action
	err := service.UpdateHealthCheck(logger, updateDetail, "")

	// assert
	mockObj.AssertExpectations(t)
	assert.NoError(t, err)
}

func TestUpdateHealthCheckFailCreatingService(t *testing.T) {
	// setup
	// fail to create a new ssm service
	ssmSvc = nil
	updateDetail := createUpdateDetail(Installed)
	service := &svcManager{}
	// action
	err := service.UpdateHealthCheck(logger, updateDetail, "")

	// assert
	assert.Error(t, err)
}

func createUpdateDetail(state UpdateState) *UpdateDetail {
	updateDetail := &UpdateDetail{}
	updateDetail.Result = contracts.ResultStatusInProgress
	updateDetail.SourceVersion = "5.0.0.0"
	updateDetail.TargetVersion = "6.0.0.0"
	updateDetail.State = state
	updateDetail.UpdateRoot = "testdata"
	updateDetail.MessageID = "message id"

	return updateDetail
}
