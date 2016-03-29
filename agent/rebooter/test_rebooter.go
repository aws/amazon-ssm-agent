// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package rebooter

import (
	"github.com/stretchr/testify/mock"
)

// Note: This code is used in the test files. However, this code is not in a _test.go file
// because then we would have to copy it in every test package that needs the mock.

// MockedRebooter stands for a mock Rebooter.
type MockedRebooter struct {
	mock.Mock
}

// RebootMachine mocks the RebootMachine method.
func (rebooterMock *MockedRebooter) RebootMachine() {
	_ = rebooterMock.Called()
	return
}
