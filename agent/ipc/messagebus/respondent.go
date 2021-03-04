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

//Package messagebus logic to send message and get reply over IPC
package messagebus

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/common/channel"
	"github.com/aws/amazon-ssm-agent/common/message"
	_ "go.nanomsg.org/mangos/v3/transport/ipc"
)

// IMessageBus is the interface for process the core agent broadcast request
type IMessageBus interface {
	ProcessHealthRequest()
	ProcessTerminationRequest()
	RebootRequestChannel() chan bool
}

// MessageBus contains the ipc channel to communicate to core agent.
// It contains a reboot request channel that agent listens to
type MessageBus struct {
	context            context.T
	healthChannel      channel.IChannel
	terminationChannel channel.IChannel
	rebootRequest      chan bool
}

// NewMessageBus creates a new instance of MessageBus
func NewMessageBus(context context.T) *MessageBus {
	log := context.Log()
	identity := context.Identity()
	channelCreator := channel.GetChannelCreator(log, context.AppConfig(), identity)
	return &MessageBus{
		context:            context,
		healthChannel:      channelCreator(log, identity),
		terminationChannel: channelCreator(log, identity),
		rebootRequest:      make(chan bool, 1),
	}
}

// ProcessHealthRequest handles the health requests from core agent
// and process the relies on the HealthPing to determine if worker is still running
func (bus *MessageBus) ProcessHealthRequest() {
	log := bus.context.Log()
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Process health request panic: %v", r)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()
	var err error
	var msg []byte

	defer func() {
		if bus.healthChannel.IsConnect() {

			if err = bus.healthChannel.Close(); err != nil {
				bus.context.Log().Errorf("failed to close ipc channel: %v", err)
			}
		}
	}()

	for {
		if !bus.healthChannel.IsConnect() {
			if err = bus.dialToCoreAgentChannel(message.GetWorkerHealthRequest, message.GetWorkerHealthChannel); err != nil {
				// This happens when worker started before core agent is
				// In practise, it should never happen
				log.Errorf("failed to listen to Core Agent broadcast channel: %s", err.Error())
				time.Sleep(time.Duration(bus.context.AppConfig().Ssm.HealthFrequencyMinutes) * time.Minute)
			} else {
				break
			}
		}
	}

	log.Infof("Start to listen to Core Agent health channel")
	for {
		var request *message.Message
		if msg, err = bus.healthChannel.Recv(); err != nil {
			log.Errorf("cannot recv: %s", err.Error())
			continue
		}
		log.Debugf("Received Core Agent health request %s", string(msg))

		if err = json.Unmarshal(msg, &request); err != nil {
			log.Errorf("failed to unmarshal message: %s", err.Error())
			continue
		}
		log.Debugf("unmarshal health request: %v", request)
		if request.Topic == message.GetWorkerHealthRequest {

			var result *message.Message
			if result, err = message.CreateHealthResult(
				appconfig.SSMAgentWorkerName,
				message.LongRunning,
				os.Getpid()); err != nil {
				log.Errorf("failed to create health message: %s", err.Error())
			}

			log.Debugf("Sending health response %+v", result)
			if err = bus.healthChannel.Send(result); err != nil {
				log.Errorf("failed to send health response: %s", err.Error())
				continue
			}
		} else {
			log.Infof("Received invalid message, %s", request.Topic)
		}
	}
}

// ProcessTerminationRequest handles the termination requests from core agent
// CoreAgent sends termination message when itself is stopping, Worker use it to decide if itself should be terminated
func (bus *MessageBus) ProcessTerminationRequest() {
	log := bus.context.Log()
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Process termination request panic: %v", r)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()
	var err error
	var msg []byte
	defer func() {
		if bus.terminationChannel.IsConnect() {

			if err = bus.terminationChannel.Close(); err != nil {
				bus.context.Log().Errorf("failed to close ipc channel: %v", err)
			}
		}
	}()

	for {
		if !bus.terminationChannel.IsConnect() {
			if err = bus.dialToCoreAgentChannel(message.TerminateWorkerRequest, message.TerminationWorkerChannel); err != nil {
				// This happens when worker started before core agent is
				// In practise, it should never happen
				log.Errorf("failed to listen to Core Agent broadcast channel: %s", err.Error())
				time.Sleep(time.Duration(bus.context.AppConfig().Ssm.HealthFrequencyMinutes) * time.Minute)
			} else {
				break
			}
		}
	}

	log.Infof("Start to listen to Core Agent termination channel")
	for {
		var request *message.Message
		if msg, err = bus.terminationChannel.Recv(); err != nil {
			log.Errorf("cannot recv: %s", err.Error())
			continue
		}
		log.Infof("Received Core Agent termination request %s", string(msg))
		if err = json.Unmarshal(msg, &request); err != nil {
			log.Errorf("failed to unmarshal message: %s", err.Error())
			continue
		}
		log.Debugf("unmarshal health request: %v", request)

		if request.Topic == message.TerminateWorkerRequest {
			log.Infof("Received Core Agent termination signal, terminating %s", appconfig.SSMAgentWorkerName)

			var result *message.Message
			if result, err = message.CreateTerminateWorkerResult(
				appconfig.SSMAgentWorkerName,
				message.LongRunning,
				os.Getpid(),
				true); err != nil {
				log.Errorf("failed to create health message: %s", err.Error())
			}

			if err = bus.terminationChannel.Send(result); err != nil {
				log.Errorf("failed to send termination response: %s", err.Error())
				continue
			}

			// terminating ssm-agent-worker
			bus.rebootRequest <- true
			break
		} else {
			log.Infof("Received invalid message, %s", request.Topic)
		}
	}
}

// RebootRequestChannel returns the reboot request channel
func (bus *MessageBus) RebootRequestChannel() chan bool {
	return bus.rebootRequest
}

func (bus *MessageBus) dialToCoreAgentChannel(topic message.TopicType, address string) error {
	var err error

	bus.context.Log().Infof("Dial to Core Agent broadcast channel")

	switch topic {
	case message.GetWorkerHealthRequest:
		if err = bus.healthChannel.Initialize("respondent"); err != nil {
			return fmt.Errorf("can't get new respondent socket: %s", err.Error())
		}
		if err = bus.healthChannel.Dial(address); err != nil {
			return fmt.Errorf("can't dial on respondent socket: %s", err.Error())
		}

		return nil
	case message.TerminateWorkerRequest:
		if err = bus.terminationChannel.Initialize("respondent"); err != nil {
			return fmt.Errorf("can't get new respondent socket: %s", err.Error())
		}
		if err = bus.terminationChannel.Dial(address); err != nil {
			return fmt.Errorf("can't dial on respondent socket: %s", err.Error())
		}

		return nil
	default:
		return fmt.Errorf("unknown topic type: %s", topic)
	}
}
