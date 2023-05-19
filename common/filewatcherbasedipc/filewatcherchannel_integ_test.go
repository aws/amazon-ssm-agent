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

//go:build integration
// +build integration

// Package channel defines and implements the communication interface between agent and command runner process
package filewatcherbasedipc

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/stretchr/testify/assert"
)

var logger = log.NewMockLog()
var defaultRootDir = "."
var channelName = "testchannel"
var messageSet1 = []string{"s000", "s001", "s002"}
var messageSet2 = []string{"r000", "r001", "r002"}
var messageSet3 = []string{"n000", "n001", "n002"}

var cwPath, _ = os.Getwd()
var filePath = filepath.Join(cwPath, "sampleChannel.txt")

func TestChannelDuplexTransmission(t *testing.T) {
	logger.Info("hello filewatcher channel started")
	order := []Mode{ModeMaster, ModeWorker}
	for i := 0; i < 2; i++ {
		roleA := order[i%2]
		roleB := order[(i+1)%2]
		done := make(chan bool)
		channelA, err := NewFileWatcherChannel(log.NewMockLogWithContext(string(roleA)), roleA, path.Join(defaultRootDir, channelName), false)
		assert.NoError(t, err)
		logger.Info("agent channel opened, start transmission")
		// sender non-blocked
		send(channelA, messageSet1, string(roleA))
		go verifyReceive(t, channelA, messageSet2, string(roleA), done)

		channelB, err := NewFileWatcherChannel(log.NewMockLogWithContext(string(roleB)), roleB, path.Join(defaultRootDir, channelName), false)
		assert.NoError(t, err)
		logger.Info("worker channel opened, start transmission")
		send(channelB, messageSet2, string(roleB))

		go verifyReceive(t, channelB, messageSet1, string(roleB), done)
		<-done
		<-done
		channelA.Close()
		channelB.Close()
		channelA.Destroy()
		channelB.Destroy()
	}

}

// agent channel is reopened, and starts receiving only after re-open
func TestChannelReopen(t *testing.T) {
	done := make(chan bool)
	agentChannel, err := NewFileWatcherChannel(log.NewMockLogWithContext("AGENT"), ModeMaster, path.Join(defaultRootDir, channelName), false)
	assert.NoError(t, err)
	logger.Info("agent channel opened, start transmission")
	// run all threads in parallel
	send(agentChannel, messageSet1, "agent")
	agentChannel.Close()

	// Sleep a bit to allow agentChannel.Close() to finish closing file watcher.
	time.Sleep(250 * time.Millisecond)

	logger.Info("agent channel closed")
	workerChannel, err := NewFileWatcherChannel(log.NewMockLogWithContext("WORKER"), ModeWorker, path.Join(defaultRootDir, channelName), false)
	assert.NoError(t, err)
	logger.Info("worker channel opened, start transmission")
	send(workerChannel, messageSet3, "worker")

	logger.Info("re-opening agent channel...")
	newAgentChannel, err := NewFileWatcherChannel(log.NewMockLogWithContext("NEWAGENT"), ModeMaster, path.Join(defaultRootDir, channelName), false)
	assert.NoError(t, err)
	send(newAgentChannel, messageSet2, "new agent")
	assert.NoError(t, err)
	go verifyReceive(t, newAgentChannel, messageSet3, "new agent", done)
	go verifyReceive(t, workerChannel, append(messageSet1, messageSet2...), "worker", done)
	workerChannel.Close()
	logger.Info("destroying the file channel")
	newAgentChannel.Destroy()
}

// verify the given set of messages are received
func verifyReceive(t *testing.T, ch IPCChannel, messages []string, name string, done chan bool) {

	//timer := time.After(5 * time.Second)
	for _, testMsg := range messages {
		msg := <-ch.GetMessage()
		logger.Infof("%v received message: %v", name, msg)
		assert.Equal(t, testMsg, msg)

	}
	done <- true
}

// send a given set of messages
func send(ch IPCChannel, messages []string, name string) {

	for _, testMsg := range messages {
		logger.Infof("%v sending messages: %v", name, testMsg)
		ch.Send(testMsg)
		time.Sleep(200 * time.Millisecond)
	}
}

// Test case for reading file
func TestReadFile(t *testing.T) {
	defer func() {
		os.Remove(filePath)
	}()

	fd, err := os.Create(filePath)
	osStatFn = os.Stat

	assert.Nil(t, err)
	fileBytes := []byte("sample content1")
	fd.Write(fileBytes)
	fd.Close()

	output, err := fileRead(logger, filePath)
	assert.Nil(t, err)
	assert.Equal(t, string(output), string(fileBytes))
}

// Test case for reading file through retry
func TestReadFileWithRetry(t *testing.T) {
	defer func() {
		os.Remove(filePath)
	}()
	osStatFn = os.Stat
	fd, err := os.Create(filePath)
	assert.Nil(t, err)
	fileBytes := []byte("sample content2")
	fd.Write(fileBytes)
	fd.Close()

	output, err := fileReadWithRetry(filePath)
	assert.Nil(t, err)
	assert.Equal(t, string(output), string(fileBytes))
}

// Test case for reading file through retry and produce error
func TestReadFileRetryWithError(t *testing.T) {
	defer func() {
		os.Remove(filePath)
	}()
	dummyError := "dummy error"
	osStatFn = func(name string) (info os.FileInfo, err error) {
		return info, errors.New("dummy error")
	}

	fd, err := os.Create(filePath)
	assert.Nil(t, err)
	fileBytes := []byte("sample content3")
	fd.Write(fileBytes)
	fd.Close()

	_, err = fileReadWithRetry(filePath)
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), dummyError)
}
