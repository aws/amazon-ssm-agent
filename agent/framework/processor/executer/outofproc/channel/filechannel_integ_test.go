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

	agentChannel := NewFileWatcherChannel(log.NewMockLogWithContext("AGENT"), ModeMaster)
	workerChannel := NewFileWatcherChannel(log.NewMockLogWithContext("WORKER"), ModeSlave)
	if err := agentChannel.Open(path.Join(defaultRootDir, channelName)); err != nil {
		logger.Errorf("Error encountered on opening master channel: %v", err)
		return
	}
	if err := workerChannel.Open(path.Join(defaultRootDir, channelName)); err != nil {
		logger.Errorf("Error encountered on opening slave channel: %v", err)
		return
	}
	logger.Info("channel connected, start message exchange")
	done := make(chan bool)
	// run all threads in parallel
	go send(agentChannel, messageSet1, "agent")
	go verifyReceive(t, workerChannel, messageSet1, "worker", done)
	go send(workerChannel, messageSet2, "worker")
	go verifyReceive(t, agentChannel, messageSet2, "agent", done)
	<-done
	<-done
	workerChannel.Close()
	agentChannel.Close()
}

//verify the given set of messages are received
func verifyReceive(t *testing.T, ch Channel, messages []string, name string, done chan bool) {

	//timer := time.After(5 * time.Second)
	onMsgChan := ch.GetMessageChannel()
	for _, testMsg := range messages {
		select {
		case msg := <-onMsgChan:
			logger.Infof("%v received message: %v", name, msg)
			assert.Equal(t, testMsg, msg)
		}
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
