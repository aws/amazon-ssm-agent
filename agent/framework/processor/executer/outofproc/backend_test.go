package outofproc

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"

	executermocks "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/mock"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
)

var contextMock = context.NewMockDefault()

func CreatePluginConfigs() []model.PluginState {
	pluginState := model.PluginState{
		Name: "aws:runScript",
		Id:   "aws:runScript",
	}
	return []model.PluginState{pluginState}
}

func TestExecuterBackend_Close(t *testing.T) {
	docStoreMock := new(executermocks.MockDocumentStore)
	docState := model.DocumentState{}
	docStoreMock.On("Save", docState).Return(nil)
	outputChan := make(chan contracts.DocumentResult)
	backend := ExecuterBackend{
		docState: &docState,
		docStore: docStoreMock,
		output:   outputChan,
	}
	backend.Close()
	docStoreMock.AssertCalled(t, "Save", docState)
	_, more := <-outputChan
	assert.False(t, more)
}

//test the datagram mashalling v1
func TestExecuterBackend_ProcessV1(t *testing.T) {
	testCase := CreateTestCase()
	testPlugin := "aws:runScript"
	replyData := "{\"version\":\"1.0\",\"type\":\"reply\",\"content\":\"{\\\"DocumentName\\\":\\\"\\\",\\\"DocumentVersion\\\":\\\"\\\",\\\"MessageID\\\":\\\"\\\",\\\"AssociationID\\\":\\\"\\\",\\\"PluginResults\\\":{\\\"aws:runScript\\\":{\\\"pluginName\\\":\\\"aws:runScript\\\",\\\"status\\\":\\\"Success\\\",\\\"code\\\":0,\\\"output\\\":null,\\\"startDateTime\\\":\\\"2017-08-13T00:00:00Z\\\",\\\"endDateTime\\\":\\\"2017-08-13T00:00:01Z\\\",\\\"outputS3BucketName\\\":\\\"\\\",\\\"outputS3KeyPrefix\\\":\\\"\\\",\\\"standardOutput\\\":\\\"\\\",\\\"standardError\\\":\\\"\\\"}},\\\"Status\\\":\\\"InProgress\\\",\\\"LastPlugin\\\":\\\"aws:runScript\\\",\\\"NPlugins\\\":0}\"}"
	completeData := "{\"version\":\"1.0\",\"type\":\"complete\",\"content\":\"{\\\"DocumentName\\\":\\\"\\\",\\\"DocumentVersion\\\":\\\"\\\",\\\"MessageID\\\":\\\"\\\",\\\"AssociationID\\\":\\\"\\\",\\\"PluginResults\\\":{\\\"aws:runScript\\\":{\\\"pluginName\\\":\\\"aws:runScript\\\",\\\"status\\\":\\\"Success\\\",\\\"code\\\":0,\\\"output\\\":null,\\\"startDateTime\\\":\\\"2017-08-13T00:00:00Z\\\",\\\"endDateTime\\\":\\\"2017-08-13T00:00:01Z\\\",\\\"outputS3BucketName\\\":\\\"\\\",\\\"outputS3KeyPrefix\\\":\\\"\\\",\\\"standardOutput\\\":\\\"\\\",\\\"standardError\\\":\\\"\\\"}},\\\"Status\\\":\\\"Success\\\",\\\"LastPlugin\\\":\\\"\\\",\\\"NPlugins\\\":0}\"}"
	outputChan := make(chan contracts.DocumentResult, 10)
	stopChan := make(chan int, 1)
	backend := ExecuterBackend{
		output:   outputChan,
		stopChan: stopChan,
		docState: &testCase.docState,
	}
	err := backend.Process(replyData)
	assert.NoError(t, err)
	res := <-outputChan
	assert.Equal(t, testPlugin, res.LastPlugin)
	assertValueEqual(t, testCase.results, res.PluginResults)
	assert.Equal(t, contracts.ResultStatusInProgress, res.Status)
	assert.Equal(t, testDocumentName, res.DocumentName)
	assert.Equal(t, testAssociationID, res.AssociationID)
	assert.Equal(t, testMessageID, res.MessageID)
	assert.Equal(t, len(testCase.docState.InstancePluginsInformation), res.NPlugins)
	assert.Equal(t, testDocumentVersion, res.DocumentVersion)
	err = backend.Process(completeData)
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
	backend := WorkerBackend{
		cancelFlag: cancelFlag,
	}
	backend.Close()
	cancelFlag.AssertCalled(t, "Set", task.Completed)
}
