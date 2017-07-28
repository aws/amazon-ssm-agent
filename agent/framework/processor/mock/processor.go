package processormock

import (
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/stretchr/testify/mock"
)

type MockedProcessor struct {
	mock.Mock
}

func (m *MockedProcessor) Start() (chan contracts.DocumentResult, error) {
	args := m.Called()
	return args.Get(0).(chan contracts.DocumentResult), args.Error(1)
}

func (m *MockedProcessor) Stop(stopType contracts.StopType) {
	m.Called(stopType)
	return
}

func (m *MockedProcessor) Submit(docState model.DocumentState) {
	m.Called(docState)
	return
}

func (m *MockedProcessor) Cancel(docState model.DocumentState) {
	m.Called(docState)
	return
}
