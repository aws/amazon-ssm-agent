package messaging

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	channelmock "github.com/aws/amazon-ssm-agent/common/filewatcherbasedipc/mocks"
	"github.com/stretchr/testify/mock"
)

var logger = log.NewMockLog()

func TestMessagingTerminate(t *testing.T) {
	testInputDatagram := "testinput"
	testOutputDatagram := "testoutput"
	recvChan := make(chan string)
	sendChan := make(chan string)
	stopChan := make(chan int)
	channelMock := new(channelmock.MockedChannel)
	channelMock.On("GetMessage").Return(recvChan)
	channelMock.On("Destroy").Return(nil)
	channelMock.On("GetPath").Return("/test/path").Times(3)
	channelMock.On("Send", testInputDatagram).Return(nil)
	backendMock := new(BackendMock)
	backendMock.On("Accept").Return(sendChan)
	backendMock.On("Process", testOutputDatagram).Return(nil)
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

//soft stop, messaging will return only when the connection to data backend is closed and also the ipc is closed
func TestMessagingShutdown(t *testing.T) {
	testInputDatagram := "testinput"
	testOutputDatagram := "testoutput"
	recvChan := make(chan string)
	sendChan := make(chan string)
	stopChan := make(chan int)
	channelMock := new(channelmock.MockedChannel)
	channelMock.On("GetMessage").Return(recvChan)
	channelMock.On("Close").Run(func(mock.Arguments) {
		close(recvChan)
	}).Return(nil)
	channelMock.On("Send", testInputDatagram).Return(nil)
	backendMock := new(BackendMock)
	backendMock.On("Accept").Return(sendChan)
	channelMock.On("GetPath").Return("/test/path").Times(3)
	backendMock.On("Process", testOutputDatagram).Return(nil)
	backendMock.On("Stop").Return(stopChan)
	go func() {
		//first, signal messaging worker to send to ipc
		sendChan <- testInputDatagram

		//then, receive a message from ipc
		recvChan <- testOutputDatagram
		//finally terminate the worker, messaging worker returns
		stopChan <- stopTypeShutdown
		close(sendChan)
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

func (m *BackendMock) Stop() <-chan int {
	args := m.Called()
	return args.Get(0).(chan int)
}

func (m *BackendMock) Close() {
	m.Called()
}

func (m *BackendMock) CloseStop() {
	m.Called()
}
