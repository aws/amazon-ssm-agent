package messaging

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"

	"time"

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
var testDocumentCompleteRawJSON string

type TestCase struct {
	context      *context.Mock
	docState     model.DocumentState
	results      map[string]*contracts.PluginResult
	resultStatus contracts.ResultStatus
}

func CreateTestCase() *TestCase {
	contextMock := context.NewMockDefaultWithContext([]string{"MASTER"})
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
	docState := model.DocumentState{
		DocumentInformation:        docInfo,
		DocumentType:               "SendCommand",
		InstancePluginsInformation: []model.PluginState{pluginState},
	}

	result := contracts.PluginResult{
		PluginName:    "aws:runScript",
		PluginID:      "plugin1",
		Status:        contracts.ResultStatusSuccess,
		StartDateTime: testStartDateTime,
		EndDateTime:   testEndDateTime,
	}
	results := make(map[string]*contracts.PluginResult)
	results[pluginState.Id] = &result
	//corresponding rawJSON data
	//TODO this is V2 Schema, add V1 schema later
	testPluginReplyRawJSON = "{\"version\":\"1.0\",\"type\":\"reply\",\"content\":\"{\\\"DocumentName\\\":\\\"\\\",\\\"DocumentVersion\\\":\\\"\\\",\\\"MessageID\\\":\\\"\\\",\\\"AssociationID\\\":\\\"\\\",\\\"PluginResults\\\":{\\\"plugin1\\\":{\\\"pluginName\\\":\\\"aws:runScript\\\",\\\"pluginID\\\":\\\"plugin1\\\",\\\"status\\\":\\\"Success\\\",\\\"code\\\":0,\\\"output\\\":null,\\\"startDateTime\\\":\\\"2017-08-13T00:00:00Z\\\",\\\"endDateTime\\\":\\\"2017-08-13T00:00:01Z\\\",\\\"outputS3BucketName\\\":\\\"\\\",\\\"outputS3KeyPrefix\\\":\\\"\\\",\\\"standardOutput\\\":\\\"\\\",\\\"standardError\\\":\\\"\\\"}},\\\"Status\\\":\\\"InProgress\\\",\\\"LastPlugin\\\":\\\"plugin1\\\",\\\"NPlugins\\\":0}\"}"
	testDocumentCompleteRawJSON = "{\"version\":\"1.0\",\"type\":\"complete\",\"content\":\"{\\\"DocumentName\\\":\\\"\\\",\\\"DocumentVersion\\\":\\\"\\\",\\\"MessageID\\\":\\\"\\\",\\\"AssociationID\\\":\\\"\\\",\\\"PluginResults\\\":{\\\"plugin1\\\":{\\\"pluginName\\\":\\\"aws:runScript\\\",\\\"pluginID\\\":\\\"plugin1\\\",\\\"status\\\":\\\"Success\\\",\\\"code\\\":0,\\\"output\\\":null,\\\"startDateTime\\\":\\\"2017-08-13T00:00:00Z\\\",\\\"endDateTime\\\":\\\"2017-08-13T00:00:01Z\\\",\\\"outputS3BucketName\\\":\\\"\\\",\\\"outputS3KeyPrefix\\\":\\\"\\\",\\\"standardOutput\\\":\\\"\\\",\\\"standardError\\\":\\\"\\\"}},\\\"Status\\\":\\\"Success\\\",\\\"LastPlugin\\\":\\\"\\\",\\\"NPlugins\\\":0}\"}"

	return &TestCase{
		context:      contextMock,
		docState:     docState,
		results:      results,
		resultStatus: contracts.ResultStatusSuccess,
	}
}

//backend should close the stopChan at close time
func TestExecuterBackend_Close(t *testing.T) {
	backend := ExecuterBackend{
		stopChan: make(chan int),
	}
	backend.Close()
	_, more := <-backend.Stop()
	assert.False(t, more)
}

//test the datagram mashalling v1
func TestExecuterBackend_ProcessV1(t *testing.T) {
	testCase := CreateTestCase()
	outputChan := make(chan contracts.DocumentResult, 10)
	stopChan := make(chan int, 1)
	backend := ExecuterBackend{
		output:   outputChan,
		stopChan: stopChan,
		docState: &testCase.docState,
	}
	err := backend.Process(testPluginReplyRawJSON)
	assert.NoError(t, err)
	res := <-outputChan
	assert.Equal(t, "plugin1", res.LastPlugin)
	assertValueEqual(t, testCase.results, res.PluginResults)
	assert.Equal(t, contracts.ResultStatusInProgress, res.Status)
	assert.Equal(t, testDocumentName, res.DocumentName)
	assert.Equal(t, testAssociationID, res.AssociationID)
	assert.Equal(t, testMessageID, res.MessageID)
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
}

func TestWorkerBackend_ProcessV1(t *testing.T) {
	//testCase := CreateTestCase()
	pluginsData := "{\"version\":\"1.0\",\"type\":\"pluginconfig\",\"content\":\"[{\\\"Configuration\\\":{\\\"Settings\\\":null,\\\"Properties\\\":null,\\\"OutputS3KeyPrefix\\\":\\\"\\\",\\\"OutputS3BucketName\\\":\\\"\\\",\\\"OrchestrationDirectory\\\":\\\"\\\",\\\"MessageId\\\":\\\"\\\",\\\"BookKeepingFileName\\\":\\\"\\\",\\\"PluginName\\\":\\\"\\\",\\\"PluginID\\\":\\\"\\\",\\\"DefaultWorkingDirectory\\\":\\\"\\\",\\\"Preconditions\\\":null,\\\"IsPreconditionEnabled\\\":false},\\\"Name\\\":\\\"aws:runScript\\\",\\\"Result\\\":{\\\"pluginName\\\":\\\"\\\",\\\"status\\\":\\\"\\\",\\\"code\\\":0,\\\"output\\\":null,\\\"startDateTime\\\":\\\"0001-01-01T00:00:00Z\\\",\\\"endDateTime\\\":\\\"0001-01-01T00:00:00Z\\\",\\\"outputS3BucketName\\\":\\\"\\\",\\\"outputS3KeyPrefix\\\":\\\"\\\",\\\"standardOutput\\\":\\\"\\\",\\\"standardError\\\":\\\"\\\"},\\\"Id\\\":\\\"aws:runScript\\\"}]\"}"
	cancelData := "{\"version\":\"1.0\",\"type\":\"cancel\",\"content\":\"\"}"
	inputChan := make(chan string, 10)
	cancelFlag := new(task.MockCancelFlag)
	cancelFlag.On("Set", task.Canceled).Return(nil)
	isRunnerCalled := false
	pluginRunner := func(
		context context.T,
		plugins []model.PluginState,
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
	backend.Process(pluginsData)
	backend.Process(cancelData)
	//assert messaging worker stopped
	assert.Equal(t, stopTypeTerminate, <-stopChan)
	//assert plugin runner called
	assert.True(t, isRunnerCalled)
	//assert cancel flag set
	cancelFlag.AssertExpectations(t)

}

func TestWorkerBackend_Close(t *testing.T) {
	cancelFlag := new(task.MockCancelFlag)
	cancelFlag.On("Set", task.Completed).Return(nil)
	backend := WorkerBackend{
		cancelFlag: cancelFlag,
	}
	backend.Close()
	cancelFlag.AssertCalled(t, "Set", task.Completed)
}

//this is needed, since after marshal-unmarshalling thru the data channel, the pointer value changed
func assertValueEqual(t *testing.T, a map[string]*contracts.PluginResult, b map[string]*contracts.PluginResult) {
	assert.Equal(t, len(a), len(b))
	for key, val := range a {
		assert.Equal(t, *val, *b[key])
	}
}
