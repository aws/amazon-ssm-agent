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

// Package outofproc implements Executer interface with out-of-process plugin running capabilities
package outofproc

import (
	"testing"

	"time"

	"errors"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	executermocks "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/mock"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/channel"
	channelmock "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/channel/mock"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"src/github.com/stretchr/testify/assert"
)

var pluginRunner PluginRunner

// At integ_test, procController is a singleton that represents an entire lifecycle of a child process
var fakeProcController *FakeProcController

func setup(t *testing.T) *TestCase {
	logger.Info("initalizing dependencies for integration testing...")
	testCase := CreateTestCase()
	channelCreator = func(mode channel.Mode) channel.Channel {
		return channelmock.NewFakeChannel(mode)
	}
	channelDiscoverer = func(documentID string) (string, bool) {
		if channelmock.IsClose(documentID) {
			return "", false
		} else {
			return documentID, true
		}
	}
	fakeProcController = NewFakeProcController(t)
	testCase.executer.procController = fakeProcController
	return testCase
}

func teardown(t *testing.T) {
	//assert the child process died
	assert.False(t, fakeProcController.attached)
	assert.False(t, fakeProcController.live)
	//assert IPC channel is destroyed
	assert.True(t, channelmock.IsClose(testDocumentID))
}

func TestOutOfProcExecuter_Success(t *testing.T) {
	testCase := setup(t)
	testDocState := testCase.docState
	testPlugin := "aws:runScript"
	resultDocState := testCase.docState
	resultDocState.DocumentInformation.DocumentStatus = contracts.ResultStatusSuccess
	outofprocExe := testCase.executer
	testCase.docStore.On("Load").Return(testDocState)
	testCase.docStore.On("Save", resultDocState).Return(nil)
	pluginRunner = func(
		context context.T,
		plugins []model.PluginState,
		resChan chan contracts.PluginResult,
		cancelFlag task.CancelFlag,
	) {

		for _, res := range testCase.results {
			resChan <- *res
		}
		close(resChan)
	}
	cancelFlag := task.NewChanneledCancelFlag()
	resChan := outofprocExe.Run(cancelFlag, testCase.docStore)
	//Plugin update
	res := <-resChan
	//TODO change plugin id to unique in testcases
	assertValueEqual(t, testCase.results, res.PluginResults)
	assert.Equal(t, testPlugin, res.LastPlugin)
	assert.Equal(t, contracts.ResultStatusInProgress, res.Status)
	assert.Equal(t, testDocumentName, res.DocumentName)
	assert.Equal(t, testAssociationID, res.AssociationID)
	assert.Equal(t, testMessageID, res.MessageID)
	assert.Equal(t, len(testCase.docState.InstancePluginsInformation), res.NPlugins)
	assert.Equal(t, testDocumentVersion, res.DocumentVersion)
	//complete result
	res = <-resChan
	assertValueEqual(t, testCase.results, res.PluginResults)
	assert.Equal(t, "", res.LastPlugin)
	assert.Equal(t, testCase.resultStatus, res.Status)
	assert.Equal(t, testDocumentName, res.DocumentName)
	assert.Equal(t, testAssociationID, res.AssociationID)
	assert.Equal(t, testMessageID, res.MessageID)
	assert.Equal(t, len(testCase.docState.InstancePluginsInformation), res.NPlugins)
	assert.Equal(t, testDocumentVersion, res.DocumentVersion)
	//wait for messaging worker to finish
	_, more := <-resChan
	//assert response channel closed
	assert.False(t, more)
	testCase.docStore.AssertExpectations(t)
	teardown(t)
}

func TestOutOfProcExecuter_ShutdownAndReconnect(t *testing.T) {
	testCase := setup(t)
	testDocState := testCase.docState
	testPlugin := "aws:runScript"
	resultDocState := testCase.docState
	resultDocState.DocumentInformation.DocumentStatus = contracts.ResultStatusSuccess
	outofprocExe := testCase.executer
	testCase.docStore.On("Load").Return(testDocState)
	//the temp state is still the original one
	testCase.docStore.On("Save", testDocState).Return(nil)

	masterCancelFlag := task.NewChanneledCancelFlag()
	masterClosed := make(chan bool)
	pluginRunner = func(
		context context.T,
		plugins []model.PluginState,
		resChan chan contracts.PluginResult,
		cancelFlag task.CancelFlag,
	) {
		//wait for master to shutdown
		<-masterClosed
		//then start to send reply
		for _, res := range testCase.results {
			resChan <- *res
		}
		close(resChan)
	}
	logger.Info("launching out-of-proc Executer...")
	resChan := outofprocExe.Run(masterCancelFlag, testCase.docStore)
	logger.Info("shutting down the out-of-proc Executer...")
	masterCancelFlag.Set(task.ShutDown)
	_, more := <-resChan
	//make sure the result channel is closed
	assert.False(t, more)
	masterClosed <- true
	/***************************************************************************************************************
	 **    Executer experienced a shutdown
	 **    Relauched after Processor comes back
	 ****************************************************************************************************************/
	//TODO assert procController.Find()
	//now, with the same procController stubbed, launch the old Executer
	logger.Info("relaunching the out-of-proc Executer...")
	newCancelFlag := task.NewChanneledCancelFlag()
	newDocStore := new(executermocks.MockDocumentStore)
	newDocStore.On("Load").Return(testDocState)
	newDocStore.On("Save", resultDocState).Return(nil)
	newResChan := outofprocExe.Run(newCancelFlag, newDocStore)
	res := <-newResChan
	//TODO change plugin id to unique in testcases
	assertValueEqual(t, testCase.results, res.PluginResults)
	assert.Equal(t, testPlugin, res.LastPlugin)
	assert.Equal(t, contracts.ResultStatusInProgress, res.Status)
	assert.Equal(t, testDocumentName, res.DocumentName)
	assert.Equal(t, testAssociationID, res.AssociationID)
	assert.Equal(t, testMessageID, res.MessageID)
	assert.Equal(t, len(testCase.docState.InstancePluginsInformation), res.NPlugins)
	assert.Equal(t, testDocumentVersion, res.DocumentVersion)
	//complete result
	res = <-newResChan
	assertValueEqual(t, testCase.results, res.PluginResults)
	assert.Equal(t, "", res.LastPlugin)
	assert.Equal(t, testCase.resultStatus, res.Status)
	assert.Equal(t, testDocumentName, res.DocumentName)
	assert.Equal(t, testAssociationID, res.AssociationID)
	assert.Equal(t, testMessageID, res.MessageID)
	assert.Equal(t, len(testCase.docState.InstancePluginsInformation), res.NPlugins)
	assert.Equal(t, testDocumentVersion, res.DocumentVersion)
	//wait for messaging worker to finish
	_, more = <-newResChan
	//assert response channel closed
	assert.False(t, more)
	testCase.docStore.AssertExpectations(t)
	newDocStore.AssertExpectations(t)
	teardown(t)

}

func TestOutOfProcExecuter_Cancel(t *testing.T) {
	testCase := setup(t)
	//test result is canceled
	testCase.resultStatus = contracts.ResultStatusCancelled
	testDocState := testCase.docState
	testPlugin := "aws:runScript"
	resultDocState := testCase.docState
	resultDocState.DocumentInformation.DocumentStatus = contracts.ResultStatusCancelled
	outofprocExe := testCase.executer
	testCase.docStore.On("Load").Return(testDocState)
	testCase.docStore.On("Save", resultDocState).Return(nil)
	pluginRunner = func(
		context context.T,
		plugins []model.PluginState,
		resChan chan contracts.PluginResult,
		cancelFlag task.CancelFlag,
	) {
		//make sure the cancelflag is well received
		cancelFlag.Wait()
		assert.True(t, cancelFlag.Canceled())
		//then start to send reply
		//TODO once we have multi-plugin test case, we can interleave success and cancel here
		for _, res := range testCase.results {
			res.Status = contracts.ResultStatusCancelled
			resChan <- *res
		}
		close(resChan)
	}
	logger.Info("launching out-of-proc Executer...")
	masterCancelFlag := task.NewChanneledCancelFlag()
	resChan := outofprocExe.Run(masterCancelFlag, testCase.docStore)
	masterCancelFlag.Set(task.Canceled)
	res := <-resChan
	//TODO change plugin id to unique in testcases
	assert.Equal(t, testPlugin, res.LastPlugin)
	assert.Equal(t, contracts.ResultStatusInProgress, res.Status)
	assert.Equal(t, testDocumentName, res.DocumentName)
	assert.Equal(t, testAssociationID, res.AssociationID)
	assert.Equal(t, testMessageID, res.MessageID)
	assert.Equal(t, len(testCase.docState.InstancePluginsInformation), res.NPlugins)
	assert.Equal(t, testDocumentVersion, res.DocumentVersion)
	//complete result
	res = <-resChan
	assert.Equal(t, "", res.LastPlugin)
	assert.Equal(t, testCase.resultStatus, res.Status)
	assert.Equal(t, testDocumentName, res.DocumentName)
	assert.Equal(t, testAssociationID, res.AssociationID)
	assert.Equal(t, testMessageID, res.MessageID)
	assert.Equal(t, len(testCase.docState.InstancePluginsInformation), res.NPlugins)
	assert.Equal(t, testDocumentVersion, res.DocumentVersion)
	//wait for messaging worker to finish
	_, more := <-resChan
	//assert response channel closed
	assert.False(t, more)
	testCase.docStore.AssertExpectations(t)
	teardown(t)
}

//replicate the same procedure as the worker main function
func (p *FakeProcController) fakeWorker(t *testing.T, handle string) {
	ctx := context.NewMockDefaultWithContext([]string{"FAKE-DOCUMENT-WORKER"})
	log := ctx.Log()
	log.Info("document process started")
	//make sure the channel name is correct
	assert.Equal(t, testDocumentID, handle)
	ipc := channelCreator(channel.ModeWorker)
	if err := ipc.Open(handle); err != nil {
		logger.Errorf("failed to connect to channel: %v", handle)
		assert.FailNow(t, "worker")
		return
	}
	pipeline, stopChan := NewWorkerBackend(ctx, pluginRunner)
	if err := Messaging(log, ipc, pipeline, stopChan); err != nil {
		logger.Errorf("messaging worker encountered error: %v", err)
		assert.Fail(t, "worker messaging returned err")
	}
	//assume process will die at this point
	p.live = false
}

type FakeProcController struct {
	live     bool
	attached bool
	t        *testing.T
}

func NewFakeProcController(t *testing.T) *FakeProcController {
	return &FakeProcController{
		t: t,
	}
}
func (p *FakeProcController) StartProcess(name string, argv []string) (int, error) {
	if p.live {
		//launched more than one process, fatal problem
		assert.FailNow(p.t, "fatal error: attempt to launch 2 document worker")
		return -1, errors.New("process already exists")
	}
	p.attached = true
	p.live = true
	docID := argv[0]
	go p.fakeWorker(p.t, docID)
	return 0, nil
}

func (p *FakeProcController) Kill() error {
	p.attached = false
	p.live = false
	return nil
}

func (p *FakeProcController) Release() error {
	p.attached = false
	return nil
}

//TODO this will be useful once we added the test
func (p *FakeProcController) Find(pid int, t time.Time) bool {
	return p.live
}
