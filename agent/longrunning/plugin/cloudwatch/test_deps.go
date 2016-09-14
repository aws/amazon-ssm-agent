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

// Package cloudwatch implements cloudwatch plugin and its configuration
package cloudwatch

import (
	"os"

	"github.com/stretchr/testify/mock"
)

// Mock stands for a mocked fileUtil.
type fileUtilMock struct {
	mock.Mock
}

// Exists returns true if the given file exists, false otherwise, ignoring any underlying error
func (f *fileUtilMock) Exists(filePath string) bool {
	args := f.Called(filePath)
	return args.Get(0).(bool)
}

// MakeDirs create the directories along the path if missing.
func (f *fileUtilMock) MakeDirs(destinationDir string) error {
	args := f.Called(destinationDir)
	return args.Error(0)
}

// WriteIntoFileWithPermissions writes into file with given file mode permissions
func (f *fileUtilMock) WriteIntoFileWithPermissions(absolutePath, content string, perm os.FileMode) (bool, error) {
	args := f.Called(absolutePath, content, perm)
	return args.Get(0).(bool), args.Error(1)
}
