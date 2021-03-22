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
//
package linuxcontainerutil

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateinfo"
	"github.com/stretchr/testify/mock"
)

func init() {
	dep = &DepMock{}
}

type DepMock struct {
	mock.Mock
}

func (m *DepMock) UpdateUtilExeCommandOutput(
	customUpdateExecutionTimeoutInSeconds int,
	log log.T,
	cmd string,
	parameters []string,
	workingDir string,
	outputRoot string,
	stdOut string,
	stdErr string,
	usePlatformSpecificCommand bool) (output string, err error) {
	args := m.Called(log, cmd, parameters, workingDir, outputRoot, stdOut, stdErr, usePlatformSpecificCommand)
	return args.String(0), args.Error(1)
}

func (m *DepMock) GetInstanceInfo(context context.T) (instanceInfo updateinfo.T, err error) {
	args := m.Called(context)
	return args.Get(0).(updateinfo.T), args.Error(1)
}
