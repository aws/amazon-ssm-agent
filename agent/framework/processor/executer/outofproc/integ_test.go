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

//+build integration

// Package outofproc implements Executer interface with out-of-process plugin running capabilities
package outofproc

import (
	"errors"
	"testing"

	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	executermocks "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/mock"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/channel"
	channelmock "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/channel/mock"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/messaging"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/proc"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
)

var pluginRunner messaging.PluginRunner

// At integ_test, procController is a singleton that represents an entire lifecycle of a child process
var fakeProcess *FakeProcess

func setup(t *testing.T) *TestCase {
	logger.Info("initializing dependencies for integration testing...")
	testCase := CreateTestCase()
	channelCreator = func(log log.T, mode channel.Mode, documentID string) (channel.Channel, error, bool) {
		isFound := channelmock.IsExists(documentID)
		assert.Equal(t, testDocumentID, documentID)
		fakeChannel := channelmock.NewFakeChannel(logger, mode, documentID)
		//stuff some bad messages to the channel
		fakeChannel.Send("{\"version\":\"1.0\",\"type\":\"some unknown type\",\"content\":\"\"}")
		fakeChannel.Send("bad message 2")
		return fakeChannel, nil, isFound
	}
	//creating fake process
	if fakeProcess != nil {
		t.Fatalf("process already exists: %v", fakeProcess)
	}
	fakeProcess = NewFakeProcess(t)
	processCreator = func(name string, argv []string) (proc.OSProcess, error) {
		//fakeProcess is imposed as singleton here
		if fakeProcess.live {
			t.Fatalf("start process repeatedly, already exists: %v", fakeProcess)
		}
		fakeProcess.live = true
		fakeProcess.attached = true
		docID := argv[0]
		//launc a faked worker
		go fakeProcess.fakeWorker(fakeProcess.t, docID)
		return fakeProcess, nil
	}
	processFinder = func(log log.T, procinfo contracts.OSProcInfo) bool {
		assert.Equal(t, testPid, procinfo.Pid)
		return fakeProcess != nil && fakeProcess.live
	}
	return testCase
}

func teardown(t *testing.T) {
	//assert the child process died, and clear the variable
	assert.False(t, fakeProcess.attached)
	assert.False(t, fakeProcess.live)
	fakeProcess = nil
	//assert IPC channel is destroyed
	assert.False(t, channelmock.IsExists(testDocumentID))
}

func TestOutOfProcExecuter_Success(t *testing.T) {
	testCase := setup(t)
	testDocState := testCase.docState
	resultDocState := testCase.docState
	resultDocState.InstancePluginsInformation[0].Result = *testCase.results["plugin1"]
	resultDocState.InstancePluginsInformation[1].Result = *testCase.results["plugin2"]
	resultDocState.DocumentInformation.DocumentStatus = contracts.ResultStatusSuccess
	resultDocState.DocumentInformation.ProcInfo.Pid = testPid
	resultDocState.DocumentInformation.ProcInfo.StartTime = testStartDateTime
	//using the real constructor
	outofprocExe := NewOutOfProcExecuter(testCase.context)
	testCase.docStore.On("Load").Return(testDocState)
	testCase.docStore.On("Save", resultDocState).Return(nil)
	pluginRunner = func(
		context context.T,
		docState contracts.DocumentState,
		resChan chan contracts.PluginResult,
		cancelFlag task.CancelFlag,
	) {
		//then start to send reply
		resChan <- *testCase.results["plugin1"]
		resChan <- *testCase.results["plugin2"]
		close(resChan)
	}
	cancelFlag := task.NewChanneledCancelFlag()
	resChan := outofprocExe.Run(cancelFlag, testCase.docStore)
	//Plugin1 update
	res := <-resChan
	assert.Equal(t, 1, len(res.PluginResults))
	assert.Equal(t, "plugin1", res.LastPlugin)
	assert.EqualValues(t, testCase.results["plugin1"], res.PluginResults["plugin1"])
	assert.Equal(t, contracts.ResultStatusInProgress, res.Status)
	assert.Equal(t, testDocumentName, res.DocumentName)
	assert.Equal(t, testAssociationID, res.AssociationID)
	assert.Equal(t, testMessageID, res.MessageID)
	assert.Equal(t, len(testCase.docState.InstancePluginsInformation), res.NPlugins)
	assert.Equal(t, testDocumentVersion, res.DocumentVersion)
	//plugin2 update
	res = <-resChan
	assert.Equal(t, "plugin2", res.LastPlugin)
	assertValueEqual(t, testCase.results, res.PluginResults)
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

//TODO test Zombie and Orphan child separately
func TestOutOfProcExecuter_ShutdownAndReconnect(t *testing.T) {
	testCase := setup(t)
	docState1 := testCase.docState
	//still inprogress, update the docState saved
	docState1.DocumentInformation.ProcInfo = contracts.OSProcInfo{
		Pid:       testPid,
		StartTime: testStartDateTime,
	}
	testCase.docStore.On("Load").Return(testCase.docState)
	//the temp state is still the original one
	testCase.docStore.On("Save", docState1).Return(nil)

	masterCancelFlag := task.NewChanneledCancelFlag()
	masterClosed := make(chan bool)
	pluginRunner = func(
		context context.T,
		docState contracts.DocumentState,
		resChan chan contracts.PluginResult,
		cancelFlag task.CancelFlag,
	) {
		//wait for master to shutdown
		<-masterClosed
		//then start to send reply
		resChan <- *testCase.results["plugin1"]
		resChan <- *testCase.results["plugin2"]
		close(resChan)
	}
	logger.Info("launching out-of-proc Executer...")
	outofprocExe := NewOutOfProcExecuter(testCase.context)
	resChan := outofprocExe.Run(masterCancelFlag, testCase.docStore)
	logger.Info("shutting down the out-of-proc Executer...")
	masterCancelFlag.Set(task.ShutDown)
	_, more := <-resChan
	//make sure the result channel is closed
	assert.False(t, more)
	masterClosed <- true
	//detach the child process
	fakeProcess.detach()
	/***************************************************************************************************************
	 **    Executer experienced a shutdown
	 **    Relauched after Processor comes back
	 ****************************************************************************************************************/
	//now, with the same child process, launch the old Executer
	//assert the process is detached: either in orphan state or zombie
	resultDocState := docState1
	//final docstate to be saved
	resultDocState.InstancePluginsInformation[0].Result = *testCase.results["plugin1"]
	resultDocState.InstancePluginsInformation[1].Result = *testCase.results["plugin2"]
	resultDocState.DocumentInformation.DocumentStatus = contracts.ResultStatusSuccess
	assert.False(t, fakeProcess.attached)
	logger.Info("relaunching the out-of-proc Executer...")
	newCancelFlag := task.NewChanneledCancelFlag()
	newDocStore := new(executermocks.MockDocumentStore)
	newDocStore.On("Load").Return(docState1)
	newDocStore.On("Save", resultDocState).Return(nil)
	newContext := context.NewMockDefaultWithContext([]string{"NEWMASTER"})
	newOutofProcExe := NewOutOfProcExecuter(newContext)
	newResChan := newOutofProcExe.Run(newCancelFlag, newDocStore)
	//plugin1 update
	res := <-newResChan
	assert.Equal(t, "plugin1", res.LastPlugin)
	assert.Equal(t, contracts.ResultStatusInProgress, res.Status)
	assert.Equal(t, testDocumentName, res.DocumentName)
	assert.Equal(t, testAssociationID, res.AssociationID)
	assert.Equal(t, testMessageID, res.MessageID)
	assert.Equal(t, len(testCase.docState.InstancePluginsInformation), res.NPlugins)
	assert.Equal(t, testDocumentVersion, res.DocumentVersion)
	//plugin2 update
	res = <-newResChan
	assert.Equal(t, "plugin2", res.LastPlugin)
	assertValueEqual(t, testCase.results, res.PluginResults)
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
	resultDocState := testCase.docState
	for _, res := range testCase.results {
		res.Code = 1
		res.Status = contracts.ResultStatusCancelled
		res.Output = "command has been cancelled"
	}
	resultDocState.InstancePluginsInformation[0].Result = *testCase.results["plugin1"]
	resultDocState.InstancePluginsInformation[1].Result = *testCase.results["plugin2"]
	resultDocState.DocumentInformation.DocumentStatus = contracts.ResultStatusCancelled
	resultDocState.DocumentInformation.ProcInfo.Pid = testPid
	resultDocState.DocumentInformation.ProcInfo.StartTime = testStartDateTime
	outofprocExe := NewOutOfProcExecuter(testCase.context)
	testCase.docStore.On("Load").Return(testDocState)
	testCase.docStore.On("Save", resultDocState).Return(nil)
	pluginRunner = func(
		context context.T,
		docState contracts.DocumentState,
		resChan chan contracts.PluginResult,
		cancelFlag task.CancelFlag,
	) {
		//make sure the cancelflag is well received
		cancelFlag.Wait()
		assert.True(t, cancelFlag.Canceled())
		//then start to send reply
		resChan <- *testCase.results["plugin1"]
		resChan <- *testCase.results["plugin2"]
		close(resChan)
	}
	logger.Info("launching out-of-proc Executer...")
	masterCancelFlag := task.NewChanneledCancelFlag()
	resChan := outofprocExe.Run(masterCancelFlag, testCase.docStore)
	masterCancelFlag.Set(task.Canceled)
	//plugin1 update
	res := <-resChan
	assert.Equal(t, "plugin1", res.LastPlugin)
	assert.Equal(t, contracts.ResultStatusInProgress, res.Status)
	assert.Equal(t, testDocumentName, res.DocumentName)
	assert.Equal(t, testAssociationID, res.AssociationID)
	assert.Equal(t, testMessageID, res.MessageID)
	assert.Equal(t, len(testCase.docState.InstancePluginsInformation), res.NPlugins)
	assert.Equal(t, testDocumentVersion, res.DocumentVersion)
	//plugin2 update
	res = <-resChan
	assert.Equal(t, "plugin2", res.LastPlugin)
	assertValueEqual(t, testCase.results, res.PluginResults)
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

type FakeProcess struct {
	exitChan chan bool
	live     bool
	attached bool
	t        *testing.T
}

func NewFakeProcess(t *testing.T) *FakeProcess {
	return &FakeProcess{
		t:        t,
		exitChan: make(chan bool, 10),
	}
}

//replicate the same procedure as the worker main function
func (p *FakeProcess) fakeWorker(t *testing.T, handle string) {
	ctx := context.NewMockDefaultWithContext([]string{"FAKE-DOCUMENT-WORKER"})
	log := ctx.Log()
	log.Infof("document: %v process started", handle)
	//make sure the channel name is correct
	assert.Equal(t, testDocumentID, handle)
	ipc := channelmock.NewFakeChannel(logger, channel.ModeWorker, handle)
	pipeline := messaging.NewWorkerBackend(ctx, pluginRunner)
	stopTimer := make(chan bool)
	if err := messaging.Messaging(log, ipc, pipeline, stopTimer); err != nil {
		t.Fatalf("worker process messaging encountered error: %v", err)
	}
	log.Info("document worker process exited")
	//process exits
	p.live = false
	p.attached = false
	//faked syscall Wait() should return now
	p.exitChan <- true
}

//Make the process to become an orphan when parent dies
//In reality, Wait() is transferred to OS daemon. In our test cases, Wait() is held by the old Executer so we need to fail the new Executer's Wait() call
func (p *FakeProcess) detach() {
	p.attached = false
}

func (p *FakeProcess) Pid() int {
	return testPid
}

func (p *FakeProcess) Kill() error {
	p.attached = false
	p.live = false
	p.exitChan <- true
	return nil
}

func (p *FakeProcess) StartTime() time.Time {
	return testStartDateTime
}

func (p *FakeProcess) Wait() error {
	//once the child is detached (controlled by our test engine), Wait() is illegal since the Executer is no longer the direct parent of the child
	if !p.attached {
		return errors.New("Wait() called by illegal party")
	}
	<-p.exitChan
	p.live = false
	return nil
}
