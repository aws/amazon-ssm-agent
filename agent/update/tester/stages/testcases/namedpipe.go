// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
// permissions and limitations under the License
//
// package testcases contains test cases from all testStages
package testcases

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	testCommon "github.com/aws/amazon-ssm-agent/agent/update/tester/common"
	"github.com/aws/amazon-ssm-agent/common/channel"
	channelutil "github.com/aws/amazon-ssm-agent/common/channel/utils"
	"github.com/aws/amazon-ssm-agent/common/message"
	"go.nanomsg.org/mangos/v3"
	_ "go.nanomsg.org/mangos/v3/transport/ipc"
)

var (
	channelCreator = testCommon.CreateIPCChannelIfNotExists

	createChannel func(log.T) channel.IChannel
)

// NamedPipeTestCase represents the test case testing the named pipe on customer instance
type NamedPipeTestCase struct {
	listenChannel               channel.IChannel
	dialChannel                 channel.IChannel
	log                         log.T
	expectedOutput              string
	TestSetupCleanupEventHandle func()
}

// Initialize initializes values needed for this test case
func (l *NamedPipeTestCase) Initialize(logger log.T) {
	l.log = logger.WithContext("[Test" + l.GetTestCaseName() + "]")
	if createChannel == nil {
		createChannel = channel.NewNamedPipeChannel
	}
	l.listenChannel = createChannel(l.log)
	l.dialChannel = createChannel(l.log)
	l.expectedOutput = "reply"
}

// RegisterTestCase executes the named pipe test case
// creates listen go routine and tries to dial pipe for communication
func (l *NamedPipeTestCase) ExecuteTestCase() testCommon.TestOutput {
	l.log.Info("named pipe test case started")
	testOutput := testCommon.TestOutput{
		Err:    errors.New("dialing was unsuccessful"),
		Result: testCommon.TestCaseFail,
	}

	// creates the ipc folder
	if err := channelCreator(); err != nil {
		testOutput.Err = err
		return testOutput
	}

	go func() {
		testOutput.Err = l.listenPipe()
		l.dialChannel.Close() // should kill the dial
	}()
	l.dialPipe()

	if testOutput.Err == nil {
		testOutput.Result = testCommon.TestCasePass
	}
	return testOutput
}

// GetTestCaseName gets the test case name
func (l *NamedPipeTestCase) GetTestCaseName() string {
	return testCommon.NamedPipeTestCaseName
}

// listenPipe creates named pipe and waits for connection
func (l *NamedPipeTestCase) listenPipe() (err error) {
	defer func() {
		if msg := recover(); msg != nil {
			err = errors.New(fmt.Sprintf("listen pipe panicked: %v", msg))
			return
		}
		l.log.Info("listen pipe thread ended")
	}()
	var msg []byte
	l.log.Info("listen pipe thread started")

	if err = l.listenChannel.Initialize(channelutil.Surveyor); err != nil {
		return errors.New(fmt.Sprintf("listen pipe initialization failed: %v", err))
	}
	if err = l.listenChannel.Listen(testCommon.TestIPCChannel); err != nil {
		return errors.New(fmt.Sprintf("listening to pipe failed: %v", err))
	}
	if err = l.listenChannel.SetOption(mangos.OptionSurveyTime, time.Second*2); err != nil {
		return errors.New(fmt.Sprintf("setting up option for listening failed: %v", err))
	}

	var reply *message.Message
	requestMsg := &message.Message{
		SchemaVersion: 1,
		Topic:         "TestSurveyTopic",
		Payload:       []byte("request"),
	}
	for {
		time.Sleep(300 * time.Millisecond)
		if err = l.listenChannel.Send(requestMsg); err != nil {
			return errors.New(fmt.Sprintf("sending failed %v", err))
		}
		for {
			if msg, err = l.listenChannel.Recv(); err != nil {
				break
			}
			if err = json.Unmarshal(msg, &reply); err != nil {
				return errors.New(fmt.Sprintf("failed to unmarshal message in listen pipe thread: %v %v", err, string(msg)))
			}
			l.log.Debugf("received message in listen pipe thread %+v", reply)
			if string(reply.Payload) == l.expectedOutput {
				l.log.Info("received expected message in listening thread")
				return nil
			}
			return errors.New("received reply was not expected")
		}
	}
}

// dialPipe connects to the named pipe created by listen go routing
func (l *NamedPipeTestCase) dialPipe() {
	defer func() {
		l.log.Info("dial pipe thread ended")
	}()
	l.log.Info("dial pipe thread started")
	var msg []byte
	var err error

	if err = l.dialChannel.Initialize(channelutil.Respondent); err != nil {
		l.log.Errorf("dial pipe initialization failed: %v", err)
		return
	}
	time.Sleep(200 * time.Millisecond)
	if err = l.dialChannel.Dial(testCommon.TestIPCChannel); err != nil {
		l.log.Errorf("dial pipe failed: %v", err)
		return
	}

	var request *message.Message
	replyMsg := &message.Message{
		SchemaVersion: 1,
		Topic:         "TestRespondentTopic",
		Payload:       []byte(l.expectedOutput),
	}
	for iterationNo := 1; iterationNo <= 5; iterationNo++ {
		if msg, err = l.dialChannel.Recv(); err != nil {
			continue
		}
		if err = json.Unmarshal(msg, &request); err != nil {
			l.log.Error(errors.New(fmt.Sprintf("failed to unmarshal message: %v %v", err, string(msg))))
			continue
		}
		l.log.Debugf("received message in dial pipe %+v", request)
		if err = l.dialChannel.Send(replyMsg); err != nil {
			l.log.Errorf("problem sending message: %v", err)
			return
		}
	}
}

// CleanupTestCase cleans up the test case
func (l *NamedPipeTestCase) CleanupTestCase() {
	l.TestSetupCleanupEventHandle = func() {
		l.dialChannel.Close()
		l.listenChannel.Close()
	}
	l.log.Info("named pipe test case cleanup")
}

// GetTestSetUpCleanupEventHandle helps us to clean resources at the end of testSuite
func (l *NamedPipeTestCase) GetTestSetUpCleanupEventHandle() func() {
	return l.TestSetupCleanupEventHandle
}
