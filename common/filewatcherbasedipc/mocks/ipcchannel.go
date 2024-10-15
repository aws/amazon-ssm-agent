package ipcchannelmock

import "github.com/stretchr/testify/mock"

type MockedChannel struct {
	mock.Mock
}

func (m *MockedChannel) Send(msg string) error {
	args := m.Called(msg)
	return args.Error(0)
}

func (m *MockedChannel) GetMessage() <-chan string {
	args := m.Called()
	return args.Get(0).(chan string)
}

func (m *MockedChannel) Close() {
	m.Called()
	return
}

func (m *MockedChannel) Destroy() {
	m.Called()
	return
}

func (m *MockedChannel) CleanupOwnModeFiles() {
	m.Called()
	return
}

func (m *MockedChannel) GetPath() string {
	args := m.Called()
	return args.Get(0).(string)
}
