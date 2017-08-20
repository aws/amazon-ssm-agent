package messaging

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/channel/mock"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/mock"
)

var logger = log.NewMockLog()

func TestMessaging(t *testing.T) {
	testInputDatagram := "testinput"
	testOutputDatagram := "testoutput"
	recvChan := make(chan string)
	sendChan := make(chan string)
	stopChan := make(chan int)
	channelMock := new(channelmock.MockedChannel)
	channelMock.On("GetMessageChannel").Return(recvChan)
	channelMock.On("Close").Return(nil)
	channelMock.On("Send", testInputDatagram).Return(nil)
	backendMock := new(BackendMock)
	backendMock.On("Accept").Return(sendChan)
	backendMock.On("Process", testOutputDatagram).Return(nil)
	backendMock.On("Close").Return(nil)
	backendMock.On("Stop").Return(stopChan)
	go func() {
		//first, signal messaging worker to send to ipc
		sendChan <- testInputDatagram

		//then, receive a message from ipc
		recvChan <- testOutputDatagram
		//finally terminate the worker, messaging worker returns
		stopChan <- stopTypeTerminate
	}()
	stopTimer := make(chan bool)
	Messaging(logger, channelMock, backendMock, stopTimer)
	channelMock.AssertExpectations(t)
	backendMock.AssertExpectations(t)
}

type BackendMock struct {
	mock.Mock
}

func (m *BackendMock) Accept() <-chan string {
	args := m.Called()
	return args.Get(0).(chan string)
}

func (m *BackendMock) Process(datagram string) error {
	args := m.Called(datagram)
	return args.Error(0)
}

func (m *BackendMock) Close() {
	m.Called()
	return
}

func (m *BackendMock) Stop() <-chan int {
	args := m.Called()
	return args.Get(0).(chan int)
}
