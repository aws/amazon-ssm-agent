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
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"github.com/stretchr/testify/assert"
)

func TestUpdateStateChange(t *testing.T) {
	updater := createDefaultUpdaterStub()
	context := generateTestCase().Context
	err := updater.mgr.inProgress(context, logger, Initialized)

	assert.Equal(t, context.Current.State, Initialized)
	assert.Equal(t, context.Current.Result, contracts.ResultStatusInProgress)

	assert.Equal(t, err, nil)
}

func TestUpdateSucceed(t *testing.T) {
	updater := createDefaultUpdaterStub()
	context := generateTestCase().Context
	context.Current.OutputS3BucketName = "test"
	err := updater.mgr.succeeded(context, logger)

	emptyUpdate := &UpdateDetail{}

	assert.Equal(t, context.Current.State, emptyUpdate.State)
	assert.Equal(t, context.Current.Result, emptyUpdate.Result)

	assert.Equal(t, err, nil)
	assert.Equal(t, context.Histories[0].State, Completed)
	assert.Equal(t, context.Histories[0].Result, contracts.ResultStatusSuccess)
}

func TestUpdateFailed(t *testing.T) {
	updater := createDefaultUpdaterStub()
	context := generateTestCase().Context
	context.Current.OutputS3BucketName = "test"
	err := updater.mgr.failed(context, logger, updateutil.ErrorInstallFailed, "Cannot Install", true)

	emptyUpdate := &UpdateDetail{}

	assert.Equal(t, context.Current.State, emptyUpdate.State)
	assert.Equal(t, context.Current.Result, emptyUpdate.Result)

	assert.Equal(t, err, nil)
	assert.Equal(t, context.Histories[0].State, Completed)
	assert.Equal(t, context.Histories[0].Result, contracts.ResultStatusFailed)
}

type ContextTestCase struct {
	Context      *UpdateContext
	InfoMessage  string
	ErrorMessage string
	HasMessageID bool
}

func generateTestCase() ContextTestCase {
	testCase := ContextTestCase{
		Context:      &UpdateContext{},
		InfoMessage:  "Test Message",
		ErrorMessage: "Error Message",
		HasMessageID: true,
	}

	testCase.Context.Current = &UpdateDetail{
		MessageID: "MessageId",
	}
	testCase.Context.Histories = []*UpdateDetail{}
	return testCase
}
