// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package processor defines the document processing unit interface
package processor

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer"
	executermocks "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/mock"
	contextmocks "github.com/aws/amazon-ssm-agent/agent/mocks/context"
	taskmocks "github.com/aws/amazon-ssm-agent/agent/mocks/task"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestEngineProcessor_Submit tests the basic flow of start command thread operation
// this function submits to the job pool
func TestEngineProcessor_Submit(t *testing.T) {
	sendCommandPoolMock := new(taskmocks.MockedPool)
	ctx := contextmocks.NewMockDefault()
	executerMock := executermocks.NewMockExecuter()
	creator := func(ctx context.T) executer.Executer {
		return executerMock
	}
	sendCommandPoolMock.On("Submit", ctx.Log(), "messageID", mock.Anything).Return(nil)
	sendCommandPoolMock.On("BufferTokensIssued").Return(0)

	docMock := new(DocumentMgrMock)
	processor := EngineProcessor{
		executerCreator: creator,
		sendCommandPool: sendCommandPoolMock,
		context:         ctx,
		documentMgr:     docMock,
		startWorker:     NewWorkerProcessorSpec(ctx, 1, contracts.StartSession, 0),
	}
	docState := contracts.DocumentState{}
	docState.DocumentInformation.MessageID = "messageID"
	docState.DocumentType = contracts.StartSession
	docMock.On("PersistDocumentState", mock.Anything, appconfig.DefaultLocationOfPending, docState)
	errorCode := processor.Submit(docState)
	assert.Equal(t, errorCode, ErrorCode(""))
	sendCommandPoolMock.AssertExpectations(t)
}

func TestEngineProcessor_Cancel(t *testing.T) {
	cancelCommandPoolMock := new(taskmocks.MockedPool)
	ctx := contextmocks.NewMockDefault()
	docMock := new(DocumentMgrMock)
	processor := EngineProcessor{
		context:           ctx,
		documentMgr:       docMock,
		cancelCommandPool: cancelCommandPoolMock,
		cancelWorker:      NewWorkerProcessorSpec(ctx, 1, contracts.TerminateSession, 0),
		startWorker:       NewWorkerProcessorSpec(ctx, 1, contracts.StartSession, 0),
	}
	cancelCommandPoolMock.On("Submit", ctx.Log(), "cancelMessageID", mock.Anything).Return(nil)
	cancelCommandPoolMock.On("BufferTokensIssued").Return(0)

	docState := contracts.DocumentState{}
	expectedVal := "cancelMessageID"
	docState.DocumentInformation.MessageID = expectedVal
	docState.DocumentType = contracts.TerminateSession

	docMock.On("PersistDocumentState", mock.Anything, appconfig.DefaultLocationOfPending, docState)
	errorCode := processor.Cancel(docState)
	assert.Equal(t, errorCode, ErrorCode(""))
	docMock.AssertExpectations(t)
}

func TestEngineProcessor_Stop(t *testing.T) {
	sendCommandPoolMock := new(taskmocks.MockedPool)
	cancelCommandPoolMock := new(taskmocks.MockedPool)
	ctx := contextmocks.NewMockDefault()
	resChan := make(chan contracts.DocumentResult)
	processor := EngineProcessor{
		sendCommandPool:   sendCommandPoolMock,
		cancelCommandPool: cancelCommandPoolMock,
		context:           ctx,
		resChan:           resChan,
	}
	sendCommandPoolMock.On("ShutdownAndWait", mock.AnythingOfType("time.Duration")).Return(true)
	cancelCommandPoolMock.On("ShutdownAndWait", mock.AnythingOfType("time.Duration")).Return(true)
	processor.Stop()
	sendCommandPoolMock.AssertExpectations(t)
	cancelCommandPoolMock.AssertExpectations(t)
	// multiple stop
	sendCommandPoolMock = new(taskmocks.MockedPool)
	cancelCommandPoolMock = new(taskmocks.MockedPool)
	processor.Stop()
	sendCommandPoolMock.AssertNotCalled(t, "ShutdownAndWait", mock.AnythingOfType("time.Duration"))
	cancelCommandPoolMock.AssertNotCalled(t, "ShutdownAndWait", mock.AnythingOfType("time.Duration"))
}

// TODO add shutdown and reboot test once we encapsulate docmanager
func TestProcessCommand(t *testing.T) {
	ctx := contextmocks.NewMockDefault()
	docState := contracts.DocumentState{}
	docState.DocumentInformation.MessageID = "messageID"
	docState.DocumentInformation.InstanceID = "instanceID"
	docState.DocumentInformation.DocumentID = "documentID"
	executerMock := executermocks.NewMockExecuter()
	resChan := make(chan contracts.DocumentResult)
	statusChan := make(chan contracts.DocumentResult)
	cancelFlag := task.NewChanneledCancelFlag()
	executerMock.On("Run", cancelFlag, mock.AnythingOfType("*executer.DocumentFileStore")).Return(statusChan)

	// call method under test
	//orchestrationRootDir is set to empty such that it can meet the test expectation.
	creator := func(ctx context.T) executer.Executer {
		return executerMock
	}
	go func() {
		//send 3 updates
		for i := 0; i < 3; i++ {
			last := ""
			if i < 2 {
				last = fmt.Sprintf("plugin%d", i)
			}
			res := contracts.DocumentResult{
				LastPlugin: last,
				Status:     contracts.ResultStatusSuccess,
			}
			statusChan <- res
			res2 := <-resChan
			assert.Equal(t, res, res2)
		}
		close(statusChan)
	}()
	docMock := new(DocumentMgrMock)
	docMock.On("MoveDocumentState", "documentID", appconfig.DefaultLocationOfPending, appconfig.DefaultLocationOfCurrent)
	docMock.On("RemoveDocumentState", "documentID", appconfig.DefaultLocationOfCurrent)
	processCommand(ctx, creator, cancelFlag, resChan, &docState, docMock)
	executerMock.AssertExpectations(t)
	docMock.AssertExpectations(t)
	close(resChan)
	//assert channel is not closed, each instance of Processor keeps a distinct copy of channel
	assert.NotNil(t, resChan)
}

func TestCheckDocSubmissionAllowed(t *testing.T) {
	sendCommandPoolMock := new(taskmocks.MockedPool)
	ctx := contextmocks.NewMockDefault()
	resChan := make(chan contracts.DocumentResult)
	processor := EngineProcessor{
		sendCommandPool:             sendCommandPoolMock,
		context:                     ctx,
		resChan:                     resChan,
		startWorker:                 NewWorkerProcessorSpec(ctx, 1, contracts.StartSession, 1),
		poolToProcessorErrorCodeMap: make(map[task.PoolErrorCode]ErrorCode),
	}
	sendCommandPoolMock.On("AcquireBufferToken", "messageID").Return(task.JobQueueFull)
	sendCommandPoolMock.On("ReleaseBufferToken", "messageID").Return(task.PoolErrorCode(""))
	sendCommandPoolMock.On("BufferTokensIssued").Return(1)

	bufferLimit := 1
	docState := contracts.DocumentState{}
	docState.DocumentInformation.MessageID = "messageID"
	docState.DocumentInformation.InstanceID = "instanceID"
	docState.DocumentInformation.DocumentID = "documentID"
	docState.DocumentType = contracts.StartSession

	errorCode := processor.checkDocSubmissionAllowed(&docState, sendCommandPoolMock, bufferLimit)
	assert.Equal(t, ConversionFailed, errorCode, "conversion failed")

	processor.loadProcessorPoolErrorCodes()
	sendCommandPoolMock = new(taskmocks.MockedPool)
	sendCommandPoolMock.On("BufferTokensIssued").Return(0)
	sendCommandPoolMock.On("AcquireBufferToken", mock.Anything).Return(task.JobQueueFull)
	processor.sendCommandPool = sendCommandPoolMock
	errorCode = processor.checkDocSubmissionAllowed(&docState, sendCommandPoolMock, bufferLimit)
	assert.Equal(t, CommandBufferFull, errorCode, "command buffer full")
}

func TestDocSubmission_Panic(t *testing.T) {
	sendCommandPoolMock := new(taskmocks.MockedPool)
	ctx := contextmocks.NewMockDefault()
	executerMock := executermocks.NewMockExecuter()
	creator := func(ctx context.T) executer.Executer {
		return executerMock
	}
	sendCommandPoolMock.On("Submit", ctx.Log(), "messageID", mock.Anything).Return(nil)
	sendCommandPoolMock.On("BufferTokensIssued").Return(0)
	sendCommandPoolMock.On("AcquireBufferToken", mock.Anything).Return(task.PoolErrorCode(""))
	sendCommandPoolMock.On("ReleaseBufferToken", mock.Anything).Return(task.PoolErrorCode(""))

	processor := EngineProcessor{
		executerCreator: creator,
		sendCommandPool: sendCommandPoolMock,
		context:         ctx,
		documentMgr:     nil, // assigning nil panics Submit()
		startWorker:     NewWorkerProcessorSpec(ctx, 1, contracts.StartSession, 1),
	}
	docState := contracts.DocumentState{}
	docState.DocumentInformation.MessageID = "messageID"
	docState.DocumentType = contracts.StartSession

	errorCode := processor.Submit(docState)
	assert.Equal(t, errorCode, SubmissionPanic)
}

func TestDocSubmission_CheckDocSubmissionAllowedError(t *testing.T) {
	ctx := contextmocks.NewMockDefault()
	executerMock := executermocks.NewMockExecuter()
	creator := func(ctx context.T) executer.Executer {
		return executerMock
	}

	sendCommandPoolMock := new(taskmocks.MockedPool)
	processor := EngineProcessor{
		executerCreator:             creator,
		sendCommandPool:             sendCommandPoolMock,
		context:                     ctx,
		documentMgr:                 nil, // assigning nil panics Submit()
		startWorker:                 NewWorkerProcessorSpec(ctx, 1, contracts.StartSession, 1),
		poolToProcessorErrorCodeMap: make(map[task.PoolErrorCode]ErrorCode),
	}
	processor.loadProcessorPoolErrorCodes()
	docState := contracts.DocumentState{}
	docState.DocumentInformation.MessageID = "messageID"
	docState.DocumentInformation.InstanceID = "instanceID"
	docState.DocumentInformation.DocumentID = "documentID"
	docState.DocumentType = contracts.StartSession

	docMock := new(DocumentMgrMock)
	docMock.On("PersistDocumentState", mock.Anything, appconfig.DefaultLocationOfPending, docState)

	sendCommandPoolMock.On("AcquireBufferToken", mock.Anything).Return(task.DuplicateCommand)
	sendCommandPoolMock.On("Submit", ctx.Log(), "messageID", mock.Anything).Return(nil)
	sendCommandPoolMock.On("BufferTokensIssued").Return(0)
	sendCommandPoolMock.On("ReleaseBufferToken", mock.Anything).Return(task.PoolErrorCode(""))
	errorCode := processor.Submit(docState)
	assert.Equal(t, errorCode, DuplicateCommand)

	sendCommandPoolMock = new(taskmocks.MockedPool)
	sendCommandPoolMock.On("BufferTokensIssued").Return(0)
	sendCommandPoolMock.On("AcquireBufferToken", mock.Anything).Return(task.InvalidJobId)
	processor.sendCommandPool = sendCommandPoolMock
	errorCode = processor.Submit(docState)
	assert.Equal(t, errorCode, InvalidDocumentId)

	sendCommandPoolMock = new(taskmocks.MockedPool)
	sendCommandPoolMock.On("BufferTokensIssued").Return(0)
	sendCommandPoolMock.On("AcquireBufferToken", mock.Anything).Return(task.JobQueueFull)
	processor.sendCommandPool = sendCommandPoolMock
	errorCode = processor.Submit(docState)
	assert.Equal(t, errorCode, CommandBufferFull)
}

func TestDocCancellation_Panic(t *testing.T) {
	cancelCommandPoolMock := new(taskmocks.MockedPool)
	ctx := contextmocks.NewMockDefault()
	executerMock := executermocks.NewMockExecuter()
	creator := func(ctx context.T) executer.Executer {
		return executerMock
	}
	cancelCommandPoolMock.On("Submit", ctx.Log(), "messageID", mock.Anything).Return(nil)
	cancelCommandPoolMock.On("BufferTokensIssued").Return(0)
	cancelCommandPoolMock.On("AcquireBufferToken", mock.Anything).Return(task.PoolErrorCode(""))
	cancelCommandPoolMock.On("ReleaseBufferToken", mock.Anything).Return(task.PoolErrorCode(""))

	processor := EngineProcessor{
		executerCreator:   creator,
		cancelCommandPool: cancelCommandPoolMock,
		context:           ctx,
		documentMgr:       nil, // assigning nil panics Submit()
		startWorker:       NewWorkerProcessorSpec(ctx, 1, contracts.StartSession, 1),
		cancelWorker:      NewWorkerProcessorSpec(ctx, 1, contracts.TerminateSession, 1),
	}
	docState := contracts.DocumentState{}
	docState.DocumentInformation.MessageID = "messageID"
	docState.DocumentType = contracts.TerminateSession

	errorCode := processor.Cancel(docState)
	assert.Equal(t, errorCode, SubmissionPanic)
}

// TODO add shutdown and reboot test once we encapsulate docmanager
func TestProcessCommand_Shutdown(t *testing.T) {
	ctx := contextmocks.NewMockDefault()
	docState := contracts.DocumentState{}
	docState.DocumentInformation.MessageID = "messageID"
	docState.DocumentInformation.InstanceID = "instanceID"
	docState.DocumentInformation.DocumentID = "documentID"
	executerMock := executermocks.NewMockExecuter()
	resChan := make(chan contracts.DocumentResult)
	statusChan := make(chan contracts.DocumentResult)
	cancelFlag := task.NewChanneledCancelFlag()
	executerMock.On("Run", cancelFlag, mock.AnythingOfType("*executer.DocumentFileStore")).Return(statusChan)

	// call method under test
	//orchestrationRootDir is set to empty such that it can meet the test expectation.
	creator := func(ctx context.T) executer.Executer {
		return executerMock
	}
	go func() {
		//executer shutdown
		close(statusChan)
	}()
	docMock := new(DocumentMgrMock)
	docMock.On("MoveDocumentState", "documentID", appconfig.DefaultLocationOfPending, appconfig.DefaultLocationOfCurrent)
	processCommand(ctx, creator, cancelFlag, resChan, &docState, docMock)
	executerMock.AssertExpectations(t)
	docMock.AssertExpectations(t)
	close(resChan)
	//assert channel is not closed, each instance of Processor keeps a distinct copy of channel
	assert.NotNil(t, resChan)
	//TODO assert document file is not moved

}

func TestProcessCancelCommand_Success(t *testing.T) {
	ctx := contextmocks.NewMockDefault()
	sendCommandPoolMock := new(taskmocks.MockedPool)
	docState := contracts.DocumentState{}
	docState.CancelInformation.CancelMessageID = "messageID"
	sendCommandPoolMock.On("Cancel", "messageID").Return(true)
	docMock := new(DocumentMgrMock)
	docMock.On("MoveDocumentState", "", appconfig.DefaultLocationOfPending, appconfig.DefaultLocationOfCurrent)
	docMock.On("RemoveDocumentState", "", appconfig.DefaultLocationOfCurrent, mock.Anything)
	processCancelCommand(ctx, sendCommandPoolMock, &docState, docMock)
	sendCommandPoolMock.AssertExpectations(t)
	docMock.AssertExpectations(t)
	assert.Equal(t, docState.DocumentInformation.DocumentStatus, contracts.ResultStatusSuccess)

}

type DocumentMgrMock struct {
	mock.Mock
}

func (m *DocumentMgrMock) MoveDocumentState(fileName, srcLocationFolder, dstLocationFolder string) {
	m.Called(fileName, srcLocationFolder, dstLocationFolder)
	return
}

func (m *DocumentMgrMock) PersistDocumentState(fileName, locationFolder string, state contracts.DocumentState) {
	m.Called(fileName, locationFolder, state)
	return
}

func (m *DocumentMgrMock) GetDocumentState(fileName, locationFolder string) contracts.DocumentState {
	args := m.Called(fileName, locationFolder)
	return args.Get(0).(contracts.DocumentState)
}

func (m *DocumentMgrMock) RemoveDocumentState(documentID, location string) {
	m.Called(documentID, location)
	return
}
