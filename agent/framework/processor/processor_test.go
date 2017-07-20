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

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/message/processor/executer"
	"github.com/aws/amazon-ssm-agent/agent/message/processor/executer/mock"
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
	processor := EngineProcessor{
		executerCreator: creator,
		sendCommandPool: sendCommandPoolMock,
		context:         ctx,
	}
	docState := model.DocumentState{}
	docState.DocumentInformation.MessageID = "messageID"
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
	processor := EngineProcessor{
		executerCreator:   creator,
		cancelCommandPool: cancelCommandPoolMock,
		context:           ctx,
	}
	docState := model.DocumentState{}
	docState.DocumentInformation.MessageID = "cancelMessageID"
	processor.Cancel(docState)
	cancelCommandPoolMock.AssertExpectations(t)
}

func TestEngineProcessor_Stop(t *testing.T) {
	sendCommandPoolMock := new(task.MockedPool)
	cancelCommandPoolMock := new(task.MockedPool)
	ctx := context.NewMockDefault()
	processor := EngineProcessor{
		sendCommandPool:   sendCommandPoolMock,
		cancelCommandPool: cancelCommandPoolMock,
		context:           ctx,
	}
	sendCommandPoolMock.On("ShutdownAndWait", mock.AnythingOfType("time.Duration")).Return(true)
	cancelCommandPoolMock.On("ShutdownAndWait", mock.AnythingOfType("time.Duration")).Return(true)
	processor.Stop(contracts.StopTypeSoftStop)
	sendCommandPoolMock.AssertExpectations(t)
	cancelCommandPoolMock.AssertExpectations(t)
}

//TODO add Shut test
func TestProcessCommand(t *testing.T) {
	ctx := context.NewMockDefault()
	docState := model.DocumentState{}
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
			res := contracts.DocumentResult{Status: contracts.ResultStatusSuccess}
			statusChan <- res
			res2 := <-resChan
			assert.Equal(t, res, res2)
		}
		close(statusChan)
	}()
	processCommand(ctx, creator, cancelFlag, resChan, &docState)
	executerMock.AssertExpectations(t)
	close(resChan)
	//assert channel is not closed, each instance of Processor keeps a distinct copy of channel
	assert.NotNil(t, resChan)

}

func TestProcessCancelCommand_Success(t *testing.T) {
	ctx := context.NewMockDefault()
	sendCommandPoolMock := new(task.MockedPool)
	docState := model.DocumentState{}
	docState.CancelInformation.CancelMessageID = "messageID"
	sendCommandPoolMock.On("Cancel", "messageID").Return(true)
	processCancelCommand(ctx, sendCommandPoolMock, &docState)
	sendCommandPoolMock.AssertExpectations(t)
	assert.Equal(t, docState.DocumentInformation.DocumentStatus, contracts.ResultStatusSuccess)

}
