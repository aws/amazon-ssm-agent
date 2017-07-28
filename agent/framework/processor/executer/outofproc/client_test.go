package outofproc

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/channel"
	channelmock "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/channel/mock"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/contracts"

	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var contextMock = context.NewMockDefault()

func CreatePluginConfigs() []model.PluginState {
	pluginState := model.PluginState{
		Name: "aws:runScript",
		Id:   "aws:runScript",
	}
	return []model.PluginState{pluginState}
}
func TestClientRunPluginsV1(t *testing.T) {

	v1 := "1.0"
	channelMock := new(channelmock.MockedChannel)
	channelMock.On("Connect").Return(nil)
	serverChan := make(chan messageContracts.Message)
	channelMock.On("GetMessageChannel").Return(serverChan)
	channelMock.On("Send", mock.Anything).Return(nil)
	channelMock.On("Close").Return(nil).Once()
	expectedChannelName := createChannelHandle(testDocumentID, v1)
	channelCreator = func(handle string, mode channel.Mode) (channel.Channel, error) {
		assert.Equal(t, mode, channel.ModeClient)
		assert.Equal(t, handle, expectedChannelName)
		return channelMock, nil
	}
	runnerCalled := false
	runner := func(
		context context.T,
		plugins []model.PluginState,
		resChan chan contracts.PluginResult,
		cancelFlag task.CancelFlag,
	) {
		resChan <- contracts.PluginResult{
			PluginName: "plugin1",
			Status:     contracts.ResultStatusSuccess,
		}
		close(resChan)
		runnerCalled = true
	}
	//launch a fake server
	go func() {
		msg, err := messageContracts.Marshal(v1, messageContracts.MessageTypeStart, CreatePluginConfigs())
		assert.NoError(t, err)
		serverChan <- msg
	}()

	Client(contextMock, expectedChannelName, runner)
	channelMock.AssertExpectations(t)
	//plugin update + document update + close message
	channelMock.AssertNumberOfCalls(t, "Send", 3)
	assert.True(t, runnerCalled)

}

func TestClientCancelRunningDocumentV1(t *testing.T) {

	v1 := "1.0"
	channelMock := new(channelmock.MockedChannel)
	channelMock.On("Connect").Return(nil)
	serverChan := make(chan messageContracts.Message)
	channelMock.On("GetMessageChannel").Return(serverChan)
	channelMock.On("Send", mock.Anything).Return(nil)
	channelMock.On("Close").Return(nil).Once()
	expectedChannelName := createChannelHandle(testDocumentID, v1)
	channelCreator = func(handle string, mode channel.Mode) (channel.Channel, error) {
		assert.Equal(t, mode, channel.ModeClient)
		assert.Equal(t, handle, expectedChannelName)
		return channelMock, nil
	}
	runnerCalled := false
	runner := func(
		context context.T,
		plugins []model.PluginState,
		resChan chan contracts.PluginResult,
		cancelFlag task.CancelFlag,
	) {
		cancelFlag.Wait()
		assert.True(t, cancelFlag.Canceled())
		logger.Info("command cancelled")
		resChan <- contracts.PluginResult{
			PluginName: "plugin1",
			Status:     contracts.ResultStatusCancelled,
		}
		close(resChan)
		runnerCalled = true
	}
	//launch a fake server
	go func() {
		msg, err := messageContracts.Marshal(v1, messageContracts.MessageTypeStart, CreatePluginConfigs())
		assert.NoError(t, err)
		serverChan <- msg
		msg, err = messageContracts.Marshal(v1, messageContracts.MessageTypeControl, DocumentControl{Type: controlTypeCancelled})
		assert.NoError(t, err)
		serverChan <- msg
	}()

	Client(contextMock, expectedChannelName, runner)
	channelMock.AssertExpectations(t)
	//plugin cancelled + document Cancelled + close
	channelMock.AssertNumberOfCalls(t, "Send", 3)
	assert.True(t, runnerCalled)

}
