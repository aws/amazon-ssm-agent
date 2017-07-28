package outofproc

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	executermocks "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/mock"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/channel"
	channelmock "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/channel/mock"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/contracts"
	procmock "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/proc/mock"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/task"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type TestCase struct {
	context        *context.Mock
	docStore       *executermocks.MockDocumentStore
	docState       model.DocumentState
	procController *procmock.MockedProcessController
	cancelFlag     task.CancelFlag
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
var logger = log.NewMockLog()

func CreateTestCase() *TestCase {
	contextMock := context.NewMockDefault()
	docStore := new(executermocks.MockDocumentStore)
	procController := new(procmock.MockedProcessController)
	cancelFlag := task.NewChanneledCancelFlag()
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
		PluginName: "aws:runScript",
		Status:     contracts.ResultStatusSuccess,
	}
	results := make(map[string]*contracts.PluginResult)
	results[pluginState.Id] = &result

	exe := OutOfProcExecuter{
		ctx:            contextMock,
		cancelFlag:     cancelFlag,
		docState:       &docState,
		docStore:       docStore,
		procController: procController,
	}
	return &TestCase{
		context:        contextMock,
		cancelFlag:     cancelFlag,
		docStore:       docStore,
		docState:       docState,
		procController: procController,
		executer:       &exe,
		results:        results,
		resultStatus:   contracts.ResultStatusSuccess,
	}
}

func TestPrepareStartNewProcessV1(t *testing.T) {
	testCase := CreateTestCase()
	//testing message version
	v1 := "1.0"
	channelDiscoverer = func(documentID string) (string, bool) {
		return "", false
	}
	channelMock := new(channelmock.MockedChannel)
	expectedChannelName := createChannelHandle(testDocumentID, v1)
	channelMock.On("Connect").Return(nil)
	channelCreator = func(handle string, mode channel.Mode) (channel.Channel, error) {
		assert.Equal(t, mode, channel.ModeServer)
		assert.Equal(t, handle, expectedChannelName)
		return channelMock, nil
	}
	exe := testCase.executer

	expectedProcName := "amazon-ssm-command-" + testDocumentID
	expectedArgList := []string{expectedChannelName}
	testCase.procController.On("StartProcess", expectedProcName, expectedArgList).Return(0, nil)
	_, version, _ := exe.prepare()
	assert.Equal(t, version, "1.0")
	testCase.procController.AssertExpectations(t)
	channelMock.AssertExpectations(t)
}

//TODO downgrade is currently not supported
func TestPrepareConnectOldClient(t *testing.T) {
	testCase := CreateTestCase()
	//testing message version
	vclient := "0.9"
	expectedChannelName := createChannelHandle(testDocumentID, vclient)
	channelDiscoverer = func(documentID string) (string, bool) {
		return expectedChannelName, true
	}
	channelMock := new(channelmock.MockedChannel)
	channelMock.On("Connect").Return(nil)
	channelCreator = func(handle string, mode channel.Mode) (channel.Channel, error) {
		assert.Equal(t, mode, channel.ModeServer)
		assert.Equal(t, handle, expectedChannelName)
		return channelMock, nil
	}
	exe := testCase.executer
	_, version, _ := exe.prepare()
	assert.Equal(t, version, vclient)
	testCase.procController.AssertNotCalled(t, "StartProcess", mock.Anything, mock.Anything)
	channelMock.AssertExpectations(t)
}

//server version 1.0 successfully run document
func TestServerSuccessV1(t *testing.T) {
	testCase := CreateTestCase()
	v1 := "1.0"
	//create new sub-process
	channelDiscoverer = func(documentID string) (string, bool) {
		return "", false
	}
	channelMock := new(channelmock.MockedChannel)
	expectedChannelName := createChannelHandle(testDocumentID, v1)
	channelMock.On("Connect").Return(nil)
	clientChan := make(chan messageContracts.Message)
	clientDone := make(chan bool)
	//client send results
	go func() {
		results := make(map[string]*contracts.PluginResult)
		for key, val := range testCase.results {
			results[key] = val
			res1 := contracts.DocumentResult{
				LastPlugin:    key,
				PluginResults: results,
				Status:        contracts.ResultStatusInProgress,
			}
			contents, err := jsonutil.Marshal(res1)
			assert.Empty(t, err)
			clientChan <- messageContracts.Message{
				Version: v1,
				Type:    messageContracts.MessageTypePayload,
				Content: contents,
			}
		}
		//send document complete result
		res := contracts.DocumentResult{
			LastPlugin:    "",
			PluginResults: results,
			Status:        testCase.resultStatus,
		}
		contents, err := jsonutil.Marshal(res)
		assert.Empty(t, err)
		clientChan <- messageContracts.Message{
			Version: v1,
			Type:    messageContracts.MessageTypePayload,
			Content: contents,
		}
		//send close message
		clientChan <- messageContracts.Message{
			Version: v1,
			Type:    messageContracts.MessageTypeClose,
		}
		clientDone <- true
	}()
	channelMock.On("GetMessageChannel").Return(clientChan)
	channelMock.On("Send", mock.Anything).Return(nil)
	channelMock.On("Close").Return(nil).Once()
	channelCreator = func(handle string, mode channel.Mode) (channel.Channel, error) {
		assert.Equal(t, mode, channel.ModeServer)
		assert.Equal(t, handle, expectedChannelName)
		return channelMock, nil
	}
	resultState := testCase.docState
	resultState.DocumentInformation.DocumentStatus = testCase.resultStatus
	testCase.docStore.On("Save", resultState).Return(nil)
	expectedProcName := "amazon-ssm-command-" + testDocumentID
	expectedArgList := []string{expectedChannelName}
	testCase.procController.On("StartProcess", expectedProcName, expectedArgList).Return(0, nil)
	exe := testCase.executer
	//make a big buffered channel to assert later
	nPlugins := len(testCase.results)
	resChan := make(chan contracts.DocumentResult, nPlugins+1)
	serverDone := make(chan bool)
	go func() {
		num := 0
		var last contracts.DocumentResult
		for res := range resChan {
			num++
			last = res
		}
		assert.Equal(t, num, nPlugins+1)
		assert.Equal(t, "", last.LastPlugin)
		assert.Equal(t, testCase.resultStatus, last.Status)
		assert.Equal(t, testDocumentName, last.DocumentName)
		assert.Equal(t, testAssociationID, last.AssociationID)
		assert.Equal(t, testMessageID, last.MessageID)
		assert.Equal(t, len(testCase.docState.InstancePluginsInformation), last.NPlugins)
		assert.Equal(t, testDocumentVersion, last.DocumentVersion)
		serverDone <- true
	}()

	exe.server(exe.docState.InstancePluginsInformation, resChan)
	<-clientDone
	<-serverDone
	channelMock.AssertExpectations(t)
	channelMock.AssertNumberOfCalls(t, "Send", 1)
	testCase.procController.AssertExpectations(t)
	testCase.docStore.AssertExpectations(t)
}

func TestServerCancelV1(t *testing.T) {
	testCase := CreateTestCase()
	//test result is canceled
	testCase.resultStatus = contracts.ResultStatusCancelled
	v1 := "1.0"
	//create new sub-process
	channelDiscoverer = func(documentID string) (string, bool) {
		return "", false
	}
	channelMock := new(channelmock.MockedChannel)
	expectedChannelName := createChannelHandle(testDocumentID, v1)
	clientChan := make(chan messageContracts.Message)
	clientDone := make(chan bool)

	channelMock.On("Connect").Return(nil)
	channelMock.On("GetMessageChannel").Return(clientChan)
	channelMock.On("Send", mock.Anything).Return(nil)
	channelMock.On("Close").Return(nil).Once()

	channelCreator = func(handle string, mode channel.Mode) (channel.Channel, error) {
		assert.Equal(t, mode, channel.ModeServer)
		assert.Equal(t, handle, expectedChannelName)
		return channelMock, nil
	}
	resultState := testCase.docState
	resultState.DocumentInformation.DocumentStatus = testCase.resultStatus
	testCase.docStore.On("Save", resultState).Return(nil)
	expectedProcName := "amazon-ssm-command-" + testDocumentID
	expectedArgList := []string{expectedChannelName}
	testCase.procController.On("StartProcess", expectedProcName, expectedArgList).Return(0, nil)
	exe := testCase.executer
	cancelChan := make(chan bool)
	//client send results
	go func() {
		<-cancelChan
		//send document cancelled update
		res := contracts.DocumentResult{
			LastPlugin: "",
			Status:     testCase.resultStatus,
		}
		contents, err := jsonutil.Marshal(res)
		assert.Empty(t, err)
		logger.Info("client sending canceled document result")
		clientChan <- messageContracts.Message{
			Version: v1,
			Type:    messageContracts.MessageTypePayload,
			Content: contents,
		}
		logger.Info("client closing connection...")
		//send close message
		clientChan <- messageContracts.Message{
			Version: v1,
			Type:    messageContracts.MessageTypeClose,
		}
		clientDone <- true
	}()
	resChan := make(chan contracts.DocumentResult)
	go exe.server(exe.docState.InstancePluginsInformation, resChan)
	exe.cancelFlag.Set(task.Canceled)
	logger.Info("canceling client routine...")
	cancelChan <- true
	logger.Info("client has been canceled")
	resultReceived := false
	for res := range resChan {
		if res.LastPlugin == "" {
			assert.Equal(t, res.Status, testCase.resultStatus)
			resultReceived = true
		}
	}
	//server has returned, make sure client is closed as well
	<-clientDone
	assert.True(t, resultReceived)
	channelMock.AssertExpectations(t)
	//server send exact 2 messages in the cancel situation at V1 implementation
	channelMock.AssertNumberOfCalls(t, "Send", 2)
	testCase.procController.AssertExpectations(t)
	testCase.docStore.AssertExpectations(t)

}

func TestServerShutdownV1(t *testing.T) {
	testCase := CreateTestCase()
	//test result is InProgress since the job is not done yet
	//TODO this might be needed since later processor will poke the status of the document
	//testCase.resultStatus = contracts.ResultStatusInProgress
	v1 := "1.0"
	//create new sub-process
	channelDiscoverer = func(documentID string) (string, bool) {
		return "", false
	}
	channelMock := new(channelmock.MockedChannel)
	channelMock.On("Connect").Return(nil)
	clientChan := make(chan messageContracts.Message)
	channelMock.On("GetMessageChannel").Return(clientChan)
	//server sent 2 messages, start + cancel
	channelMock.On("Send", mock.Anything).Return(nil)
	expectedChannelName := createChannelHandle(testDocumentID, v1)
	//server sent 2 messages, start + cancel
	channelCreator = func(handle string, mode channel.Mode) (channel.Channel, error) {
		assert.Equal(t, mode, channel.ModeServer)
		assert.Equal(t, handle, expectedChannelName)
		return channelMock, nil
	}
	//document status is actually unchanged
	resultState := testCase.docState
	testCase.docStore.On("Save", resultState).Return(nil)
	expectedProcName := "amazon-ssm-command-" + testDocumentID
	expectedArgList := []string{expectedChannelName}
	testCase.procController.On("StartProcess", expectedProcName, expectedArgList).Return(0, nil)
	//make sure the started process is released
	testCase.procController.On("Release").Return(nil)
	exe := testCase.executer
	resChan := make(chan contracts.DocumentResult)
	go exe.server(exe.docState.InstancePluginsInformation, resChan)
	exe.cancelFlag.Set(task.ShutDown)
	//result channel should exit immediately
	_, more := <-resChan
	assert.False(t, more)
	//at whatever timeline server receives a "Shutdown", it should not destroy the channel objects it created
	channelMock.AssertNotCalled(t, "Close")
	//assert Save() is called during clean up
	testCase.procController.AssertExpectations(t)
	testCase.docStore.AssertExpectations(t)
}
