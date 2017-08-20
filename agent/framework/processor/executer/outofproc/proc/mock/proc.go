package procmock

import (
	"time"

	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/proc"
	"github.com/stretchr/testify/mock"
)

type MockedOSProcess struct {
	mock.Mock
}

func (m *MockedOSProcess) Pid() int {
	args := m.Called()
	return args.Get(0).(int)
}

func (m *MockedOSProcess) StartTime() time.Time {
	args := m.Called()
	return args.Get(0).(time.Time)
}

func (m *MockedOSProcess) Kill() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockedOSProcess) Wait() (proc.ProcessState, error) {
	args := m.Called()
	return args.Get(0).(proc.ProcessState), args.Error(1)
}

type MockedOSProcessState struct {
	mock.Mock
}

func (m *MockedOSProcessState) Success() bool {
	args := m.Called()
	return args.Get(0).(bool)
}
