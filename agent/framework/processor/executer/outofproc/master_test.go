package outofproc

import (
	"errors"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	executermocks "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/mock"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/messaging"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/proc"
	procmock "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/proc/mock"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/mocks/context"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/common/filewatcherbasedipc"
	channelmock "github.com/aws/amazon-ssm-agent/common/filewatcherbasedipc/mocks"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/aws/amazon-ssm-agent/core/executor"
	executormocks "github.com/aws/amazon-ssm-agent/core/executor/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type TestCase struct {
	context      *context.Mock
	docStore     *executermocks.MockDocumentStore
	docState     contracts.DocumentState
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
var testDocumentCreatedDate = "2017-06-10T01-23-07.853Z"
var testStartDateTime = time.Date(2017, 8, 13, 0, 0, 0, 0, time.UTC)
var testEndDateTime = time.Date(2017, 8, 13, 0, 0, 1, 0, time.UTC)
var testPid = 100

var logger = logmocks.NewMockLog()

func CreateTestCase() *TestCase {
	contextMock := context.NewMockDefaultWithContext([]string{"MASTER"})
	docStore := new(executermocks.MockDocumentStore)
	processMock := new(procmock.MockedOSProcess)
	docInfo := contracts.DocumentInfo{
		CreatedDate:     "2017-06-10T01-23-07.853Z",
		MessageID:       testMessageID,
		DocumentName:    testDocumentName,
		AssociationID:   testAssociationID,
		DocumentID:      testDocumentID,
		InstanceID:      testInstanceID,
		DocumentVersion: testDocumentVersion,
		RunCount:        0,
	}

	pluginState := contracts.PluginState{
		Name: "aws:runScript",
		Id:   "plugin1",
	}
	pluginState2 := contracts.PluginState{
		Name: "aws:runPowershellScript",
		Id:   "plugin2",
	}
	docState := contracts.DocumentState{
		DocumentInformation:        docInfo,
		DocumentType:               "SendCommand",
		InstancePluginsInformation: []contracts.PluginState{pluginState, pluginState2},
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
	channelCreator = func(log log.T, identity identity.IAgentIdentity, mode filewatcherbasedipc.Mode, documentID string) (filewatcherbasedipc.IPCChannel, error, bool) {
		assert.Equal(t, mode, filewatcherbasedipc.ModeMaster)
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

func TestInitializeNewProcessForSession(t *testing.T) {
	testCase := createTestCaseForStartSession()
	channelMock := new(channelmock.MockedChannel)
	channelCreator = func(log log.T, identity identity.IAgentIdentity, mode filewatcherbasedipc.Mode, documentID string) (filewatcherbasedipc.IPCChannel, error, bool) {
		assert.Equal(t, mode, filewatcherbasedipc.ModeMaster)
		assert.Equal(t, testDocumentID, documentID)
		return channelMock, nil, false
	}
	processCreator = func(name string, argv []string) (proc.OSProcess, error) {
		assert.Equal(t, name, appconfig.DefaultSessionWorker)
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

func createTestCaseForStartSession() *TestCase {
	contextMock := context.NewMockDefaultWithContext([]string{"MASTER"})
	docStore := new(executermocks.MockDocumentStore)
	processMock := new(procmock.MockedOSProcess)
	docInfo := contracts.DocumentInfo{
		CreatedDate:     testDocumentCreatedDate,
		MessageID:       testMessageID,
		DocumentName:    testDocumentName,
		DocumentID:      testDocumentID,
		InstanceID:      testInstanceID,
		DocumentVersion: testDocumentVersion,
		RunCount:        0,
	}

	pluginState := contracts.PluginState{
		Name: "Standard_Stream",
		Id:   "plugin1",
	}
	docState := contracts.DocumentState{
		DocumentInformation:        docInfo,
		DocumentType:               "StartSession",
		InstancePluginsInformation: []contracts.PluginState{pluginState},
	}

	results := make(map[string]*contracts.PluginResult)
	return &TestCase{
		context:      contextMock,
		docStore:     docStore,
		docState:     docState,
		processMock:  processMock,
		results:      results,
		resultStatus: contracts.ResultStatusSuccess,
	}
}

func TestCreateProcessFailed(t *testing.T) {
	testCase := CreateTestCase()
	channelMock := new(channelmock.MockedChannel)
	channelMock.On("Destroy").Return(nil)
	channelCreator = func(log log.T, identity identity.IAgentIdentity, mode filewatcherbasedipc.Mode, documentID string) (filewatcherbasedipc.IPCChannel, error, bool) {
		assert.Equal(t, mode, filewatcherbasedipc.ModeMaster)
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
	channelCreator = func(log log.T, identity identity.IAgentIdentity, mode filewatcherbasedipc.Mode, documentID string) (filewatcherbasedipc.IPCChannel, error, bool) {
		assert.Equal(t, mode, filewatcherbasedipc.ModeMaster)
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
	channelCreator = func(log log.T, identity identity.IAgentIdentity, mode filewatcherbasedipc.Mode, documentID string) (filewatcherbasedipc.IPCChannel, error, bool) {
		assert.Equal(t, mode, filewatcherbasedipc.ModeMaster)
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
	processFinder = func(log log.T, procinfo contracts.OSProcInfo, executor executor.IExecutor) bool {
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

// this is needed, since after marshal-unmarshalling thru the data channel, the pointer value changed
func assertValueEqual(t *testing.T, a map[string]*contracts.PluginResult, b map[string]*contracts.PluginResult) {
	assert.Equal(t, len(a), len(b))
	for key, val := range a {
		assert.Equal(t, *val, *b[key])
	}
}

func TestProcessFinder_PidZero_ReturnsFalse(t *testing.T) {
	mockLog := logmocks.NewMockLog()
	mockExec := &executormocks.IExecutor{}

	result := findProcess(mockLog, contracts.OSProcInfo{Pid: 0}, mockExec)

	assert.False(t, result)
	mockExec.AssertNotCalled(t, "IsPidRunning")
}

func TestProcessFinder_PidNotRunning_ReturnsFalse(t *testing.T) {
	mockLog := logmocks.NewMockLog()
	mockExec := &executormocks.IExecutor{}
	mockExec.On("IsPidRunning", 1234).Return(false, nil)

	result := findProcess(mockLog, contracts.OSProcInfo{Pid: 1234}, mockExec)

	assert.False(t, result)
	mockExec.AssertNotCalled(t, "Processes")
}

func TestProcessFinder_IsPidRunningError_ReturnsFalse(t *testing.T) {
	mockLog := logmocks.NewMockLog()
	mockExec := &executormocks.IExecutor{}
	mockExec.On("IsPidRunning", 1234).Return(false, errors.New("access denied"))

	result := findProcess(mockLog, contracts.OSProcInfo{Pid: 1234}, mockExec)

	assert.False(t, result)
}

func TestProcessFinder_PidRunning_SSMWorker_ReturnsTrue(t *testing.T) {
	mockLog := logmocks.NewMockLog()
	mockExec := &executormocks.IExecutor{}
	mockExec.On("IsPidRunning", 5678).Return(true, nil)
	mockExec.On("Processes").Return([]executor.OsProcess{
		{Pid: 5678, Executable: "ssm-document-worker"},
	}, nil)

	result := findProcess(mockLog, contracts.OSProcInfo{Pid: 5678}, mockExec)

	assert.True(t, result)
}

func TestProcessFinder_PidRunning_NotSSMWorker_ReturnsFalse(t *testing.T) {
	mockLog := logmocks.NewMockLog()
	mockExec := &executormocks.IExecutor{}
	mockExec.On("IsPidRunning", 5678).Return(true, nil)
	mockExec.On("Processes").Return([]executor.OsProcess{
		{Pid: 5678, Executable: "nginx"},
	}, nil)

	result := findProcess(mockLog, contracts.OSProcInfo{Pid: 5678}, mockExec)

	assert.False(t, result)
}

func TestProcessFinder_PidRunning_ProcessesError_ReturnsTrue(t *testing.T) {
	mockLog := logmocks.NewMockLog()
	mockExec := &executormocks.IExecutor{}
	mockExec.On("IsPidRunning", 5678).Return(true, nil)
	mockExec.On("Processes").Return([]executor.OsProcess(nil), errors.New("ps failed"))

	result := findProcess(mockLog, contracts.OSProcInfo{Pid: 5678}, mockExec)

	assert.True(t, result)
}

func TestProcessFinder_PidRunning_NotInProcessList_ReturnsFalse(t *testing.T) {
	mockLog := logmocks.NewMockLog()
	mockExec := &executormocks.IExecutor{}
	mockExec.On("IsPidRunning", 5678).Return(true, nil)
	mockExec.On("Processes").Return([]executor.OsProcess{
		{Pid: 1111, Executable: "sshd"},
		{Pid: 2222, Executable: "ssm-agent-worker"},
	}, nil)

	result := findProcess(mockLog, contracts.OSProcInfo{Pid: 5678}, mockExec)

	assert.False(t, result)
}

func TestProcessFinder_PidRunning_CaseInsensitive_ReturnsTrue(t *testing.T) {
	mockLog := logmocks.NewMockLog()
	mockExec := &executormocks.IExecutor{}
	mockExec.On("IsPidRunning", 5678).Return(true, nil)
	mockExec.On("Processes").Return([]executor.OsProcess{
		{Pid: 5678, Executable: "SSM-Document-Worker.exe"},
	}, nil)

	result := findProcess(mockLog, contracts.OSProcInfo{Pid: 5678}, mockExec)

	assert.True(t, result)
}

// The agent worker is not a document or session worker, so a pid holding it must not be adopted as
// the document's worker. The "ssm-" prefix this replaced would have accepted it.
func TestProcessFinder_PidRunning_AgentWorker_ReturnsFalse(t *testing.T) {
	mockLog := logmocks.NewMockLog()
	mockExec := &executormocks.IExecutor{}
	mockExec.On("IsPidRunning", 5678).Return(true, nil)
	mockExec.On("Processes").Return([]executor.OsProcess{
		{Pid: 5678, Executable: "ssm-agent-worker.exe"},
	}, nil)

	result := findProcess(mockLog, contracts.OSProcInfo{Pid: 5678}, mockExec)

	assert.False(t, result)
}

// A patch document must not stay in the shutdown registry once messaging returns. The paths that
// leak are the ones that never deliver a terminal result -- here, a messaging error -- so this
// exercises messaging() itself rather than calling Release directly, which would still pass if the
// deferred call were dropped.
func TestMessaging_ReleasesPatchDocFromShutdownRegistryOnError(t *testing.T) {
	testCase := CreateTestCase()
	testCase.docState.DocumentInformation.DocumentName = "AWS-RunPatchBaseline"
	testCase.docStore.On("Save", mock.Anything).Return()

	// Closing the inbound channel makes Messaging return without a terminal result.
	recvChan := make(chan string)
	close(recvChan)
	channelMock := new(channelmock.MockedChannel)
	channelMock.On("GetPath").Return("test-path")
	channelMock.On("GetMessage").Return(recvChan)

	exe := &OutOfProcExecuter{
		ctx:        testCase.context,
		docState:   &testCase.docState,
		cancelFlag: task.NewChanneledCancelFlag(),
	}
	resChan := make(chan contracts.DocumentResult, len(testCase.docState.InstancePluginsInformation)+1)

	exe.messaging(logger, channelMock, resChan, exe.cancelFlag, make(chan bool, 1), testCase.docStore)

	// PrepareForShutdown reports how many in-flight patch documents it prepared. A leaked
	// registration would let it mutate and re-persist a document that is no longer running.
	assert.Equal(t, 0, messaging.PrepareForShutdown(logger),
		"messaging must drop the patch document from the shutdown registry before returning")
}

// createPatchResumeTestCase builds the state initialize() sees when a patch document is resumed
// after a reboot: the previous worker's channel is still on disk and the document is InProgress.
func createPatchResumeTestCase() *TestCase {
	testCase := CreateTestCase()
	testCase.docState.DocumentInformation.DocumentName = "AWS-RunPatchBaseline"
	testCase.docState.DocumentInformation.DocumentStatus = contracts.ResultStatusInProgress
	return testCase
}

// The worker must be launched on the fresh channel, not the stale one: asserting only that Destroy
// was called would still pass if the stale channel were handed to the worker, which is the defect.
func TestInitializePatchResume_DestroysStaleChannelAndUsesNewOne(t *testing.T) {
	testCase := createPatchResumeTestCase()
	staleChannel := new(channelmock.MockedChannel)
	staleChannel.On("Destroy").Return(nil)
	freshChannel := new(channelmock.MockedChannel)

	createCount := 0
	channelCreator = func(log log.T, identity identity.IAgentIdentity, mode filewatcherbasedipc.Mode, documentID string) (filewatcherbasedipc.IPCChannel, error, bool) {
		createCount++
		assert.Equal(t, filewatcherbasedipc.ModeMaster, mode)
		assert.Equal(t, testDocumentID, documentID)
		if createCount == 1 {
			// the channel left behind by the previous worker
			return staleChannel, nil, true
		}
		return freshChannel, nil, false
	}
	processCreator = func(name string, argv []string) (proc.OSProcess, error) {
		assert.Equal(t, appconfig.DefaultDocumentWorker, name)
		return testCase.processMock, nil
	}
	processFinder = func(log log.T, procinfo contracts.OSProcInfo, executor executor.IExecutor) bool {
		// the prior worker exited normally before the resume, so it is genuinely gone
		return false
	}
	exe := &OutOfProcExecuter{
		ctx:        testCase.context,
		docState:   &testCase.docState,
		cancelFlag: task.NewChanneledCancelFlag(),
	}
	// buffered so the zombie timer in WaitForProcess can complete its send without this test
	// spending defaultZombieProcessTimeout waiting on it
	stopTimer := make(chan bool, 1)
	testCase.processMock.On("Wait").Return(nil)
	testCase.processMock.On("Pid").Return(testPid)
	testCase.processMock.On("StartTime").Return(testStartDateTime)

	ipc, err := exe.initialize(stopTimer)

	assert.NoError(t, err)
	assert.Equal(t, 2, createCount, "the channel must be recreated after being destroyed")
	assert.Same(t, freshChannel, ipc, "the resumed run must use the recreated channel")
	assert.NotSame(t, staleChannel, ipc, "the resumed run must not reuse the stale channel")
	staleChannel.AssertExpectations(t)
	freshChannel.AssertNotCalled(t, "Destroy")
	assert.Equal(t, testPid, exe.docState.DocumentInformation.ProcInfo.Pid, "the new worker's pid must be recorded")
}

// If the channel cannot be recreated there is nowhere safe to run, so initialize must surface the
// error and must not launch a worker.
func TestInitializePatchResume_RecreateFailsReturnsErrorAndStartsNoWorker(t *testing.T) {
	testCase := createPatchResumeTestCase()
	staleChannel := new(channelmock.MockedChannel)
	staleChannel.On("Destroy").Return(nil)
	createErr := errors.New("failed to create channel directory")

	createCount := 0
	channelCreator = func(log log.T, identity identity.IAgentIdentity, mode filewatcherbasedipc.Mode, documentID string) (filewatcherbasedipc.IPCChannel, error, bool) {
		createCount++
		if createCount == 1 {
			return staleChannel, nil, true
		}
		return nil, createErr, false
	}
	isCreateCalled := false
	processCreator = func(name string, argv []string) (proc.OSProcess, error) {
		isCreateCalled = true
		return testCase.processMock, nil
	}
	processFinder = func(log log.T, procinfo contracts.OSProcInfo, executor executor.IExecutor) bool {
		return false
	}
	exe := &OutOfProcExecuter{
		ctx:        testCase.context,
		docState:   &testCase.docState,
		cancelFlag: task.NewChanneledCancelFlag(),
	}
	stopTimer := make(chan bool)

	_, err := exe.initialize(stopTimer)

	assert.Error(t, err)
	assert.Equal(t, createErr, err)
	assert.Equal(t, 2, createCount)
	assert.False(t, isCreateCalled, "no worker may be launched when the channel could not be recreated")
	staleChannel.AssertExpectations(t)
}

// Negative control on the branch scoping: a non-patch document whose worker is gone takes the zombie
// path, which must neither destroy nor recreate the channel.
func TestInitializeNonPatchDocument_DoesNotRecreateChannel(t *testing.T) {
	testCase := CreateTestCase()
	testCase.docState.DocumentInformation.DocumentStatus = contracts.ResultStatusInProgress
	channelMock := new(channelmock.MockedChannel)

	createCount := 0
	channelCreator = func(log log.T, identity identity.IAgentIdentity, mode filewatcherbasedipc.Mode, documentID string) (filewatcherbasedipc.IPCChannel, error, bool) {
		createCount++
		return channelMock, nil, true
	}
	isCreateCalled := false
	processCreator = func(name string, argv []string) (proc.OSProcess, error) {
		isCreateCalled = true
		return testCase.processMock, nil
	}
	processFinder = func(log log.T, procinfo contracts.OSProcInfo, executor executor.IExecutor) bool {
		return false
	}
	cancel := task.NewChanneledCancelFlag()
	exe := &OutOfProcExecuter{
		ctx:        testCase.context,
		docState:   &testCase.docState,
		cancelFlag: cancel,
	}
	stopTimer := make(chan bool)

	ipc, err := exe.initialize(stopTimer)
	cancel.Set(task.Completed)

	assert.NoError(t, err)
	assert.Equal(t, 1, createCount, "the non-patch path must not recreate the channel")
	assert.Same(t, channelMock, ipc)
	assert.False(t, isCreateCalled)
	channelMock.AssertNotCalled(t, "Destroy")
}

// A terminal result left unread in the channel is replayed if that channel is reused, and the
// document is then reported complete while the worker the resume launched is still running. Driven
// through the queue-backed fake channel so the stale datagram is real rather than mocked away.
func TestInitializePatchResume_DoesNotReplayStaleTerminalDatagram(t *testing.T) {
	testCase := createPatchResumeTestCase()
	// the fake channel's registry is package-global and keyed by document id, so use an id no other
	// test in this file shares; otherwise a sibling test's leftovers can mask a replay
	staleReplayDocumentID := "test-doc-stale-replay"
	testCase.docState.DocumentInformation.DocumentID = staleReplayDocumentID

	// the previous worker's side of the channel, still holding its unread terminal result
	staleWorkerSide := channelmock.NewFakeChannel(logger, filewatcherbasedipc.ModeWorker, staleReplayDocumentID)
	staleTerminalResult, datagramErr := messaging.CreateDatagram(messaging.MessageTypeComplete, contracts.DocumentResult{
		Status:     contracts.ResultStatusFailed,
		LastPlugin: "",
	})
	assert.NoError(t, datagramErr)
	assert.NoError(t, staleWorkerSide.Send(staleTerminalResult))
	assert.True(t, channelmock.IsExists(staleReplayDocumentID), "the stale channel must exist before the resume")

	channelCreator = func(log log.T, identity identity.IAgentIdentity, mode filewatcherbasedipc.Mode, documentID string) (filewatcherbasedipc.IPCChannel, error, bool) {
		// mirrors the real creator: the channel is "found" only while its directory still exists
		isFound := channelmock.IsExists(documentID)
		return channelmock.NewFakeChannel(logger, mode, documentID), nil, isFound
	}
	processCreator = func(name string, argv []string) (proc.OSProcess, error) {
		return testCase.processMock, nil
	}
	processFinder = func(log log.T, procinfo contracts.OSProcInfo, executor executor.IExecutor) bool {
		return false
	}
	exe := &OutOfProcExecuter{
		ctx:        testCase.context,
		docState:   &testCase.docState,
		cancelFlag: task.NewChanneledCancelFlag(),
	}
	stopTimer := make(chan bool, 1)
	testCase.processMock.On("Wait").Return(nil)
	testCase.processMock.On("Pid").Return(testPid)
	testCase.processMock.On("StartTime").Return(testStartDateTime)

	ipc, err := exe.initialize(stopTimer)
	assert.NoError(t, err)
	// leave no channel behind in the fake's package-level registry
	defer ipc.Destroy()

	// the resumed run must see an empty channel; reusing the stale one delivers the terminal result
	// here and the document is declared complete while its new worker is still running
	select {
	case replayed := <-ipc.GetMessage():
		t.Fatalf("stale terminal datagram was replayed on resume: %v", replayed)
	case <-time.After(500 * time.Millisecond):
	}
}

// Destroy() returns nothing and only logs when the removal fails, so the creator still reporting the
// channel as found is the only detection available; running a worker against it is the defect.
func TestInitializePatchResume_StaleChannelSurvivesDestroyFailsResume(t *testing.T) {
	testCase := createPatchResumeTestCase()
	staleChannel := new(channelmock.MockedChannel)
	staleChannel.On("Destroy").Return(nil)
	survivingChannel := new(channelmock.MockedChannel)
	// Run discards the channel once initialize errors, so this one must be closed here or its watch
	// goroutine and fsnotify handle leak.
	survivingChannel.On("Close").Return(nil)

	createCount := 0
	channelCreator = func(log log.T, identity identity.IAgentIdentity, mode filewatcherbasedipc.Mode, documentID string) (filewatcherbasedipc.IPCChannel, error, bool) {
		createCount++
		if createCount == 1 {
			return staleChannel, nil, true
		}
		// Destroy did not remove the directory, so the recreate rediscovered it
		return survivingChannel, nil, true
	}
	isCreateCalled := false
	processCreator = func(name string, argv []string) (proc.OSProcess, error) {
		isCreateCalled = true
		return testCase.processMock, nil
	}
	processFinder = func(log log.T, procinfo contracts.OSProcInfo, executor executor.IExecutor) bool {
		return false
	}
	exe := &OutOfProcExecuter{
		ctx:        testCase.context,
		docState:   &testCase.docState,
		cancelFlag: task.NewChanneledCancelFlag(),
	}
	stopTimer := make(chan bool, 1)

	_, err := exe.initialize(stopTimer)

	assert.Error(t, err, "a channel that survived Destroy must fail the resume, not be reused")
	assert.Contains(t, err.Error(), "still present after destroy")
	assert.Equal(t, 2, createCount)
	assert.False(t, isCreateCalled, "no worker may be launched against a channel that may replay a stale result")
	staleChannel.AssertExpectations(t)
	survivingChannel.AssertExpectations(t)
	survivingChannel.AssertNotCalled(t, "Destroy")
}
