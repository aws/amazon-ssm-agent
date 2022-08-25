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
// permissions and limitations under the License.

// Package messagebus logic to send message and get reply over IPC
package messagebus

import (
	"encoding/json"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/aws/amazon-ssm-agent/common/channel"
	channelutil "github.com/aws/amazon-ssm-agent/common/channel/utils"
	"github.com/aws/amazon-ssm-agent/common/message"
	"github.com/aws/amazon-ssm-agent/core/app/context"
	"go.nanomsg.org/mangos/v3"
	_ "go.nanomsg.org/mangos/v3/transport/ipc"
)

const (
	DefaultCreateChannelRetryIntervalSeconds = 5
)

// IMessageBus is the interface for sending out survey message and receiving the result
type IMessageBus interface {
	Start() error
	SendSurveyMessage(survey *message.Message) ([]*message.Message, error)
	Stop()
}

// MessageBus contains the ipc channel to communicate to core agent.
type MessageBus struct {
	context        context.ICoreAgentContext
	surveyChannels map[message.TopicType]channel.IChannel
}

// NewMessageBus creates a new instance of message bus
func NewMessageBus(context context.ICoreAgentContext) *MessageBus {
	log := context.Log()
	identity := context.Identity()
	channels := make(map[message.TopicType]channel.IChannel)
	channelCreator := channel.GetChannelCreator(log, *context.AppConfig(), identity)
	channels[message.GetWorkerHealthRequest] = channelCreator(log, identity)
	channels[message.TerminateWorkerRequest] = channelCreator(log, identity)

	return &MessageBus{
		context:        context.With("[MessageBus]"),
		surveyChannels: channels,
	}
}

// Start starts the health and terminate worker message channel
func (bus *MessageBus) Start() error {
	defer func() {
		if msg := recover(); msg != nil {
			bus.context.Log().Errorf("message bus start run panic: %v", msg)
			bus.context.Log().Errorf("%s: %s", msg, debug.Stack())
		}
	}()

	if err := bus.createMessageChannelWithRetry(message.GetWorkerHealthRequest); err != nil {
		return fmt.Errorf("failed to start health channel: %s", err)
	}
	if err := bus.createMessageChannelWithRetry(message.TerminateWorkerRequest); err != nil {
		return fmt.Errorf("failed to start termination channel: %s", err)
	}

	return nil
}

// SendSurveyMessage sends the health or termination survey message
func (bus *MessageBus) SendSurveyMessage(survey *message.Message) ([]*message.Message, error) {
	logger := bus.context.Log()
	defer func() {
		if msg := recover(); msg != nil {
			logger.Errorf("message bus send survey run panic: %v", msg)
			logger.Errorf("%s: %s", msg, debug.Stack())
		}
	}()

	logger.Debugf("Start survey %s", survey.Topic)
	if survey.Topic != message.GetWorkerHealthRequest && survey.Topic != message.TerminateWorkerRequest {
		return []*message.Message{}, fmt.Errorf("unsupported topic: %s", survey.Topic)
	}

	if surveyChannel, ok := bus.surveyChannels[survey.Topic]; !ok || !surveyChannel.IsConnect() {
		if err := bus.createMessageChannelWithRetry(survey.Topic); err != nil {
			return []*message.Message{}, err
		}
	}

	if err := bus.surveyChannels[survey.Topic].Send(survey); err != nil {
		return []*message.Message{}, fmt.Errorf("failed sending request: %s", err.Error())
	}

	var results []*message.Message
	for {
		var result *message.Message
		msg, err := bus.surveyChannels[survey.Topic].Recv()
		if err != nil {
			break
		}
		logger.Debugf("Received channel response:\"%s\" ", string(msg))

		if err = json.Unmarshal(msg, &result); err != nil {
			logger.Errorf("failed to deserialize the message %s", string(msg))
			continue
		}
		logger.Debugf("unmarshal channel response: %v", result)
		results = append(results, result)
	}

	logger.Debugf("Survey completed")
	return results, nil
}

// Stop stops the message channels
func (bus *MessageBus) Stop() {
	var err error
	for _, surveyChannel := range bus.surveyChannels {
		if surveyChannel != nil {

			if err = surveyChannel.Close(); err != nil {
				bus.context.Log().Errorf("failed to close ipc channel: %v", err)
			}
		}
	}
}

func (bus *MessageBus) createMessageChannelWithRetry(topic message.TopicType) error {
	var err error
	var address string

	switch topic {
	case message.GetWorkerHealthRequest:
		address = message.GetWorkerHealthChannel
	case message.TerminateWorkerRequest:
		address = message.TerminationWorkerChannel
	default:
		return fmt.Errorf("unknown topic type: %s", topic)
	}

	bus.context.Log().Debugf("Create channel %s for core/worker communication", address)

	for i := 0; i < 3; i++ {
		if err = bus.createMessageChannel(topic, address); err == nil {
			return nil
		}

		time.Sleep(DefaultCreateChannelRetryIntervalSeconds * time.Second)
	}

	return err
}

func (bus *MessageBus) createMessageChannel(topic message.TopicType, address string) error {
	var err error
	if err = bus.surveyChannels[topic].Initialize(channelutil.Surveyor); err != nil {
		return fmt.Errorf("failed to create new channel: %s, %v", address, err)
	}

	if err = bus.surveyChannels[topic].Listen(address); err != nil {
		return fmt.Errorf("failed to listen on the channel: %s, %v", address, err)
	}

	if err = bus.surveyChannels[topic].SetOption(mangos.OptionSurveyTime, time.Second*1); err != nil {
		return fmt.Errorf("setOption(): %v", err)
	}

	return nil
}
