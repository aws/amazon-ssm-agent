// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package installer_mock implements the mock for the installer package
package installer_mock

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/stretchr/testify/mock"
)

type Mock struct {
	mock.Mock
}

func (inst *Mock) Install(context context.T) contracts.PluginOutputer {
	args := inst.Called(context)
	return args.Get(0).(contracts.PluginOutputer)
}

func (inst *Mock) Uninstall(context context.T) contracts.PluginOutputer {
	args := inst.Called(context)
	return args.Get(0).(contracts.PluginOutputer)
}

func (inst *Mock) Validate(context context.T) contracts.PluginOutputer {
	args := inst.Called(context)
	return args.Get(0).(contracts.PluginOutputer)
}

func (inst *Mock) Version() string {
	args := inst.Called()
	return args.String(0)
}

func (inst *Mock) PackageName() string {
	args := inst.Called()
	return args.String(0)
}
