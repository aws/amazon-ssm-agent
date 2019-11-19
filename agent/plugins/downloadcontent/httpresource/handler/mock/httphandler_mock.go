/*
 * Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"). You may not
 * use this file except in compliance with the License. A copy of the
 * License is located at
 *
 * http://aws.amazon.com/apache2.0/
 *
 * or in the "license" file accompanying this file. This file is distributed
 * on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing
 * permissions and limitations under the License.
 */

package mock

import (
	"github.com/aws/amazon-ssm-agent/agent/fileutil/filemanager"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/mock"
)

type HTTPHandlerMock struct {
	mock.Mock
}

func (mock *HTTPHandlerMock) Download(log log.T, fileSystem filemanager.FileSystem, destPath string) (string, error) {
	args := mock.Called(log, fileSystem, destPath)
	return args.String(0), args.Error(1)
}

func (mock *HTTPHandlerMock) Validate() (bool, error) {
	args := mock.Called()
	return args.Bool(0), args.Error(1)
}
