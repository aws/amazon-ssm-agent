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

// Package iomodulemock implements the mock iomodule
package iomodulemock

import (
	"io"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/stretchr/testify/mock"
)

// MockIOModule mocks an IOModule.
type MockIOModule struct {
	mock.Mock
}

// Read is a mocked method that acknowledges that the function has been called.
func (m *MockIOModule) Read(context context.T, reader *io.PipeReader, exitCode int) {
	m.Called(context, reader, exitCode)
}
