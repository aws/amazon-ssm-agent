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

	"errors"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/proc"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
)

type TestCase struct {
	context      *context.Mock
	docStore     *executermocks.MockDocumentStore
	docState     model.DocumentState
	processMock  *procmock.MockedOSProcess
	results      map[string]*contracts.PluginResult
	resultStatus contracts.ResultStatus
}

var testInstanceID = "i-400e1090"
var testDocumentID = "13e8e6ad-e195-4ccb-86ee-328153b0dafe"
var testMessageID = "MessageID"
var testAssociationID = "AssociationID"
var testDocumentName = "AWS-RunPowerShellScript"
var testDocumentVersion = "testVersion"
var testStartDateTime = time.Date(2017, 8, 13, 0, 0, 0, 0, time.UTC)
var testEndDateTime = time.Date(2017, 8, 13, 0, 0, 1, 0, time.UTC)
var testPid = 100

var logger = log.NewMockLog()

func CreateTestCase() *TestCase {
	contextMock := context.NewMockDefaultWithContext([]string{"MASTER"})
	docStore := new(executermocks.MockDocumentStore)
	processMock := new(procmock.MockedOSProcess)
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
		Id:   "plugin1",
	}
	pluginState2 := model.PluginState{
		Name: "aws:runPowershellScript",
		Id:   "plugin2",
	}
	docState := model.DocumentState{
		DocumentInformation:        docInfo,
		DocumentType:               "SendCommand",
		InstancePluginsInformation: []model.PluginState{pluginState, pluginState2},
	}

	result := contracts.PluginResult{
		PluginName:    "aws:runScript",
		PluginID:      "plugin1",
		Status:        contracts.ResultStatusSuccess,
		StartDateTime: testStartDateTime,
		EndDateTime:   testEndDateTime,
	}
	result2 := contracts.PluginResult{
		PluginName:    "aws:runPowershellScript",
		PluginID:      "plugin2",
		Status:        contracts.ResultStatusSuccess,
		StartDateTime: testStartDateTime,
		EndDateTime:   testEndDateTime,
	}
	results := make(map[string]*contracts.PluginResult)
	results["plugin1"] = &result
	results["plugin2"] = &result2

	return &TestCase{
		context:      contextMock,
		docStore:     docStore,
		docState:     docState,
		processMock:  processMock,
		results:      results,
		resultStatus: contracts.ResultStatusSuccess,
	}
}

func TestInitializeNewProcess(t *testing.T) {
	testCase := CreateTestCase()
	channelMock := new(channelmock.MockedChannel)
	channelCreator = func(log log.T, mode channel.Mode, documentID string) (channel.Channel, error, bool) {
		assert.Equal(t, mode, channel.ModeMaster)
		assert.Equal(t, testDocumentID, documentID)
		return channelMock, nil, false
	}
	processCreator = func(name string, argv []string) (proc.OSProcess, error) {
		assert.Equal(t, name, appconfig.DefaultDocumentWorker)
		assert.Equal(t, argv, []string{testDocumentID})
		return testCase.processMock, nil
	}
	exe := &OutOfProcExecuter{
		ctx:        testCase.context,
		docState:   &testCase.docState,
		cancelFlag: task.NewChanneledCancelFlag(),
	}
	//assert Wait() syscall is called
	stopTimer := make(chan bool)

	testCase.processMock.On("Wait").Return(nil)
	testCase.processMock.On("Pid").Return(testPid)
	testCase.processMock.On("StartTime").Return(testStartDateTime)
	_, err := exe.initialize(stopTimer)
	assert.NoError(t, err)
	//Wait() returns immediately, block until zombie timeout
	<-stopTimer
	testCase.processMock.AssertExpectations(t)
	channelMock.AssertExpectations(t)
	//assert pid is saved
	assert.Equal(t, testPid, exe.docState.DocumentInformation.ProcInfo.Pid)
}

func TestCreateProcessFailed(t *testing.T) {
	testCase := CreateTestCase()
	channelMock := new(channelmock.MockedChannel)
	channelMock.On("Destroy").Return(nil)
	channelCreator = func(log log.T, mode channel.Mode, documentID string) (channel.Channel, error, bool) {
		assert.Equal(t, mode, channel.ModeMaster)
		assert.Equal(t, testDocumentID, documentID)
		return channelMock, nil, false
	}
	var err = errors.New("failed to create process")
	processCreator = func(name string, argv []string) (proc.OSProcess, error) {
		assert.Equal(t, name, appconfig.DefaultDocumentWorker)
		assert.Equal(t, argv, []string{testDocumentID})
		return nil, err
	}
	exe := &OutOfProcExecuter{
		ctx:        testCase.context,
		docState:   &testCase.docState,
		cancelFlag: task.NewChanneledCancelFlag(),
	}
	//assert Wait() syscall is called
	stopTimer := make(chan bool)
	_, err2 := exe.initialize(stopTimer)
	assert.Error(t, err2)
	channelMock.AssertExpectations(t)
}
func TestInitializeProcessUnexpectedExited(t *testing.T) {
	testCase := CreateTestCase()
	channelMock := new(channelmock.MockedChannel)
	channelMock.On("Destroy").Return(nil)
	channelCreator = func(log log.T, mode channel.Mode, documentID string) (channel.Channel, error, bool) {
		assert.Equal(t, mode, channel.ModeMaster)
		assert.Equal(t, testDocumentID, documentID)
		return channelMock, nil, false
	}
	processCreator = func(name string, argv []string) (proc.OSProcess, error) {
		assert.Equal(t, name, appconfig.DefaultDocumentWorker)
		assert.Equal(t, argv, []string{testDocumentID})
		return testCase.processMock, nil
	}
	cancel := task.NewChanneledCancelFlag()
	var err = errors.New("process exited with status 1")
	exe := &OutOfProcExecuter{
		ctx:        testCase.context,
		docState:   &testCase.docState,
		cancelFlag: cancel,
	}
	testCase.processMock.On("Wait").Return(err)
	testCase.processMock.On("Pid").Return(testPid)
	testCase.processMock.On("StartTime").Return(testStartDateTime)
	//assert Wait() syscall is called
	stopTimer := make(chan bool)
	_, err2 := exe.initialize(stopTimer)
	//initialize should work, but timeout should be triggered
	assert.NoError(t, err2)
	<-stopTimer
	//assert pid is saved
	assert.Equal(t, testPid, exe.docState.DocumentInformation.ProcInfo.Pid)
	//set job complete and kill is not called
	cancel.Set(task.Completed)
	testCase.processMock.AssertExpectations(t)
}

//TODO revisit this feature
//func TestTerminateWaitWhenJobComplete(t *testing.T) {
//	testCase := CreateTestCase()
//	cancel := task.NewChanneledCancelFlag()
//	exe := &OutOfProcExecuter{
//		ctx:        testCase.context,
//		docState:   &testCase.docState,
//		cancelFlag: cancel,
//	}
//	killChan := make(chan bool)
//	//wait hangs indefinitely
//	testCase.processMock.On("Pid").Return(testPid)
//	testCase.processMock.On("Wait").Run(func(mock.Arguments) {
//		<-killChan
//	}).Return(errors.New("process received SIGTERM"))
//	testCase.processMock.On("Kill").Run(func(mock.Arguments) {
//		killChan <- true
//	}).Return(nil)
//	stopTimer := make(chan bool)
//	cancel.Set(task.Completed)
//	//in this case, wait should immediately return and kill the process
//	exe.WaitForProcess(stopTimer, testCase.processMock)
//	testCase.processMock.AssertExpectations(t)
//}

func TestInitializeConnectOldOrphan(t *testing.T) {
	testCase := CreateTestCase()
	channelMock := new(channelmock.MockedChannel)
	channelCreator = func(log log.T, mode channel.Mode, documentID string) (channel.Channel, error, bool) {
		assert.Equal(t, mode, channel.ModeMaster)
		assert.Equal(t, testDocumentID, documentID)
		return channelMock, nil, true
	}
	//make sure not create new process
	isCreateCalled := false
	processCreator = func(name string, argv []string) (proc.OSProcess, error) {
		isCreateCalled = true
		return testCase.processMock, nil
	}
	//make sure the finder is called
	isFinderCalled := false
	processFinder = func(log log.T, procinfo model.OSProcInfo) bool {
		isFinderCalled = true
		return true
	}
	cancel := task.NewChanneledCancelFlag()
	exe := &OutOfProcExecuter{
		ctx:        testCase.context,
		docState:   &testCase.docState,
		cancelFlag: cancel,
	}
	stopTimer := make(chan bool)
	_, err := exe.initialize(stopTimer)
	//make sure timeout returns when cancel is set
	cancel.Set(task.Completed)
	assert.NoError(t, err)
	assert.False(t, isCreateCalled)
	assert.True(t, isFinderCalled)
	channelMock.AssertExpectations(t)
}

//TODO add Run() unittest

//this is needed, since after marshal-unmarshalling thru the data channel, the pointer value changed
func assertValueEqual(t *testing.T, a map[string]*contracts.PluginResult, b map[string]*contracts.PluginResult) {
	assert.Equal(t, len(a), len(b))
	for key, val := range a {
		assert.Equal(t, *val, *b[key])
	}
}
