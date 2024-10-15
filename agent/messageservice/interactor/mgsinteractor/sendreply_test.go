package mgsinteractor

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	replytypesmock "github.com/aws/amazon-ssm-agent/agent/messageservice/interactor/mgsinteractor/replytypes/mocks"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/messagehandler/mocks"
	"github.com/aws/amazon-ssm-agent/agent/mocks/context"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	controlChannelMock "github.com/aws/amazon-ssm-agent/agent/session/controlchannel/mocks"
	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/twinj/uuid"
)

type SendReplyTestSuite struct {
	suite.Suite
}

// Execute the test suite
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

func (suite *SendReplyTestSuite) TestProcessReply_checkForWarningErrors_SkipRetry() {
	sendMessageErr := fmt.Errorf("ws not initialized still")
	mgsInteractor := suite.getMGSInteractorRef(sendMessageErr)

	msg := mgsContracts.AcknowledgeTaskContent{
		MessageId: uuid.NewV4().String(), // generate random one
		Topic:     mgsContracts.TaskCompleteMessage,
	}
	ackByte, err := json.Marshal(msg)
	assert.Nil(suite.T(), err)

	uuidVal := uuid.NewV4()
	replyTypeMock, reply := suite.getReplyWithRetry(ackByte, uuidVal)

	ackChanPresent := true
	go func() {
		time.Sleep(50 * time.Millisecond)
		if _, ok := mgsInteractor.sendReplyProp.replyAckChan.Load(uuidVal); !ok {
			ackChanPresent = false
		}
	}()

	mgsInteractor.processReply(reply)
	time.Sleep(100 * time.Millisecond)
	assert.False(suite.T(), ackChanPresent)
	replyTypeMock.AssertNumberOfCalls(suite.T(), "IncrementRetries", 1)
}

func (suite *SendReplyTestSuite) getReplyWithRetry(ackByte []byte, uuidVal uuid.UUID) (*replytypesmock.IReplyType, *agentReplyLocalContract) {
	agentMessage := mgsContracts.AgentMessage{
		MessageId:   uuid.NewV4(),
		Payload:     ackByte,
		MessageType: mgsContracts.TaskAcknowledgeMessage,
	}
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
	return replyTypeMock, reply
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

func (suite *SendReplyTestSuite) TestPersistResult_FileNotPresentAlready_SuccessfulSave() {
	reply := suite.getDocumentResultObject()
	replyId := reply.ReplyId
	mgsInteractor := suite.getMGSInteractorRef(nil)
	writeFileCheck := false
	writeIntoFile = func(absolutePath, content string, perm os.FileMode) (result bool, err error) {
		if !strings.HasSuffix(absolutePath, replyId) {
			return false, nil
		}
		if val, err := jsonutil.Marshal(reply); err != nil || jsonutil.Indent(val) != content {
			return false, nil
		}
		if perm != os.FileMode(appconfig.ReadWriteAccess) {
			return false, nil
		}
		writeFileCheck = true
		return true, nil
	}
	mgsInteractor.persistResult(reply)
	assert.True(suite.T(), writeFileCheck, "reply is saved successfully")
}

func (suite *SendReplyTestSuite) TestPersistResult_FilePresentAlready_SuccessfulSave() {
	reply := suite.getDocumentResultObject()
	replyId := reply.ReplyId
	mgsInteractor := suite.getMGSInteractorRef(nil)
	getFileNames = func(srcPath string) (files []string, err error) {
		fileList := make([]string, 0)
		fileList = append(fileList, replyId)
		return fileList, nil
	}
	writeFileCheck := false
	writeIntoFile = func(absolutePath, content string, perm os.FileMode) (result bool, err error) {
		if !strings.HasSuffix(absolutePath, replyId) {
			return false, nil
		}
		if val, err := jsonutil.Marshal(reply); err != nil || jsonutil.Indent(val) != content {
			return false, nil
		}
		if perm != os.FileMode(appconfig.ReadWriteAccess) {
			return false, nil
		}
		writeFileCheck = true
		return true, nil
	}
	mgsInteractor.persistResult(reply)
	assert.True(suite.T(), writeFileCheck, "reply is saved successfully")
}

func (suite *SendReplyTestSuite) TestFilterReplies_FiltersOutAlreadyHandledUpdateReply() {
	mgs := suite.getMGSInteractorRef(nil)

	unfilteredReplies := []string{"reply", "handled_update"}
	mgs.handledUpdateReplies.Store("handled_update", true)
	replies := mgs.filterReplies(unfilteredReplies)
	assert.True(suite.T(), len(replies) == 1)
	assert.Equal(suite.T(), replies[0], "reply")
}

func (suite *SendReplyTestSuite) TestFilterReplies_DoesNotFilterOutUnhandledUpdateReply() {
	mgs := suite.getMGSInteractorRef(nil)

	unfilteredReplies := []string{"unhandled_update"}
	replies := mgs.filterReplies(unfilteredReplies)
	assert.True(suite.T(), len(replies) == 1)
	assert.Equal(suite.T(), replies[0], "unhandled_update")
	_, handled := mgs.handledUpdateReplies.Load("unhandled_update")
	assert.True(suite.T(), handled)
}

func (suite *SendReplyTestSuite) TestSendUpdateReply_DoesNothingIfMessageAlreadyHandled() {
	mockControlChannel := &controlChannelMock.IControlChannel{}
	mockControlChannel.On("SendMessage", mock.Anything, mock.Anything, websocket.BinaryMessage)
	mockContext := context.NewMockDefault()
	messageHandlerMock := &mocks.IMessageHandler{}
	messageHandlerMock.On("RegisterReply", mock.Anything, mock.Anything)
	messageHandlerMock.On("GetMessageUUID", mock.Anything, mock.Anything)
	mgsInteractorRef, err := New(mockContext, messageHandlerMock)
	assert.Nil(suite.T(), err, "initialize passed")
	mgs := mgsInteractorRef.(*MGSInteractor)
	mgs.controlChannel = mockControlChannel

	updatePath := "updateFile"
	mgs.handledUpdateReplies.Store(updatePath, true)
	assert.Nil(suite.T(), mgs.sendUpdateReply(updatePath))
	mockControlChannel.AssertNumberOfCalls(suite.T(), "SendMessage", 0)
}

func (suite *SendReplyTestSuite) TestSendUpdateReply_WritesTrueToHandledUpdateRepliesIfMessageNotYetHandled() {
	mgs := suite.getMGSInteractorRef(nil)

	updatePath := "updateFile"
	_ = mgs.sendUpdateReply(updatePath)
	_, handled := mgs.handledUpdateReplies.Load(updatePath)
	assert.True(suite.T(), handled)
}

func (suite *SendReplyTestSuite) TestDeleteFailedReply_RemovesMessageFromHandledUpdateReplies() {
	mockContext := context.NewMockDefault()
	messageHandlerMock := &mocks.IMessageHandler{}
	messageHandlerMock.On("RegisterReply", mock.Anything, mock.Anything)
	messageHandlerMock.On("GetMessageUUID", mock.Anything, mock.Anything)
	mgsInteractorRef, err := New(mockContext, messageHandlerMock)
	assert.Nil(suite.T(), err, "initialize passed")
	mgs := mgsInteractorRef.(*MGSInteractor)

	updateFile := "updateFile"
	mgs.handledUpdateReplies.Store(updateFile, true)
	mgs.deleteFailedReply(mockContext.Log(), updateFile)
	_, handled := mgs.handledUpdateReplies.Load(updateFile)
	assert.False(suite.T(), handled)
}

func (suite *SendReplyTestSuite) TestDeleteFailedReply_HandlesFileNotInHandleUpdateReplies() {
	mockContext := context.NewMockDefault()
	messageHandlerMock := &mocks.IMessageHandler{}
	messageHandlerMock.On("RegisterReply", mock.Anything, mock.Anything)
	messageHandlerMock.On("GetMessageUUID", mock.Anything, mock.Anything)
	mgsInteractorRef, err := New(mockContext, messageHandlerMock)
	assert.Nil(suite.T(), err, "initialize passed")
	mgs := mgsInteractorRef.(*MGSInteractor)

	updateFile := "updateFile"
	otherFile := "otherFile"
	mgs.handledUpdateReplies.Store(updateFile, true)
	mgs.deleteFailedReply(mockContext.Log(), otherFile)
	_, handled := mgs.handledUpdateReplies.Load(updateFile)
	assert.True(suite.T(), handled)
}

func (suite *SendReplyTestSuite) TestIsUpdaterWriteEvent_ReturnsTrueForUpdateWriteEvent() {
	mgs := suite.getMGSInteractorRef(nil)

	event := fsnotify.Event{
		Op:   fsnotify.Write,
		Name: updateSuffix,
	}
	assert.True(suite.T(), mgs.isUpdateWriteEvent(event))
}

func (suite *SendReplyTestSuite) TestIsUpdaterWriteEvent_ReturnsFalseForUpdateNonWriteEvent() {
	mgs := suite.getMGSInteractorRef(nil)

	event := fsnotify.Event{
		Op:   fsnotify.Create,
		Name: updateSuffix,
	}
	assert.False(suite.T(), mgs.isUpdateWriteEvent(event))
}

func (suite *SendReplyTestSuite) TestIsUpdaterWriteEvent_ReturnsFalseForNonUpdateWriteEvent() {
	mgs := suite.getMGSInteractorRef(nil)

	event := fsnotify.Event{
		Op:   fsnotify.Write,
		Name: "notUpdate",
	}
	assert.False(suite.T(), mgs.isUpdateWriteEvent(event))
}

func (suite *SendReplyTestSuite) TestStopUpdateReplyFileWatcher_WritesTrueToUpdateWatcherDoneChannel() {
	mgs := suite.getMGSInteractorRef(nil)
	mgs.updateWatcherDone = make(chan bool, 1)
	mgs.stopUpdateReplyFileWatcher()
	assert.True(suite.T(), <-mgs.updateWatcherDone)
}

func (suite *SendReplyTestSuite) getMGSInteractorRef(sendControlChannelErr error) *MGSInteractor {
	mockControlChannel := &controlChannelMock.IControlChannel{}
	mockControlChannel.On("SendMessage", mock.Anything, mock.Anything, websocket.BinaryMessage).Return(sendControlChannelErr)
	mockContext := context.NewMockDefault()
	messageHandlerMock := &mocks.IMessageHandler{}
	messageHandlerMock.On("RegisterReply", mock.Anything, mock.Anything)
	messageHandlerMock.On("GetMessageUUID", mock.Anything, mock.Anything)
	mgsInteractorRef, err := New(mockContext, messageHandlerMock)
	assert.Nil(suite.T(), err, "initialize passed")
	mgsInteractor := mgsInteractorRef.(*MGSInteractor)
	mgsInteractor.controlChannel = mockControlChannel
	return mgsInteractor
}

func (suite *SendReplyTestSuite) getDocumentResultObject() AgentResultLocalStoreData {
	pluginRes := contracts.PluginResult{
		PluginID:   "aws:runScript",
		PluginName: "aws:runScript",
		Status:     contracts.ResultStatusSuccess,
		Code:       0,
	}
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginResults[pluginRes.PluginID] = &pluginRes
	result := contracts.DocumentResult{
		MessageID:     "1234",
		PluginResults: pluginResults,
		Status:        contracts.ResultStatusSuccess,
		LastPlugin:    "",
	}
	reply := AgentResultLocalStoreData{
		AgentResult: result,
		ReplyId:     uuid.NewV4().String(),
		RetryNumber: 0,
	}
	return reply
}
