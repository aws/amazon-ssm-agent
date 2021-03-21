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

// +build integration

// Package processor contains the methods for update ssm agent.
// It also provides methods for sendReply and updateInstanceInfo
package processor

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	contextmocks "github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	"github.com/stretchr/testify/assert"
)

func TestGetV12DocOrchDir(t *testing.T) {
	context := contextmocks.NewMockDefault()
	shortInstanceId, _ := context.Identity().ShortInstanceID()

	updateDetail := &UpdateDetail{}
	dir := getV12DocOrchDir(context.Identity(), context.Log(), updateDetail)

	expected := fileutil.BuildPath(
		appconfig.DefaultDataStorePath,
		shortInstanceId,
		appconfig.DefaultDocumentRootDirName,
		"orchestration",
		updateconstants.DefaultOutputFolder)
	assert.Equal(t, expected, dir)

	updateDetail.MessageID = "messageid"
	dir = getV12DocOrchDir(context.Identity(), context.Log(), updateDetail)
	expected = fileutil.BuildPath(
		appconfig.DefaultDataStorePath,
		shortInstanceId,
		appconfig.DefaultDocumentRootDirName,
		"orchestration",
		"messageid",
		updateconstants.DefaultOutputFolder)

	assert.Equal(t, expected, dir)
}

func TestGetV22DocOrchDir(t *testing.T) {
	context := contextmocks.NewMockDefault()
	shortInstanceId, _ := context.Identity().ShortInstanceID()

	updateDetail := &UpdateDetail{}
	dir := getV22DocOrchDir(context.Identity(), context.Log(), updateDetail)

	expected := fileutil.BuildPath(
		appconfig.DefaultDataStorePath,
		shortInstanceId,
		appconfig.DefaultDocumentRootDirName,
		"orchestration",
		updateconstants.DefaultOutputFolder,
		updateconstants.DefaultOutputFolder)
	assert.Equal(t, expected, dir)

	updateDetail.MessageID = "messageid"
	dir = getV22DocOrchDir(context.Identity(), context.Log(), updateDetail)
	expected = fileutil.BuildPath(
		appconfig.DefaultDataStorePath,
		shortInstanceId,
		appconfig.DefaultDocumentRootDirName,
		"orchestration",
		"messageid",
		updateconstants.DefaultOutputFolder,
		updateconstants.DefaultOutputFolder)

	assert.Equal(t, expected, dir)
}
