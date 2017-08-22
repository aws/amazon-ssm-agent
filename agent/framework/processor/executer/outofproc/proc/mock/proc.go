package procmock

import (
	"time"

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

func (m *MockedOSProcess) Wait() error {
	args := m.Called()
	return args.Error(0)
}
