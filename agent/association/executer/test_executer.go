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

// Package executer allows execute Pending association and InProgress association
package executer

import (
	"github.com/aws/amazon-ssm-agent/agent/association/taskpool"
	"github.com/aws/amazon-ssm-agent/agent/context"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/message/contracts"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/mock"
)

// DocumentExecuterMock stands for a mocked document executer.
type DocumentExecuterMock struct {
	mock.Mock
}

// ExecuteInProgressDocument mocks implementation for ExecuteInProgressDocument
func (m *DocumentExecuterMock) ExecutePendingDocument(context context.T, pool taskpool.T, interimDocState *messageContracts.DocumentState) error {
	args := m.Called(context, pool, interimDocState)
	return args.Error(0)
}

// ExecuteInProgressDocument mocks implementation for ExecuteInProgressDocument
func (m *DocumentExecuterMock) ExecuteInProgressDocument(context context.T, interimDocState *messageContracts.DocumentState, cancelFlag task.CancelFlag) {
}
