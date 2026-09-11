package messaging

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	agentlog "github.com/aws/amazon-ssm-agent/agent/log"
	contextmocks "github.com/aws/amazon-ssm-agent/agent/mocks/context"
	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	taskmocks "github.com/aws/amazon-ssm-agent/agent/mocks/task"

	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
)

var contextMock = contextmocks.NewMockDefault()
var testInstanceID = "i-400e1090"
var testDocumentID = "13e8e6ad-e195-4ccb-86ee-328153b0dafe"
var testMessageID = "MessageID"
var testAssociationID = "AssociationID"
var testDocumentName = "AWS-RunPowerShellScript"
var testDocumentVersion = "testVersion"
var testStartDateTime = time.Date(2017, 8, 13, 0, 0, 0, 0, time.UTC)
var testEndDateTime = time.Date(2017, 8, 13, 0, 0, 1, 0, time.UTC)
var testPluginReplyRawJSON string
var testPluginReply2RawJSON string
var testUnknownTypeRawJSON string
var testUnknownTypeRawJSON2 string
var testDocumentCompleteRawJSON string
var testPluginsRawJSON string
var testCancelRawJSON string

type TestCase struct {
	context      *contextmocks.Mock
	docState     contracts.DocumentState
	results      map[string]*contracts.PluginResult
	resultStatus contracts.ResultStatus
}

func CreateTestCase() *TestCase {
	contextMock := contextmocks.NewMockDefaultWithContext([]string{"MASTER"})
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
		Error:         "error occurred",
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
	//corresponding rawJSON data
	//TODO this is V2 Schema, add V1 schema later
	testPluginReplyRawJSON = "{\"version\":\"1.0\",\"type\":\"reply\",\"content\":\"{\\\"DocumentName\\\":\\\"\\\",\\\"DocumentVersion\\\":\\\"\\\",\\\"MessageID\\\":\\\"\\\",\\\"AssociationID\\\":\\\"\\\",\\\"PluginResults\\\":{\\\"plugin1\\\":{\\\"pluginName\\\":\\\"aws:runScript\\\",\\\"pluginID\\\":\\\"plugin1\\\",\\\"status\\\":\\\"Success\\\",\\\"code\\\":0,\\\"output\\\":null,\\\"startDateTime\\\":\\\"2017-08-13T00:00:00Z\\\",\\\"endDateTime\\\":\\\"2017-08-13T00:00:01Z\\\",\\\"outputS3BucketName\\\":\\\"\\\",\\\"outputS3KeyPrefix\\\":\\\"\\\",\\\"stepName\\\":\\\"\\\",\\\"error\\\":\\\"error occurred\\\",\\\"standardOutput\\\":\\\"\\\",\\\"standardError\\\":\\\"\\\"}},\\\"Status\\\":\\\"InProgress\\\",\\\"LastPlugin\\\":\\\"plugin1\\\",\\\"NPlugins\\\":0,\\\"UpstreamServiceName\\\":\\\"\\\",\\\"RelatedDocumentType\\\":\\\"\\\",\\\"ResultType\\\":\\\"\\\"}\"}"
	testPluginReply2RawJSON = "{\"version\":\"1.0\",\"type\":\"reply\",\"content\":\"{\\\"DocumentName\\\":\\\"\\\",\\\"DocumentVersion\\\":\\\"\\\",\\\"MessageID\\\":\\\"\\\",\\\"AssociationID\\\":\\\"\\\",\\\"PluginResults\\\":{\\\"plugin1\\\":{\\\"pluginID\\\":\\\"plugin1\\\",\\\"pluginName\\\":\\\"aws:runScript\\\",\\\"status\\\":\\\"Success\\\",\\\"code\\\":0,\\\"output\\\":null,\\\"startDateTime\\\":\\\"2017-08-13T00:00:00Z\\\",\\\"endDateTime\\\":\\\"2017-08-13T00:00:01Z\\\",\\\"outputS3BucketName\\\":\\\"\\\",\\\"outputS3KeyPrefix\\\":\\\"\\\",\\\"stepName\\\":\\\"\\\",\\\"error\\\":\\\"error occurred\\\",\\\"standardOutput\\\":\\\"\\\",\\\"standardError\\\":\\\"\\\"},\\\"plugin2\\\":{\\\"pluginID\\\":\\\"plugin2\\\",\\\"pluginName\\\":\\\"aws:runPowershellScript\\\",\\\"status\\\":\\\"Success\\\",\\\"code\\\":0,\\\"output\\\":null,\\\"startDateTime\\\":\\\"2017-08-13T00:00:00Z\\\",\\\"endDateTime\\\":\\\"2017-08-13T00:00:01Z\\\",\\\"outputS3BucketName\\\":\\\"\\\",\\\"outputS3KeyPrefix\\\":\\\"\\\",\\\"stepName\\\":\\\"\\\",\\\"error\\\":\\\"\\\",\\\"standardOutput\\\":\\\"\\\",\\\"standardError\\\":\\\"\\\"}},\\\"Status\\\":\\\"InProgress\\\",\\\"LastPlugin\\\":\\\"plugin2\\\",\\\"NPlugins\\\":0,\\\"UpstreamServiceName\\\":\\\"\\\",\\\"RelatedDocumentType\\\":\\\"\\\",\\\"ResultType\\\":\\\"\\\"}\"}"
	testDocumentCompleteRawJSON = "{\"version\":\"1.0\",\"type\":\"complete\",\"content\":\"{\\\"DocumentName\\\":\\\"\\\",\\\"DocumentVersion\\\":\\\"\\\",\\\"MessageID\\\":\\\"\\\",\\\"AssociationID\\\":\\\"\\\",\\\"PluginResults\\\":{\\\"plugin1\\\":{\\\"pluginID\\\":\\\"plugin1\\\",\\\"pluginName\\\":\\\"aws:runScript\\\",\\\"status\\\":\\\"Success\\\",\\\"code\\\":0,\\\"output\\\":null,\\\"startDateTime\\\":\\\"2017-08-13T00:00:00Z\\\",\\\"endDateTime\\\":\\\"2017-08-13T00:00:01Z\\\",\\\"outputS3BucketName\\\":\\\"\\\",\\\"outputS3KeyPrefix\\\":\\\"\\\",\\\"stepName\\\":\\\"\\\",\\\"error\\\":\\\"error occurred\\\",\\\"standardOutput\\\":\\\"\\\",\\\"standardError\\\":\\\"\\\"},\\\"plugin2\\\":{\\\"pluginID\\\":\\\"plugin2\\\",\\\"pluginName\\\":\\\"aws:runPowershellScript\\\",\\\"status\\\":\\\"Success\\\",\\\"code\\\":0,\\\"output\\\":null,\\\"startDateTime\\\":\\\"2017-08-13T00:00:00Z\\\",\\\"endDateTime\\\":\\\"2017-08-13T00:00:01Z\\\",\\\"outputS3BucketName\\\":\\\"\\\",\\\"outputS3KeyPrefix\\\":\\\"\\\",\\\"stepName\\\":\\\"\\\",\\\"error\\\":\\\"\\\",\\\"standardOutput\\\":\\\"\\\",\\\"standardError\\\":\\\"\\\"}},\\\"Status\\\":\\\"Success\\\",\\\"LastPlugin\\\":\\\"\\\",\\\"NPlugins\\\":0,\\\"UpstreamServiceName\\\":\\\"\\\",\\\"RelatedDocumentType\\\":\\\"\\\",\\\"ResultType\\\":\\\"\\\"}\"}"
	testPluginsRawJSON = "{\"version\":\"1.0\",\"type\":\"pluginconfig\",\"content\":\"{\\\"DocumentInformation\\\":{\\\"DocumentID\\\":\\\"\\\",\\\"CommandID\\\":\\\"\\\",\\\"AssociationID\\\":\\\"\\\",\\\"InstanceID\\\":\\\"\\\",\\\"MessageID\\\":\\\"\\\",\\\"RunID\\\":\\\"\\\",\\\"CreatedDate\\\":\\\"\\\",\\\"DocumentName\\\":\\\"\\\",\\\"DocumentVersion\\\":\\\"\\\",\\\"DocumentStatus\\\":\\\"\\\",\\\"RunCount\\\":0,\\\"ProcInfo\\\":{\\\"Pid\\\":0,\\\"StartTime\\\":\\\"2006-01-02T15:04:05Z\\\"}},\\\"DocumentType\\\":\\\"SendCommand\\\",\\\"SchemaVersion\\\":\\\"\\\",\\\"InstancePluginsInformation\\\":[{\\\"Configuration\\\":{\\\"Settings\\\":null,\\\"Properties\\\":null,\\\"OutputS3KeyPrefix\\\":\\\"\\\",\\\"OutputS3BucketName\\\":\\\"\\\",\\\"OrchestrationDirectory\\\":\\\"\\\",\\\"MessageId\\\":\\\"\\\",\\\"BookKeepingFileName\\\":\\\"\\\",\\\"PluginName\\\":\\\"\\\",\\\"PluginID\\\":\\\"\\\",\\\"DefaultWorkingDirectory\\\":\\\"\\\",\\\"Preconditions\\\":null,\\\"IsPreconditionEnabled\\\":false},\\\"Name\\\":\\\"aws:runScript\\\",\\\"Result\\\":{\\\"pluginName\\\":\\\"\\\",\\\"status\\\":\\\"\\\",\\\"code\\\":0,\\\"output\\\":null,\\\"startDateTime\\\":\\\"2017-08-13T00:00:00Z\\\",\\\"endDateTime\\\":\\\"2017-08-13T00:00:00Z\\\",\\\"outputS3BucketName\\\":\\\"\\\",\\\"outputS3KeyPrefix\\\":\\\"\\\",\\\"error\\\":\\\"\\\",\\\"standardOutput\\\":\\\"\\\",\\\"standardError\\\":\\\"\\\"},\\\"Id\\\":\\\"aws:runScript\\\"}],\\\"CancelInformation\\\":{\\\"CancelMessageID\\\":\\\"\\\",\\\"CancelCommandID\\\":\\\"\\\",\\\"Payload\\\":\\\"\\\",\\\"DebugInfo\\\":\\\"\\\"},\\\"IOConfig\\\":{\\\"OrchestrationDirectory\\\":\\\"\\\",\\\"OutputS3BucketName\\\":\\\"\\\",\\\"OutputS3KeyPrefix\\\":\\\"\\\"}}\"}"
	testUnknownTypeRawJSON = "{\"version\":\"1.0\",\"type\":\"some unknown type\",\"content\":\"\"}"
	testUnknownTypeRawJSON2 = "a very bad string"
	testCancelRawJSON = "{\"version\":\"1.0\",\"type\":\"cancel\",\"content\":\"\"}"
	return &TestCase{
		context:      contextMock,
		docState:     docState,
		results:      results,
		resultStatus: contracts.ResultStatusSuccess,
	}
}

// TODO test cancel message
func TestExecuterBackendStart_Shutdown(t *testing.T) {
	testCase := CreateTestCase()
	outputChan := make(chan contracts.DocumentResult, 10)
	stopChan := make(chan int, 1)
	inputChan := make(chan string)
	cancel := task.NewChanneledCancelFlag()
	backend := ExecuterBackend{
		output:     outputChan,
		input:      inputChan,
		cancelFlag: cancel,
		stopChan:   stopChan,
		docState:   &testCase.docState,
	}
	closed := make(chan bool)
	go func() {
		<-inputChan
		stopType := <-stopChan
		assert.Equal(t, stopTypeShutdown, stopType)
		closed <- true
	}()
	cancel.Set(task.ShutDown)
	logMock := log.NewMockLog()
	backend.start(logMock, testCase.docState)
	//make sure input is closed
	<-inputChan
	//make sure assertion are made
	<-closed
}

// test the datagram mashalling v1
func TestExecuterBackend_ProcessV1(t *testing.T) {
	testCase := CreateTestCase()
	outputChan := make(chan contracts.DocumentResult, 10)
	stopChan := make(chan int, 1)
	cancel := taskmocks.NewMockDefault()
	backend := ExecuterBackend{
		cancelFlag: cancel,
		output:     outputChan,
		stopChan:   stopChan,
		docState:   &testCase.docState,
	}
	err := backend.Process(testPluginReplyRawJSON)
	assert.NoError(t, err)
	res := <-outputChan
	assert.Equal(t, "plugin1", res.LastPlugin)
	assert.Equal(t, 1, len(res.PluginResults))
	assert.EqualValues(t, testCase.results["plugin1"], res.PluginResults["plugin1"])
	assert.Equal(t, contracts.ResultStatusInProgress, res.Status)
	assert.Equal(t, testDocumentName, res.DocumentName)
	assert.Equal(t, testAssociationID, res.AssociationID)
	assert.Equal(t, testMessageID, res.MessageID)
	assert.EqualValues(t, testCase.docState.InstancePluginsInformation[0].Result, *testCase.results["plugin1"])
	assert.Equal(t, len(testCase.docState.InstancePluginsInformation), res.NPlugins)
	assert.Equal(t, testDocumentVersion, res.DocumentVersion)
	err = backend.Process(testDocumentCompleteRawJSON)
	assert.NoError(t, err)
	res = <-outputChan
	//verify stop signal received
	assert.Equal(t, stopTypeTerminate, <-stopChan)
	assert.Equal(t, "", res.LastPlugin)
	assertValueEqual(t, testCase.results, res.PluginResults)
	assert.Equal(t, testCase.resultStatus, res.Status)
	assert.Equal(t, testDocumentName, res.DocumentName)
	assert.Equal(t, testAssociationID, res.AssociationID)
	assert.Equal(t, testMessageID, res.MessageID)
	assert.Equal(t, len(testCase.docState.InstancePluginsInformation), res.NPlugins)
	assert.Equal(t, testDocumentVersion, res.DocumentVersion)
	cancel.AssertExpectations(t)
}

// test the datagram mashalling v1
func TestExecuterBackend_ProcessUnsupportedVersion(t *testing.T) {
	testCase := CreateTestCase()
	outputChan := make(chan contracts.DocumentResult, 10)
	stopChan := make(chan int, 1)
	cancel := taskmocks.NewMockDefault()
	backend := ExecuterBackend{
		cancelFlag: cancel,
		output:     outputChan,
		stopChan:   stopChan,
		docState:   &testCase.docState,
	}
	err := backend.Process(testUnknownTypeRawJSON)
	assert.Error(t, err)
	logger.Info(err)
	err = backend.Process(testUnknownTypeRawJSON2)
	assert.Error(t, err)
	logger.Info(err)
}

func TestWorkerBackend_TestBackendStateUpdate(t *testing.T) {
	_ = CreateTestCase()
	inputChan := make(chan string, 10)
	cancelFlag := new(taskmocks.MockCancelFlag)
	cancelFlag.On("Set", task.Canceled).Return(nil)
	pluginRunner := func(
		context context.T,
		docState contracts.DocumentState,
		resChan chan contracts.PluginResult,
		cancelFlag task.CancelFlag,
	) {
		close(resChan)
	}
	stopChan := make(chan int, 1)
	backend := WorkerBackend{
		ctx:        contextMock,
		input:      inputChan,
		cancelFlag: cancelFlag,
		runner:     pluginRunner,
		stopChan:   stopChan,
		state:      atomic.Int32{},
	}
	assert.Equal(t, backend.GetBackendState(), BackendStateInit)
	backend.Process(testPluginsRawJSON)
	assert.Equal(t, backend.GetBackendState(), BackendStateProc)
}

func TestWorkerBackend_ProcessCancelV1(t *testing.T) {
	_ = CreateTestCase()
	inputChan := make(chan string, 10)
	cancelFlag := new(taskmocks.MockCancelFlag)
	cancelFlag.On("Set", task.Canceled).Return(nil)
	isRunnerCalled := false
	pluginRunner := func(
		context context.T,
		docState contracts.DocumentState,
		resChan chan contracts.PluginResult,
		cancelFlag task.CancelFlag,
	) {
		isRunnerCalled = true
		//assert version 1 message unmashal
		//TODO update testify package and assert this block: https://github.com/stretchr/testify/issues/317
		//assert.Equal(t, testCase.docState.InstancePluginsInformation, plugins)
		//return control to stop the backend
		close(resChan)
	}
	stopChan := make(chan int, 1)
	backend := WorkerBackend{
		ctx:        contextMock,
		input:      inputChan,
		cancelFlag: cancelFlag,
		runner:     pluginRunner,
		stopChan:   stopChan,
	}
	backend.Process(testPluginsRawJSON)
	backend.Process(testCancelRawJSON)
	//assert soft stop
	assert.Equal(t, stopTypeShutdown, <-stopChan)
	//assert plugin runner called
	assert.True(t, isRunnerCalled)
	//assert cancel flag set
	cancelFlag.AssertExpectations(t)

}

func TestWorkerBackendPluginListener(t *testing.T) {
	testCase := CreateTestCase()
	statusChan := make(chan contracts.PluginResult)
	inputChan := make(chan string)
	stopChan := make(chan int)
	backend := WorkerBackend{
		ctx:      contextMock,
		input:    inputChan,
		stopChan: stopChan,
	}
	go backend.pluginListener(statusChan)
	statusChan <- *testCase.results["plugin1"]
	data := <-inputChan
	//cannot assume string equal, unmarshal sometimes switch map's order
	assert.Equal(t, len(testPluginReplyRawJSON), len(data))
	statusChan <- *testCase.results["plugin2"]
	data = <-inputChan
	//cannot assume string equal, unmarshal sometimes switch map's order
	assert.Equal(t, len(testPluginReply2RawJSON), len(data))
	close(statusChan)
	data = <-inputChan
	logger.Info(data)
	assert.Equal(t, len(testDocumentCompleteRawJSON), len(data))
	assert.Equal(t, stopTypeShutdown, <-stopChan)
	//make sure input channel is closed
	_, more := <-inputChan
	assert.False(t, more)

}

// this is needed, since after marshal-unmarshalling thru the data channel, the pointer value changed
func assertValueEqual(t *testing.T, a map[string]*contracts.PluginResult, b map[string]*contracts.PluginResult) {
	assert.Equal(t, len(a), len(b))
	for key, val := range a {
		assert.Equal(t, *val, *b[key])
	}
}

// --- reboot-interrupted patch failures -------------------------------------------------------

// stubShutdownChecker pins the shutdown predicate so a gate test does not depend on the
// process-global latch. The latch itself, and the wiring from it to this predicate, are covered by
// shutdownstate_test.go and TestExecuterBackend_Process_WithholdViaRealShutdownSignal.
func stubShutdownChecker(t *testing.T, shuttingDown bool) {
	t.Helper()
	original := shutdownChecker
	shutdownChecker = func(agentlog.T) bool { return shuttingDown }
	t.Cleanup(func() { shutdownChecker = original })
}

// newPatchBackend builds a backend for a patch document whose only plugin has already failed, which
// is the state the worker leaves behind when a shutdown cuts the step short.
func newPatchBackend(t *testing.T) (*ExecuterBackend, chan contracts.DocumentResult, chan int) {
	t.Helper()
	docState := contracts.DocumentState{
		DocumentInformation: contracts.DocumentInfo{
			DocumentID:      testDocumentID,
			DocumentName:    "AWS-RunPatchBaseline",
			MessageID:       testMessageID,
			AssociationID:   testAssociationID,
			DocumentVersion: testDocumentVersion,
			DocumentStatus:  contracts.ResultStatusInProgress,
		},
		DocumentType: "SendCommand",
		InstancePluginsInformation: []contracts.PluginState{{
			Name: "aws:runPatchBaseline",
			Id:   "patch",
			Result: contracts.PluginResult{
				PluginID: "patch",
				Status:   contracts.ResultStatusFailed,
				Code:     1,
				Error:    "interrupted",
			},
		}},
	}
	outputChan := make(chan contracts.DocumentResult, 10)
	stopChan := make(chan int, 1)
	return &ExecuterBackend{
		cancelFlag: taskmocks.NewMockDefault(),
		output:     outputChan,
		stopChan:   stopChan,
		docState:   &docState,
		log:        logger,
	}, outputChan, stopChan
}

// completeDatagram builds a terminal datagram carrying the given document status.
func completeDatagram(t *testing.T, status contracts.ResultStatus) string {
	t.Helper()
	result := contracts.PluginResult{PluginID: "patch", Status: status, Code: 1}
	datagram, err := CreateDatagram(MessageTypeComplete, contracts.DocumentResult{
		Status:        status,
		PluginResults: map[string]*contracts.PluginResult{"patch": &result},
		LastPlugin:    "",
	})
	assert.NoError(t, err)
	return datagram
}

// The failure must not reach the reply channel. That send is the irreversible step -- once the
// service has the Failed, resuming the document cannot take it back -- so asserting on the channel
// is what actually pins the behavior, not the docState rewind.
func TestExecuterBackend_Process_WithholdsRebootInterruptedPatchFailure(t *testing.T) {
	stubShutdownChecker(t, true)
	backend, outputChan, stopChan := newPatchBackend(t)

	assert.NoError(t, backend.Process(completeDatagram(t, contracts.ResultStatusFailed)))

	assert.Empty(t, outputChan, "a reboot-interrupted patch failure must not be reported upstream")
	assert.Equal(t, contracts.ResultStatusInProgress, backend.docState.DocumentInformation.DocumentStatus)
	// Messaging still has to be torn down, and by the same signal a reported result uses, so the
	// Executer sees a clean exit rather than an error it would turn back into a failure.
	assert.Equal(t, stopTypeTerminate, <-stopChan)
}

// The plugin result is what makes the resume real: runpluginutil replays an already-Failed plugin
// instead of re-running it, so a withhold that left the result Failed would re-report on the very
// next pass.
func TestExecuterBackend_Process_WithheldFailureLeavesPluginResumable(t *testing.T) {
	stubShutdownChecker(t, true)
	backend, _, stopChan := newPatchBackend(t)

	assert.NoError(t, backend.Process(completeDatagram(t, contracts.ResultStatusFailed)))
	<-stopChan

	result := backend.docState.InstancePluginsInformation[0].Result
	assert.Equal(t, contracts.ResultStatusInProgress, result.Status, "the resume must re-run the step")
	assert.Equal(t, 0, result.Code)
	assert.Equal(t, "", result.Error)
}

// Exercises the real latch rather than the stub, so the wiring from the preshutdown notification
// through to the gate is covered end to end.
func TestExecuterBackend_Process_WithholdViaRealShutdownSignal(t *testing.T) {
	resetShutdownState(t)
	backend, outputChan, stopChan := newPatchBackend(t)

	PrepareForShutdown(logger)
	assert.NoError(t, backend.Process(completeDatagram(t, contracts.ResultStatusFailed)))

	assert.Empty(t, outputChan)
	assert.Equal(t, stopTypeTerminate, <-stopChan)
}

// Without a shutdown the failure is genuine and must be reported. This is the assertion that fails
// if the gate is ever widened to withhold unconditionally.
func TestExecuterBackend_Process_ReportsPatchFailureWithoutShutdown(t *testing.T) {
	stubShutdownChecker(t, false)
	backend, outputChan, stopChan := newPatchBackend(t)

	assert.NoError(t, backend.Process(completeDatagram(t, contracts.ResultStatusFailed)))

	res := <-outputChan
	assert.Equal(t, contracts.ResultStatusFailed, res.Status)
	assert.Equal(t, contracts.ResultStatusFailed, backend.docState.DocumentInformation.DocumentStatus)
	assert.Equal(t, stopTypeTerminate, <-stopChan)
}

// Only patch documents get reboot-resumption. Every other document type reports normally, even
// mid-shutdown: they have no resume path, so withholding would just lose the result.
func TestExecuterBackend_Process_ReportsNonPatchFailureDuringShutdown(t *testing.T) {
	stubShutdownChecker(t, true)
	backend, outputChan, stopChan := newPatchBackend(t)
	backend.docState.DocumentInformation.DocumentName = "AWS-RunPowerShellScript"

	assert.NoError(t, backend.Process(completeDatagram(t, contracts.ResultStatusFailed)))

	res := <-outputChan
	assert.Equal(t, contracts.ResultStatusFailed, res.Status)
	assert.Equal(t, stopTypeTerminate, <-stopChan)
}

// A patch document that finished during the shutdown window still reports: the gate is about a
// failure the reboot caused, not about suppressing results while a shutdown is pending.
func TestExecuterBackend_Process_ReportsSuccessDuringShutdown(t *testing.T) {
	stubShutdownChecker(t, true)
	backend, outputChan, stopChan := newPatchBackend(t)

	assert.NoError(t, backend.Process(completeDatagram(t, contracts.ResultStatusSuccess)))

	res := <-outputChan
	assert.Equal(t, contracts.ResultStatusSuccess, res.Status)
	assert.Equal(t, stopTypeTerminate, <-stopChan)
}

// Failed is the only plugin status to rewind. TimedOut means the step outlived its own timeout and
// Cancelled means someone stopped it deliberately -- re-running either would override a decision the
// reboot did not make, and the resume skips both anyway.
func TestRewindToInProgress_OnlyRewritesFailedPlugins(t *testing.T) {
	docState := &contracts.DocumentState{
		DocumentInformation: contracts.DocumentInfo{DocumentStatus: contracts.ResultStatusFailed},
		InstancePluginsInformation: []contracts.PluginState{
			{Id: "failed", Result: contracts.PluginResult{Status: contracts.ResultStatusFailed, Code: 1, Error: "boom"}},
			{Id: "timedout", Result: contracts.PluginResult{Status: contracts.ResultStatusTimedOut, Code: 2}},
			{Id: "cancelled", Result: contracts.PluginResult{Status: contracts.ResultStatusCancelled}},
			{Id: "succeeded", Result: contracts.PluginResult{Status: contracts.ResultStatusSuccess}},
		},
	}

	rewindToInProgress(docState)

	assert.Equal(t, contracts.ResultStatusInProgress, docState.DocumentInformation.DocumentStatus)
	assert.Equal(t, contracts.ResultStatusInProgress, docState.InstancePluginsInformation[0].Result.Status)
	assert.Equal(t, 0, docState.InstancePluginsInformation[0].Result.Code)
	assert.Equal(t, "", docState.InstancePluginsInformation[0].Result.Error)
	assert.Equal(t, contracts.ResultStatusTimedOut, docState.InstancePluginsInformation[1].Result.Status)
	assert.Equal(t, 2, docState.InstancePluginsInformation[1].Result.Code)
	assert.Equal(t, contracts.ResultStatusCancelled, docState.InstancePluginsInformation[2].Result.Status)
	assert.Equal(t, contracts.ResultStatusSuccess, docState.InstancePluginsInformation[3].Result.Status)
}

// Only the step the reboot cut short is rewound. An earlier Failed step reported its result upstream
// when it finished, so re-running it would redo work the service has already published -- which is
// reachable on AWS-RunPatchBaselineWithHooks, where several steps run on the same platform.
func TestRewindToInProgress_RewindsOnlyTheLatestFailedPlugin(t *testing.T) {
	docState := &contracts.DocumentState{
		DocumentInformation: contracts.DocumentInfo{DocumentStatus: contracts.ResultStatusFailed},
		InstancePluginsInformation: []contracts.PluginState{
			{Id: "earlierFailure", Result: contracts.PluginResult{Status: contracts.ResultStatusFailed, Code: 1, Error: "hook failed"}},
			{Id: "succeeded", Result: contracts.PluginResult{Status: contracts.ResultStatusSuccess}},
			{Id: "cutShortByReboot", Result: contracts.PluginResult{Status: contracts.ResultStatusFailed, Code: 1, Error: "interrupted"}},
		},
	}

	rewindToInProgress(docState)

	assert.Equal(t, contracts.ResultStatusInProgress, docState.InstancePluginsInformation[2].Result.Status,
		"the step the reboot interrupted must be re-run")
	assert.Equal(t, 0, docState.InstancePluginsInformation[2].Result.Code)
	assert.Equal(t, "", docState.InstancePluginsInformation[2].Result.Error)
	assert.Equal(t, contracts.ResultStatusFailed, docState.InstancePluginsInformation[0].Result.Status,
		"an earlier failure was already reported upstream and must not be re-run")
	assert.Equal(t, 1, docState.InstancePluginsInformation[0].Result.Code)
	assert.Equal(t, "hook failed", docState.InstancePluginsInformation[0].Result.Error)
	assert.Equal(t, contracts.ResultStatusSuccess, docState.InstancePluginsInformation[1].Result.Status)
}

// The Executer's error path calls this when the shutdown killed the worker before it could report
// anything, which is the other place a false failure is manufactured.
func TestRewindPatchDocumentForReboot(t *testing.T) {
	tests := []struct {
		name         string
		documentName string
		shuttingDown bool
		expected     bool
	}{
		{"patch document during shutdown", "AWS-RunPatchBaseline", true, true},
		{"patch document with no shutdown", "AWS-RunPatchBaseline", false, false},
		{"non-patch document during shutdown", "AWS-RunPowerShellScript", true, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stubShutdownChecker(t, test.shuttingDown)
			docState := &contracts.DocumentState{
				DocumentInformation: contracts.DocumentInfo{
					DocumentID:     testDocumentID,
					DocumentName:   test.documentName,
					DocumentStatus: contracts.ResultStatusInProgress,
				},
				InstancePluginsInformation: []contracts.PluginState{
					{Id: "patch", Result: contracts.PluginResult{Status: contracts.ResultStatusFailed, Code: 1}},
				},
			}

			assert.Equal(t, test.expected, RewindPatchDocumentForReboot(logger, docState))

			if test.expected {
				assert.Equal(t, contracts.ResultStatusInProgress, docState.InstancePluginsInformation[0].Result.Status)
			} else {
				assert.Equal(t, contracts.ResultStatusFailed, docState.InstancePluginsInformation[0].Result.Status,
					"a declined gate must leave the document alone")
			}
		})
	}
}
