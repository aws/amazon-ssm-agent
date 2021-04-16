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

package times

import (
	"time"

	"github.com/stretchr/testify/mock"
)

// Note: This code is used in the test files. However, this code is not in a _test.go file
// because then we would have to copy it in every test package that needs the mock.

// MockedClock implements the Now method to return a predictable time and the
// After method to return a channel that ca be woken up on demand.
type MockedClock struct {
	mock.Mock
	AfterChannel chan time.Time
}

// NewMockedClock creates a new instance of a mocked clock
func NewMockedClock() *MockedClock {
	return &MockedClock{AfterChannel: make(chan time.Time, 1)}
}

// Now returns a predefined value.
func (c *MockedClock) Now() time.Time {
	return c.Called().Get(0).(time.Time)
}

// After returns a channel that receives when we tell it to.
func (c *MockedClock) After(d time.Duration) <-chan time.Time {
	args := c.Called(d)
	return args.Get(0).(chan time.Time)
}
