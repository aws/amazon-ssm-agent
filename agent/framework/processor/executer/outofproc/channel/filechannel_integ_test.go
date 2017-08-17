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

//Package channel defines and implements the communication interface between agent and command runner process
package channel

import (
	"testing"
	"time"

	"path"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"src/github.com/stretchr/testify/assert"
)

var logger = log.NewMockLog()
var defaultRootDir = "."
var channelName = "testchannel"
var messageSet1 = []string{"s000", "s001", "s002"}
var messageSet2 = []string{"r000", "r001", "r002"}

func TestNewFileWatcherChannelDuplexTransmission(t *testing.T) {
	logger.Info("hello filewatcher channel started")
	done := make(chan bool)

	agentChannel := NewFileWatcherChannel(log.NewMockLogWithContext("AGENT"), ModeMaster)

	err := agentChannel.Open(path.Join(defaultRootDir, channelName))
	assert.NoError(t, err)
	logger.Info("agent channel opened, start transmission")
	// run all threads in parallel
	send(agentChannel, messageSet1, "agent")
	go verifyReceive(t, agentChannel, messageSet2, "agent", done)

	workerChannel := NewFileWatcherChannel(log.NewMockLogWithContext("WORKER"), ModeWorker)
	err = workerChannel.Open(path.Join(defaultRootDir, channelName))
	assert.NoError(t, err)
	logger.Info("worker channel opened, start transmission")
	send(workerChannel, messageSet2, "worker")

	go verifyReceive(t, workerChannel, messageSet1, "worker", done)
	<-done
	<-done
	workerChannel.Close()
	agentChannel.Close()
}

//agent channel is reopened, and starts receiving only after re-open
func TestChannelReopen(t *testing.T) {
	done := make(chan bool)
	agentChannel := NewFileWatcherChannel(log.NewMockLogWithContext("AGENT"), ModeMaster)
	err := agentChannel.Open(path.Join(defaultRootDir, channelName))
	assert.NoError(t, err)
	logger.Info("agent channel opened, start transmission")
	// run all threads in parallel
	send(agentChannel, messageSet1, "agent")
	workerChannel := NewFileWatcherChannel(log.NewMockLogWithContext("WORKER"), ModeWorker)
	if err := workerChannel.Open(path.Join(defaultRootDir, channelName)); err != nil {
		logger.Errorf("Error encountered on opening slave channel: %v", err)
		return
	}
	logger.Info("worker channel opened, start transmission")
	send(workerChannel, messageSet2, "worker")

	logger.Info("re-opening agent channel...")
	newAgentChannel := NewFileWatcherChannel(log.NewMockLogWithContext("NEWAGENT"), ModeMaster)
	err = newAgentChannel.Open(path.Join(defaultRootDir, channelName))
	send(newAgentChannel, messageSet2, "new agent")
	assert.NoError(t, err)
	go verifyReceive(t, newAgentChannel, messageSet2, "new agent", done)
	go verifyReceive(t, workerChannel, append(messageSet1, messageSet2...), "worker", done)
	agentChannel.Close()
	workerChannel.Close()
}

//verify the given set of messages are received
func verifyReceive(t *testing.T, ch Channel, messages []string, name string, done chan bool) {

	//timer := time.After(5 * time.Second)
	onMsgChan := ch.GetMessageChannel()
	for _, testMsg := range messages {
		msg := <-onMsgChan
		logger.Infof("%v received message: %v", name, msg)
		assert.Equal(t, testMsg, msg)

	}
	done <- true
}

//send a given set of messages
func send(ch Channel, messages []string, name string) {

	for _, testMsg := range messages {
		logger.Infof("%v sending messages: %v", name, testMsg)
		ch.Send(testMsg)
		time.Sleep(500 * time.Millisecond)
	}
}
