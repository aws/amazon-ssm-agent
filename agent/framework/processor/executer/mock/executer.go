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

// Package executer provides interfaces as document execution logic
package executermocks

import (
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/mock"
)

type MockedExecuter struct {
	mock.Mock
}

func NewMockExecuter() *MockedExecuter {
	return new(MockedExecuter)
}

func (executerMock *MockedExecuter) Run(
	cancelFlag task.CancelFlag,
	docStore executer.DocumentStore) chan contracts.DocumentResult {
	args := executerMock.Called(cancelFlag, docStore)
	return args.Get(0).(chan contracts.DocumentResult)
}

type MockDocumentStore struct {
	mock.Mock
}

func (m *MockDocumentStore) Save(docState contracts.DocumentState) {
	m.Called(docState)
	return
}

func (m *MockDocumentStore) Load() contracts.DocumentState {
	args := m.Called()
	return args.Get(0).(contracts.DocumentState)
}
