package channelmock

import (
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/contracts"
	"github.com/stretchr/testify/mock"
)

type MockedChannel struct {
	mock.Mock
}

func (m *MockedChannel) Connect() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockedChannel) Send(msg contracts.Message) error {
	args := m.Called(msg)
	return args.Error(0)
}

func (m *MockedChannel) GetMessageChannel() chan contracts.Message {
	args := m.Called()
	return args.Get(0).(chan contracts.Message)
}

func (m *MockedChannel) Close() {
	m.Called()
	return
}
