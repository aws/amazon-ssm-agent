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

// Package multiwritermock creates the mock mulitwriter
package multiwritermock

import (
	"io"

	"sync"

	"github.com/stretchr/testify/mock"
)

// MockDocumentIOMultiWriter mocks a document multi-writer.
type MockDocumentIOMultiWriter struct {
	mock.Mock
}

// AddWriter is a mocked method that just returns what mock tells it to.
func (m *MockDocumentIOMultiWriter) AddWriter(writer *io.PipeWriter) {
	m.Called(writer)
}

// GetStreamClosedChannel is a mocked method that just returns what mock tells it to.
func (m *MockDocumentIOMultiWriter) GetWaitGroup() (wg *sync.WaitGroup) {
	args := m.Called()
	return args.Get(0).(*sync.WaitGroup)
}

// Write is a mocked method that just returns what mock tells it to.
func (m *MockDocumentIOMultiWriter) Write(p []byte) (n int, err error) {
	args := m.Called(p)
	return args.Int(0), args.Error(1)
}

// WriteString is a mocked method that just returns what mock tells it to.
func (m *MockDocumentIOMultiWriter) WriteString(message string) (n int, err error) {
	args := m.Called(message)
	return args.Int(0), args.Error(1)
}

// Close is a mocked method that just returns what mock tells it to.
func (m *MockDocumentIOMultiWriter) Close() (err error) {
	args := m.Called()
	return args.Error(0)
}
