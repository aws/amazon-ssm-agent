package processormock

import (
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor"
	"github.com/stretchr/testify/mock"
)

type MockedProcessor struct {
	mock.Mock
}

func (m *MockedProcessor) Start() (chan contracts.DocumentResult, error) {
	args := m.Called()
	return args.Get(0).(chan contracts.DocumentResult), args.Error(1)
}

func (m *MockedProcessor) InitialProcessing(skipDocumentIfExpired bool) (err error) {
	args := m.Called(skipDocumentIfExpired)
	return args.Error(0)
}

func (m *MockedProcessor) Stop() {
	m.Called()
	return
}

func (m *MockedProcessor) Submit(docState contracts.DocumentState) processor.ErrorCode {
	args := m.Called(docState)
	return args.Get(0).(processor.ErrorCode)
}

func (m *MockedProcessor) Cancel(docState contracts.DocumentState) processor.ErrorCode {
	args := m.Called(docState)
	return args.Get(0).(processor.ErrorCode)
}
