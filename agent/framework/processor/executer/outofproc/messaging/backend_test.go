package messaging

import (
	"testing"

	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"

	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
)

var contextMock = context.NewMockDefault()
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
	context      *context.Mock
	docState     contracts.DocumentState
	results      map[string]*contracts.PluginResult
	resultStatus contracts.ResultStatus
}

func CreateTestCase() *TestCase {
	contextMock := context.NewMockDefaultWithContext([]string{"MASTER"})
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
	testPluginReplyRawJSON = "{\"version\":\"1.0\",\"type\":\"reply\",\"content\":\"{\\\"DocumentName\\\":\\\"\\\",\\\"DocumentVersion\\\":\\\"\\\",\\\"MessageID\\\":\\\"\\\",\\\"AssociationID\\\":\\\"\\\",\\\"PluginResults\\\":{\\\"plugin1\\\":{\\\"pluginName\\\":\\\"aws:runScript\\\",\\\"pluginID\\\":\\\"plugin1\\\",\\\"status\\\":\\\"Success\\\",\\\"code\\\":0,\\\"output\\\":null,\\\"startDateTime\\\":\\\"2017-08-13T00:00:00Z\\\",\\\"endDateTime\\\":\\\"2017-08-13T00:00:01Z\\\",\\\"outputS3BucketName\\\":\\\"\\\",\\\"outputS3KeyPrefix\\\":\\\"\\\",\\\"stepName\\\":\\\"\\\",\\\"error\\\":\\\"error occurred\\\",\\\"standardOutput\\\":\\\"\\\",\\\"standardError\\\":\\\"\\\"}},\\\"Status\\\":\\\"InProgress\\\",\\\"LastPlugin\\\":\\\"plugin1\\\",\\\"NPlugins\\\":0}\"}"
	testPluginReply2RawJSON = "{\"version\":\"1.0\",\"type\":\"reply\",\"content\":\"{\\\"DocumentName\\\":\\\"\\\",\\\"DocumentVersion\\\":\\\"\\\",\\\"MessageID\\\":\\\"\\\",\\\"AssociationID\\\":\\\"\\\",\\\"PluginResults\\\":{\\\"plugin1\\\":{\\\"pluginID\\\":\\\"plugin1\\\",\\\"pluginName\\\":\\\"aws:runScript\\\",\\\"status\\\":\\\"Success\\\",\\\"code\\\":0,\\\"output\\\":null,\\\"startDateTime\\\":\\\"2017-08-13T00:00:00Z\\\",\\\"endDateTime\\\":\\\"2017-08-13T00:00:01Z\\\",\\\"outputS3BucketName\\\":\\\"\\\",\\\"outputS3KeyPrefix\\\":\\\"\\\",\\\"stepName\\\":\\\"\\\",\\\"error\\\":\\\"error occurred\\\",\\\"standardOutput\\\":\\\"\\\",\\\"standardError\\\":\\\"\\\"},\\\"plugin2\\\":{\\\"pluginID\\\":\\\"plugin2\\\",\\\"pluginName\\\":\\\"aws:runPowershellScript\\\",\\\"status\\\":\\\"Success\\\",\\\"code\\\":0,\\\"output\\\":null,\\\"startDateTime\\\":\\\"2017-08-13T00:00:00Z\\\",\\\"endDateTime\\\":\\\"2017-08-13T00:00:01Z\\\",\\\"outputS3BucketName\\\":\\\"\\\",\\\"outputS3KeyPrefix\\\":\\\"\\\",\\\"stepName\\\":\\\"\\\",\\\"error\\\":\\\"\\\",\\\"standardOutput\\\":\\\"\\\",\\\"standardError\\\":\\\"\\\"}},\\\"Status\\\":\\\"InProgress\\\",\\\"LastPlugin\\\":\\\"plugin2\\\",\\\"NPlugins\\\":0}\"}"
	testDocumentCompleteRawJSON = "{\"version\":\"1.0\",\"type\":\"complete\",\"content\":\"{\\\"DocumentName\\\":\\\"\\\",\\\"DocumentVersion\\\":\\\"\\\",\\\"MessageID\\\":\\\"\\\",\\\"AssociationID\\\":\\\"\\\",\\\"PluginResults\\\":{\\\"plugin1\\\":{\\\"pluginID\\\":\\\"plugin1\\\",\\\"pluginName\\\":\\\"aws:runScript\\\",\\\"status\\\":\\\"Success\\\",\\\"code\\\":0,\\\"output\\\":null,\\\"startDateTime\\\":\\\"2017-08-13T00:00:00Z\\\",\\\"endDateTime\\\":\\\"2017-08-13T00:00:01Z\\\",\\\"outputS3BucketName\\\":\\\"\\\",\\\"outputS3KeyPrefix\\\":\\\"\\\",\\\"stepName\\\":\\\"\\\",\\\"error\\\":\\\"error occurred\\\",\\\"standardOutput\\\":\\\"\\\",\\\"standardError\\\":\\\"\\\"},\\\"plugin2\\\":{\\\"pluginID\\\":\\\"plugin2\\\",\\\"pluginName\\\":\\\"aws:runPowershellScript\\\",\\\"status\\\":\\\"Success\\\",\\\"code\\\":0,\\\"output\\\":null,\\\"startDateTime\\\":\\\"2017-08-13T00:00:00Z\\\",\\\"endDateTime\\\":\\\"2017-08-13T00:00:01Z\\\",\\\"outputS3BucketName\\\":\\\"\\\",\\\"outputS3KeyPrefix\\\":\\\"\\\",\\\"stepName\\\":\\\"\\\",\\\"error\\\":\\\"\\\",\\\"standardOutput\\\":\\\"\\\",\\\"standardError\\\":\\\"\\\"}},\\\"Status\\\":\\\"Success\\\",\\\"LastPlugin\\\":\\\"\\\",\\\"NPlugins\\\":0}\"}"
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

//TODO test cancel message
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

//test the datagram mashalling v1
func TestExecuterBackend_ProcessV1(t *testing.T) {
	testCase := CreateTestCase()
	outputChan := make(chan contracts.DocumentResult, 10)
	stopChan := make(chan int, 1)
	cancel := task.NewMockDefault()
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

//test the datagram mashalling v1
func TestExecuterBackend_ProcessUnsupportedVersion(t *testing.T) {
	testCase := CreateTestCase()
	outputChan := make(chan contracts.DocumentResult, 10)
	stopChan := make(chan int, 1)
	cancel := task.NewMockDefault()
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

func TestWorkerBackend_ProcessCancelV1(t *testing.T) {
	_ = CreateTestCase()
	inputChan := make(chan string, 10)
	cancelFlag := new(task.MockCancelFlag)
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

//this is needed, since after marshal-unmarshalling thru the data channel, the pointer value changed
func assertValueEqual(t *testing.T, a map[string]*contracts.PluginResult, b map[string]*contracts.PluginResult) {
	assert.Equal(t, len(a), len(b))
	for key, val := range a {
		assert.Equal(t, *val, *b[key])
	}
}
