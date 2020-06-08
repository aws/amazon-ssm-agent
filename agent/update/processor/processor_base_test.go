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
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
)

var logger = log.NewMockLog()

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

type serviceStub struct {
	Service
}

type stubControl struct {
	failCreateInstanceContext      bool
	failCreateUpdateDownloadFolder bool
	serviceIsRunning               bool
	failExeCommand                 bool
}

type utilityStub struct {
	updateutil.Utility
	controller *stubControl
}

type contextMgrStub struct{}

func (c *contextMgrStub) saveUpdateContext(log log.T, context *UpdateContext, contextLocation string) (err error) {
	return nil
}

func (c *contextMgrStub) uploadOutput(log log.T, context *UpdateContext, orchestrationDir string) error {
	return nil
}

// createUpdaterWithStubs creates stubs updater and it's manager, util and service
func createDefaultUpdaterStub() *Updater {
	return createUpdaterStubs(&stubControl{})
}

func createUpdaterStubs(control *stubControl) *Updater {
	updater := NewUpdater()
	updater.mgr.svc = &serviceStub{}
	updater.mgr.util = &utilityStub{controller: control}
	updater.mgr.ctxMgr = &contextMgrStub{}

	return updater
}
