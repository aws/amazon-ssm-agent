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
	"testing"

	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer"
	executermocks "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/mock"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

//TODO implement processor_integ_test once we encapsulate docmanager
func TestEngineProcessor_Submit(t *testing.T) {
	sendCommandPoolMock := new(task.MockedPool)
	ctx := context.NewMockDefault()
	executerMock := executermocks.NewMockExecuter()
	creator := func(ctx context.T) executer.Executer {
		return executerMock
	}
	sendCommandPoolMock.On("Submit", ctx.Log(), "messageID", mock.Anything).Return(nil)
	docMock := new(DocumentMgrMock)
	processor := EngineProcessor{
		executerCreator: creator,
		sendCommandPool: sendCommandPoolMock,
		context:         ctx,
		documentMgr:     docMock,
	}
	docState := contracts.DocumentState{}
	docState.DocumentInformation.MessageID = "messageID"
	docMock.On("PersistDocumentState", mock.Anything, mock.Anything, mock.Anything, appconfig.DefaultLocationOfPending, docState)
	processor.Submit(docState)
	sendCommandPoolMock.AssertExpectations(t)
}

func TestEngineProcessor_Cancel(t *testing.T) {
	cancelCommandPoolMock := new(task.MockedPool)
	ctx := context.NewMockDefault()
	executerMock := executermocks.NewMockExecuter()
	creator := func(ctx context.T) executer.Executer {
		return executerMock
	}
	cancelCommandPoolMock.On("Submit", ctx.Log(), "cancelMessageID", mock.Anything).Return(nil)
	docMock := new(DocumentMgrMock)

	processor := EngineProcessor{
		executerCreator:   creator,
		cancelCommandPool: cancelCommandPoolMock,
		context:           ctx,
		documentMgr:       docMock,
	}
	docState := contracts.DocumentState{}
	docState.DocumentInformation.MessageID = "cancelMessageID"
	docMock.On("PersistDocumentState", mock.Anything, mock.Anything, mock.Anything, appconfig.DefaultLocationOfPending, docState)
	processor.Cancel(docState)
	cancelCommandPoolMock.AssertExpectations(t)
}

func TestEngineProcessor_Stop(t *testing.T) {
	sendCommandPoolMock := new(task.MockedPool)
	cancelCommandPoolMock := new(task.MockedPool)
	ctx := context.NewMockDefault()
	resChan := make(chan contracts.DocumentResult)
	processor := EngineProcessor{
		sendCommandPool:   sendCommandPoolMock,
		cancelCommandPool: cancelCommandPoolMock,
		context:           ctx,
		resChan:           resChan,
	}
	sendCommandPoolMock.On("ShutdownAndWait", mock.AnythingOfType("time.Duration")).Return(true)
	cancelCommandPoolMock.On("ShutdownAndWait", mock.AnythingOfType("time.Duration")).Return(true)
	processor.Stop(contracts.StopTypeSoftStop)
	sendCommandPoolMock.AssertExpectations(t)
	cancelCommandPoolMock.AssertExpectations(t)
}

//TODO add shutdown and reboot test once we encapsulate docmanager
func TestProcessCommand(t *testing.T) {
	ctx := context.NewMockDefault()
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
	docMock.On("MoveDocumentState", mock.Anything, "documentID", "instanceID", appconfig.DefaultLocationOfPending, appconfig.DefaultLocationOfCurrent)
	docMock.On("RemoveDocumentState", mock.Anything, "documentID", "instanceID", appconfig.DefaultLocationOfCurrent)
	processCommand(ctx, creator, cancelFlag, resChan, &docState, docMock)
	executerMock.AssertExpectations(t)
	docMock.AssertExpectations(t)
	close(resChan)
	//assert channel is not closed, each instance of Processor keeps a distinct copy of channel
	assert.NotNil(t, resChan)

}

//TODO add shutdown and reboot test once we encapsulate docmanager
func TestProcessCommand_Shutdown(t *testing.T) {
	ctx := context.NewMockDefault()
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
	docMock.On("MoveDocumentState", mock.Anything, "documentID", "instanceID", appconfig.DefaultLocationOfPending, appconfig.DefaultLocationOfCurrent)
	processCommand(ctx, creator, cancelFlag, resChan, &docState, docMock)
	executerMock.AssertExpectations(t)
	docMock.AssertExpectations(t)
	close(resChan)
	//assert channel is not closed, each instance of Processor keeps a distinct copy of channel
	assert.NotNil(t, resChan)
	//TODO assert document file is not moved

}

func TestProcessCancelCommand_Success(t *testing.T) {
	ctx := context.NewMockDefault()
	sendCommandPoolMock := new(task.MockedPool)
	docState := contracts.DocumentState{}
	docState.CancelInformation.CancelMessageID = "messageID"
	sendCommandPoolMock.On("Cancel", "messageID").Return(true)
	docMock := new(DocumentMgrMock)
	docMock.On("MoveDocumentState", mock.Anything, "", "", appconfig.DefaultLocationOfPending, appconfig.DefaultLocationOfCurrent)
	docMock.On("RemoveDocumentState", mock.Anything, "", "", appconfig.DefaultLocationOfCurrent, mock.Anything)
	processCancelCommand(ctx, sendCommandPoolMock, &docState, docMock)
	sendCommandPoolMock.AssertExpectations(t)
	docMock.AssertExpectations(t)
	assert.Equal(t, docState.DocumentInformation.DocumentStatus, contracts.ResultStatusSuccess)

}

type DocumentMgrMock struct {
	mock.Mock
}

func (m *DocumentMgrMock) MoveDocumentState(log log.T, fileName, instanceID, srcLocationFolder, dstLocationFolder string) {
	m.Called(log, fileName, instanceID, srcLocationFolder, dstLocationFolder)
	return
}

func (m *DocumentMgrMock) PersistDocumentState(log log.T, fileName, instanceID, locationFolder string, state contracts.DocumentState) {
	m.Called(log, fileName, instanceID, locationFolder, state)
	return
}

func (m *DocumentMgrMock) GetDocumentState(log log.T, fileName, instanceID, locationFolder string) contracts.DocumentState {
	args := m.Called(log, fileName, instanceID, locationFolder)
	return args.Get(0).(contracts.DocumentState)
}

func (m *DocumentMgrMock) RemoveDocumentState(log log.T, documentID, instanceID, location string) {
	m.Called(log, documentID, instanceID, location)
	return
}
