package outofproc

import (
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	executermocks "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/mock"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/channel"
	channelmock "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/channel/mock"
	procmock "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/proc/mock"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type TestCase struct {
	context        *context.Mock
	docStore       *executermocks.MockDocumentStore
	docState       model.DocumentState
	procController *procmock.MockedProcessController
	executer       *OutOfProcExecuter
	results        map[string]*contracts.PluginResult
	resultStatus   contracts.ResultStatus
}

var testInstanceID = "i-400e1090"
var testDocumentID = "13e8e6ad-e195-4ccb-86ee-328153b0dafe"
var testMessageID = "MessageID"
var testAssociationID = "AssociationID"
var testDocumentName = "AWS-RunPowerShellScript"
var testDocumentVersion = "testVersion"
var testStartDateTime = time.Date(2017, 8, 13, 0, 0, 0, 0, time.UTC)
var testEndDateTime = time.Date(2017, 8, 13, 0, 0, 1, 0, time.UTC)

var logger = log.NewMockLog()

func CreateTestCase() *TestCase {
	contextMock := context.NewMockDefaultWithContext([]string{"MASTER"})
	docStore := new(executermocks.MockDocumentStore)
	procController := new(procmock.MockedProcessController)
	docInfo := model.DocumentInfo{
		CreatedDate:     "2017-06-10T01-23-07.853Z",
		MessageID:       testMessageID,
		DocumentName:    testDocumentName,
		AssociationID:   testAssociationID,
		DocumentID:      testDocumentID,
		InstanceID:      testInstanceID,
		DocumentVersion: testDocumentVersion,
		RunCount:        0,
	}

	pluginState := model.PluginState{
		Name: "aws:runScript",
		Id:   "aws:runScript",
	}
	docState := model.DocumentState{
		DocumentInformation:        docInfo,
		DocumentType:               "SendCommand",
		InstancePluginsInformation: []model.PluginState{pluginState},
	}

	result := contracts.PluginResult{
		PluginName:    "aws:runScript",
		Status:        contracts.ResultStatusSuccess,
		StartDateTime: testStartDateTime,
		EndDateTime:   testEndDateTime,
	}
	results := make(map[string]*contracts.PluginResult)
	results[pluginState.Id] = &result

	exe := OutOfProcExecuter{
		ctx:            contextMock,
		procController: procController,
		documentID:     testDocumentID,
	}
	return &TestCase{
		context: contextMock,

		docStore:       docStore,
		docState:       docState,
		procController: procController,
		executer:       &exe,
		results:        results,
		resultStatus:   contracts.ResultStatusSuccess,
	}
}

func TestPrepareStartNewProcess(t *testing.T) {
	testCase := CreateTestCase()
	//testing message version
	channelDiscoverer = func(documentID string) (string, bool) {
		return "", false
	}
	channelMock := new(channelmock.MockedChannel)
	expectedChannelName := createChannelHandle(testDocumentID)
	channelMock.On("Open", expectedChannelName).Return(nil)
	channelCreator = func(mode channel.Mode) channel.Channel {
		assert.Equal(t, mode, channel.ModeMaster)
		return channelMock
	}
	exe := testCase.executer

	expectedArgList := []string{expectedChannelName}
	testCase.procController.On("StartProcess", defaultProcessName, expectedArgList).Return(0, nil)
	testCase.procController.On("Release").Return(nil)
	_, err := exe.prepare()
	assert.NoError(t, err)
	testCase.procController.AssertExpectations(t)
	channelMock.AssertExpectations(t)
}

//TODO downgrade is currently not supported
func TestPrepareConnectOldWorker(t *testing.T) {
	testCase := CreateTestCase()
	expectedChannelName := createChannelHandle(testDocumentID)
	channelDiscoverer = func(documentID string) (string, bool) {
		return expectedChannelName, true
	}
	channelMock := new(channelmock.MockedChannel)
	channelMock.On("Open", expectedChannelName).Return(nil)
	channelCreator = func(mode channel.Mode) channel.Channel {
		assert.Equal(t, mode, channel.ModeMaster)
		return channelMock
	}
	exe := testCase.executer
	_, err := exe.prepare()
	assert.NoError(t, err)
	testCase.procController.AssertNotCalled(t, "StartProcess", mock.Anything, mock.Anything)
	channelMock.AssertExpectations(t)
}

func TestMessaging(t *testing.T) {
	testInputDatagram := "testinput"
	testOutputDatagram := "testoutput"
	recvChan := make(chan string)
	sendChan := make(chan string)
	channelMock := new(channelmock.MockedChannel)
	channelMock.On("GetMessageChannel").Return(recvChan)
	channelMock.On("Close").Return(nil)
	channelMock.On("Send", testInputDatagram).Return(nil)
	backendMock := new(BackendMock)
	backendMock.On("Accept").Return(sendChan)
	backendMock.On("Process", testOutputDatagram).Return(nil)
	backendMock.On("Close").Return(nil)
	stopChan := make(chan int)
	go func() {
		//first, signal messaging worker to send to ipc
		sendChan <- testInputDatagram

		//then, receive a message from ipc
		recvChan <- testOutputDatagram
		//finally terminate the worker, messaging worker returns
		stopChan <- stopTypeTerminate
	}()
	Messaging(logger, channelMock, backendMock, stopChan)
	channelMock.AssertExpectations(t)
	backendMock.AssertExpectations(t)
}

//this is needed, since after marshal-unmarshalling thru the data channel, the pointer value changed
func assertValueEqual(t *testing.T, a map[string]*contracts.PluginResult, b map[string]*contracts.PluginResult) {
	assert.Equal(t, len(a), len(b))
	for key, val := range a {
		assert.Equal(t, *val, *b[key])
	}
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
