package mgsinteractor

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	replytypesmock "github.com/aws/amazon-ssm-agent/agent/messageservice/interactor/mgsinteractor/replytypes/mocks"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/messagehandler/mocks"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	controlChannelMock "github.com/aws/amazon-ssm-agent/agent/session/controlchannel/mocks"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/twinj/uuid"
)

type SendReplyTestSuite struct {
	suite.Suite
}

//Execute the test suite
func TestSendReplyTestSuite(t *testing.T) {
	suite.Run(t, new(SendReplyTestSuite))
}

func (suite *SendReplyTestSuite) TestSendReplyToMGS() {
	mockControlChannel := &controlChannelMock.IControlChannel{}
	mockControlChannel.On("SendMessage", mock.Anything, mock.Anything, websocket.BinaryMessage).Return(fmt.Errorf("sample error"))

	mockContext := context.NewMockDefault()
	messageHandlerMock := &mocks.IMessageHandler{}
	messageHandlerMock.On("RegisterReply", mock.Anything, mock.Anything)
	messageHandlerMock.On("GetMessageUUID", mock.Anything, mock.Anything)
	mgsInteractorRef, err := New(mockContext, messageHandlerMock)
	assert.Nil(suite.T(), err, "initialize passed")
	mgsInteractor := mgsInteractorRef.(*MGSInteractor)
	mgsInteractor.controlChannel = mockControlChannel

	msg := mgsContracts.AcknowledgeTaskContent{
		MessageId: uuid.NewV4().String(), // generate random one
		Topic:     mgsContracts.TaskCompleteMessage,
	}
	ackByte, err := json.Marshal(msg)
	assert.Nil(suite.T(), err)
	agentMessage := mgsContracts.AgentMessage{
		MessageId:   uuid.NewV4(),
		Payload:     ackByte,
		MessageType: mgsContracts.TaskAcknowledgeMessage,
	}
	replyTypeMock := &replytypesmock.IReplyType{}
	replyTypeMock.On("ConvertToAgentMessage").Return(&agentMessage, nil)
	replyTypeMock.On("IncrementRetries").Return(1)
	replyTypeMock.On("GetNumberOfContinuousRetries").Return(4)
	replyTypeMock.On("GetMessageUUID").Return(uuid.NewV4())
	replyTypeMock.On("GetResult").Return(contracts.DocumentResult{})
	replyTypeMock.On("GetRetryNumber").Return(1)

	err = mgsInteractor.sendReplyToMGS(replyTypeMock)
	assert.Contains(suite.T(), err.Error(), "sample error")
	mockControlChannel.AssertNumberOfCalls(suite.T(), "SendMessage", 1)
}

func (suite *SendReplyTestSuite) TestTaskAgentCompleteWithRetry() {
	mockControlChannel := &controlChannelMock.IControlChannel{}
	mockControlChannel.On("SendMessage", mock.Anything, mock.Anything, websocket.BinaryMessage).Return(fmt.Errorf("sample error"))

	mockContext := context.NewMockDefault()
	messageHandlerMock := &mocks.IMessageHandler{}
	messageHandlerMock.On("RegisterReply", mock.Anything, mock.Anything)
	messageHandlerMock.On("GetMessageUUID", mock.Anything, mock.Anything)

	mgsInteractorRef, err := New(mockContext, messageHandlerMock)
	assert.Nil(suite.T(), err, "initialize passed")
	mgsInteractor := mgsInteractorRef.(*MGSInteractor)
	mgsInteractor.controlChannel = mockControlChannel

	msg := mgsContracts.AcknowledgeTaskContent{
		MessageId: uuid.NewV4().String(), // generate random one
		Topic:     mgsContracts.TaskCompleteMessage,
	}
	ackByte, err := json.Marshal(msg)
	assert.Nil(suite.T(), err)
	agentMessage := mgsContracts.AgentMessage{
		MessageId:   uuid.NewV4(),
		Payload:     ackByte,
		MessageType: mgsContracts.TaskAcknowledgeMessage,
	}

	replyTypeMock := &replytypesmock.IReplyType{}
	replyTypeMock.On("ConvertToAgentMessage").Return(&agentMessage, nil)
	replyTypeMock.On("IncrementRetries").Return(1)
	replyTypeMock.On("GetNumberOfContinuousRetries").Return(4)
	replyTypeMock.On("GetMessageUUID").Return(uuid.NewV4())
	replyTypeMock.On("ShouldPersistData").Return(false)
	replyTypeMock.On("GetBackOffSecond").Return(0)
	replyTypeMock.On("GetResult").Return(contracts.DocumentResult{})
	replyTypeMock.On("GetRetryNumber").Return(1)
	reply := &agentReplyLocalContract{
		documentResult: replyTypeMock,
		backupFile:     "",
		retryNumber:    0,
	}
	mgsInteractor.processReply(reply)
	replyTypeMock.AssertNumberOfCalls(suite.T(), "ShouldPersistData", 4)
}

func (suite *SendReplyTestSuite) TestTaskAgentCompleteWithSecondRetryAckReceive() {
	mockControlChannel := &controlChannelMock.IControlChannel{}
	mockControlChannel.On("SendMessage", mock.Anything, mock.Anything, websocket.BinaryMessage).Return(fmt.Errorf("sample error"))

	mockContext := context.NewMockDefault()
	messageHandlerMock := &mocks.IMessageHandler{}
	messageHandlerMock.On("RegisterReply", mock.Anything, mock.Anything)
	messageHandlerMock.On("GetMessageUUID", mock.Anything, mock.Anything)
	mgsInteractorRef, err := New(mockContext, messageHandlerMock)
	assert.Nil(suite.T(), err, "initialize passed")
	mgsInteractor := mgsInteractorRef.(*MGSInteractor)
	mgsInteractor.controlChannel = mockControlChannel

	msg := mgsContracts.AcknowledgeTaskContent{
		MessageId: uuid.NewV4().String(), // generate random one
		Topic:     mgsContracts.TaskCompleteMessage,
	}
	ackByte, err := json.Marshal(msg)

	assert.Nil(suite.T(), err)
	agentMessage := mgsContracts.AgentMessage{
		MessageId:   uuid.NewV4(),
		Payload:     ackByte,
		MessageType: mgsContracts.TaskAcknowledgeMessage,
	}
	uuidVal := uuid.NewV4()
	replyTypeMock := &replytypesmock.IReplyType{}
	replyTypeMock.On("ConvertToAgentMessage").Return(&agentMessage, nil)
	replyTypeMock.On("IncrementRetries").Return(1)
	replyTypeMock.On("GetNumberOfContinuousRetries").Return(4)
	replyTypeMock.On("GetMessageUUID").Return(uuidVal)
	replyTypeMock.On("ShouldPersistData").Return(false)
	replyTypeMock.On("GetBackOffSecond").Return(1)
	replyTypeMock.On("GetResult").Return(contracts.DocumentResult{})
	replyTypeMock.On("GetRetryNumber").Return(1)
	reply := &agentReplyLocalContract{
		documentResult: replyTypeMock,
		backupFile:     "",
		retryNumber:    0,
	}
	go func() {
		time.Sleep(1500 * time.Millisecond)
		if ackChan, ok := mgsInteractor.sendReplyProp.replyAckChan.Load(uuidVal.String()); ok {
			ackChan.(chan bool) <- true
		}
	}()
	mgsInteractor.processReply(reply)
	replyTypeMock.AssertNumberOfCalls(suite.T(), "ShouldPersistData", 1)
}

func (suite *SendReplyTestSuite) TestTaskAgentCompleteWithNormalAckReceive() {
	mockControlChannel := &controlChannelMock.IControlChannel{}
	mockControlChannel.On("SendMessage", mock.Anything, mock.Anything, websocket.BinaryMessage).Return(fmt.Errorf("sample error"))
	mockContext := context.NewMockDefault()
	messageHandlerMock := &mocks.IMessageHandler{}
	messageHandlerMock.On("RegisterReply", mock.Anything, mock.Anything)
	messageHandlerMock.On("GetMessageUUID", mock.Anything, mock.Anything)
	mgsInteractorRef, err := New(mockContext, messageHandlerMock)
	assert.Nil(suite.T(), err, "initialize passed")
	mgsInteractor := mgsInteractorRef.(*MGSInteractor)
	mgsInteractor.controlChannel = mockControlChannel

	msg := mgsContracts.AcknowledgeTaskContent{
		MessageId: uuid.NewV4().String(), // generate random one
		Topic:     mgsContracts.TaskCompleteMessage,
	}
	ackByte, err := json.Marshal(msg)

	assert.Nil(suite.T(), err)
	agentMessage := mgsContracts.AgentMessage{
		MessageId:   uuid.NewV4(),
		Payload:     ackByte,
		MessageType: mgsContracts.TaskAcknowledgeMessage,
	}
	uuidVal := uuid.NewV4()
	replyTypeMock := &replytypesmock.IReplyType{}
	replyTypeMock.On("ConvertToAgentMessage").Return(&agentMessage, nil)
	replyTypeMock.On("IncrementRetries").Return(1)
	replyTypeMock.On("GetNumberOfContinuousRetries").Return(4)
	replyTypeMock.On("GetMessageUUID").Return(uuidVal)
	replyTypeMock.On("ShouldPersistData").Return(false)
	replyTypeMock.On("GetBackOffSecond").Return(1)
	replyTypeMock.On("GetResult").Return(contracts.DocumentResult{})
	replyTypeMock.On("GetRetryNumber").Return(1)
	reply := &agentReplyLocalContract{
		documentResult: replyTypeMock,
		backupFile:     "",
		retryNumber:    0,
	}
	// Normal retry
	go func() {
		time.Sleep(500 * time.Millisecond)
		if ackChan, ok := mgsInteractor.sendReplyProp.replyAckChan.Load(uuidVal.String()); ok {
			ackChan.(chan bool) <- true
		}
	}()
	mgsInteractor.processReply(reply)
	replyTypeMock.AssertNumberOfCalls(suite.T(), "ShouldPersistData", 0)
}
