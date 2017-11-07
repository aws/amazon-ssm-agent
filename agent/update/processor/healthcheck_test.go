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
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/ssm"
	"github.com/aws/amazon-ssm-agent/agent/version"
	ssmService "github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/assert"
)

type healthCheckTestCase struct {
	Input  UpdateState
	Output string
}

func TestHealthCheck(t *testing.T) {
	// generate test cases
	testCases := []healthCheckTestCase{
		{NotStarted, active},
		{Initialized, updateInitialized},
		{Staged, updateStaged},
		{Installed, updateInProgress},
		{Completed, updateSucceeded},
		{Rollback, rollingBack},
		{RolledBack, rollBackCompleted},
	}

	context := createUpdateContext(Installed)

	// run tests
	for _, tst := range testCases {
		context.Current.State = tst.Input

		// call method
		result := prepareHealthStatus(context.Current, "")

		// check results
		assert.Equal(t, result, tst.Output)
	}
}

func TestUpdateHealthCheck(t *testing.T) {
	context := createUpdateContext(Installed)
	service := &svcManager{}

	mockObj := ssm.NewMockDefault()
	mockObj.On(
		"UpdateInstanceInformation",
		logger,
		version.Version,
		updateInProgress).Return(&ssmService.UpdateInstanceInformationOutput{}, nil)

	// setup
	newSsmSvc = func() ssm.Service {
		return mockObj
	}

	// action
	err := service.UpdateHealthCheck(logger, context.Current, "")

	// assert
	mockObj.AssertExpectations(t)
	assert.NoError(t, err)
}

func TestUpdateHealthCheckFailCreatingService(t *testing.T) {
	// setup
	// fail to create a new ssm service
	ssmSvc = nil
	context := createUpdateContext(Installed)
	service := &svcManager{}
	// action
	err := service.UpdateHealthCheck(logger, context.Current, "")

	// assert
	assert.Error(t, err)
}

func createUpdateContext(state UpdateState) *UpdateContext {
	context := &UpdateContext{}
	context.Current = &UpdateDetail{}
	context.Current.Result = contracts.ResultStatusSuccess
	context.Current.SourceVersion = "5.0.0.0"
	context.Current.TargetVersion = "6.0.0.0"
	context.Current.State = state
	context.Current.UpdateRoot = "testdata"
	context.Current.MessageID = "message id"

	return context
}
