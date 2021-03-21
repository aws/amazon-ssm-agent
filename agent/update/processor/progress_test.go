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
// +build e2e

package processor

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	"github.com/stretchr/testify/assert"
)

type DetailTestCase struct {
	Detail       *UpdateDetail
	InfoMessage  string
	ErrorMessage string
	HasMessageID bool
}

func TestUpdateStateChange(t *testing.T) {
	updater := createDefaultUpdaterStub()
	detail := generateTestCase().Detail
	err := updater.mgr.inProgress(detail, logger, Initialized)

	assert.Equal(t, Initialized, detail.State)
	assert.Equal(t, contracts.ResultStatusInProgress, detail.Result)

	assert.Equal(t, err, nil)
}

func TestUpdateSucceed(t *testing.T) {
	updater := createDefaultUpdaterStub()
	detail := generateTestCase().Detail
	detail.OutputS3BucketName = "test"
	err := updater.mgr.succeeded(detail, logger)

	assert.Equal(t, Completed, detail.State)
	assert.Equal(t, contracts.ResultStatusSuccess, detail.Result)

	assert.Equal(t, err, nil)
}

func TestUpdateFailed(t *testing.T) {
	updater := createDefaultUpdaterStub()
	detail := generateTestCase().Detail
	detail.OutputS3BucketName = "test"
	err := updater.mgr.failed(detail, logger, updateconstants.ErrorInstallFailed, "Cannot Install", true)

	assert.Equal(t, Completed, detail.State)
	assert.Equal(t, contracts.ResultStatusFailed, detail.Result)

	assert.Equal(t, err, nil)
}

func TestUpdateInactive(t *testing.T) {
	updater := createDefaultUpdaterStub()
	detail := generateTestCase().Detail
	detail.OutputS3BucketName = "test"
	err := updater.mgr.inactive(detail, logger, "")

	assert.Equal(t, Completed, detail.State)
	assert.Equal(t, contracts.ResultStatusSuccess, detail.Result)

	assert.Equal(t, err, nil)
}

func generateTestCase() DetailTestCase {
	return DetailTestCase{
		Detail: &UpdateDetail{
			MessageID: "MessageId",
		},
		InfoMessage:  "Test Message",
		ErrorMessage: "Error Message",
		HasMessageID: true,
	}
}
