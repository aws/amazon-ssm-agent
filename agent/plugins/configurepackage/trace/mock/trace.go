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

package trace_mock

import (
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/packageservice"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/trace"
	"github.com/stretchr/testify/mock"
)

type Mock struct {
	mock.Mock
}

func (m *Mock) BeginSection(message string) *trace.Trace {
	args := m.Called(message)
	return args.Get(0).(*trace.Trace)
}

func (m *Mock) EndSection(trace *trace.Trace) error {
	args := m.Called(trace)
	return args.Error(0)
}

func (m *Mock) AddTrace(trace *trace.Trace) {
	m.Called(trace)
}

func (m *Mock) Traces() []*trace.Trace {
	args := m.Called()
	return args.Get(0).([]*trace.Trace)
}

func (m *Mock) CurrentTrace() *trace.Trace {
	args := m.Called()
	return args.Get(0).(*trace.Trace)
}

func (m *Mock) ToPackageServiceTrace() []*packageservice.Trace {
	args := m.Called()
	return args.Get(0).([]*packageservice.Trace)
}

func (m *Mock) ToPluginOutput() iohandler.IOHandler {
	args := m.Called()
	return args.Get(0).(iohandler.IOHandler)
}
