package procmock

import (
	"time"

	"github.com/stretchr/testify/mock"
)

type MockedProcessController struct {
	mock.Mock
}

func (m *MockedProcessController) StartProcess(name string, argv []string) (int, error) {
	args := m.Called(name, argv)
	return args.Get(0).(int), args.Error(1)
}

func (m *MockedProcessController) Release() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockedProcessController) Kill() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockedProcessController) Find(pid int, createTime time.Time) bool {
	args := m.Called(pid, createTime)
	return args.Get(0).(bool)
}
