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
	"os"
	"runtime/debug"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/channel"
	"github.com/aws/amazon-ssm-agent/common/message"
	_ "go.nanomsg.org/mangos/v3/transport/ipc"
)

const (
	recvErrSleepTime = 30 * time.Second
	maxRecvErrCount  = 5
)

// shutdownPreparer records that a machine shutdown is underway so in-flight work can treat its own
// interruption as a reboot rather than a failure. A registered hook so this IPC layer does not
// depend on the framework above it; no-op by default.
var shutdownPreparer = func(log.T) {}

// RegisterShutdownPreparer installs the hook invoked when core signals a machine shutdown.
func RegisterShutdownPreparer(preparer func(log.T)) {
	if preparer != nil {
		shutdownPreparer = preparer
	}
}

// IMessageBus is the interface for process the core agent broadcast request
type IMessageBus interface {
	ProcessHealthRequest()
	ProcessTerminationRequest()
	ProcessPrepareShutdownRequest()
	GetTerminationRequestChan() chan bool
	GetTerminationChannelConnectedChan() chan bool
}

// MessageBus contains the ipc channel to communicate to core agent.
// It contains a reboot request channel that agent listens to
type MessageBus struct {
	context                     context.T
	healthChannel               channel.IChannel
	terminationChannel          channel.IChannel
	preShutdownChannel          channel.IChannel
	terminationRequestChannel   chan bool
	terminationChannelConnected chan bool
	sleepFunc                   func(time.Duration)
}

// NewMessageBus creates a new instance of MessageBus
func NewMessageBus(context context.T) *MessageBus {
	log := context.Log()
	identity := context.Identity()
	channelCreator := channel.GetChannelCreator(log, context.AppConfig(), identity)
	return &MessageBus{
		context:                     context,
		healthChannel:               channelCreator(log, identity),
		terminationChannel:          channelCreator(log, identity),
		preShutdownChannel:          channelCreator(log, identity),
		terminationRequestChannel:   make(chan bool, 1),
		terminationChannelConnected: make(chan bool, 1),
		sleepFunc:                   time.Sleep,
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
		if bus.healthChannel.IsChannelInitialized() {
			if err = bus.healthChannel.Close(); err != nil {
				bus.context.Log().Errorf("failed to close health channel: %v", err)
			}
		}
	}()

	for !bus.healthChannel.IsDialSuccessful() {
		if err = bus.dialToCoreAgentChannel(message.GetWorkerHealthRequest, message.GetWorkerHealthChannel); err != nil {
			// This happens when worker started before core agent is
			//   and when the amazon-ssm-agent is terminated milliseconds after the ssm-agent-worker has been started
			log.Errorf("failed to listen to Core Agent health channel: %s", err.Error())
			bus.sleepFunc(time.Duration(bus.context.AppConfig().Ssm.HealthFrequencyMinutes) * time.Minute)
		}
	}

	log.Infof("Start to listen to Core Agent health channel")
	errRecvCount := 0

	for {
		var request *message.Message
		if msg, err = bus.healthChannel.Recv(); err != nil {
			errRecvCount++
			log.Errorf("failed to receive from health channel: %s", err.Error())
			if errRecvCount >= maxRecvErrCount {
				// Agent core will still consider worker healthy as the ssm-agent-worker exists in the system process tree
				log.Errorf("failed to receive from agent core health channel %v times. Stopping health ipc listener", errRecvCount)
				return
			}

			log.Debugf("Retrying receive from core agent health channel in %v seconds", recvErrSleepTime.Seconds())
			bus.sleepFunc(recvErrSleepTime)
			continue
		}

		errRecvCount = 0
		log.Debugf("Received health request from core agent %s", string(msg))

		if err = json.Unmarshal(msg, &request); err != nil {
			log.Errorf("failed to unmarshal message: %s", err.Error())
			continue
		}

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
			log.Warnf("Received invalid message on health channel, %s", request.Topic)
		}
	}
}

// ProcessPrepareShutdownRequest handles machine-shutdown notifications. Unlike
// ProcessTerminationRequest it NEVER exits the worker, which must stay alive to make in-flight work
// reboot-safe. Listens on its own channel: sharing the termination channel would put two surveyors
// on one reply stream, so the acknowledgement could never be read back.
func (bus *MessageBus) ProcessPrepareShutdownRequest() {
	log := bus.context.Log()
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Process prepare-shutdown request panic: %v", r)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()
	var err error
	var msg []byte

	defer func() {
		if bus.preShutdownChannel.IsChannelInitialized() {
			if err = bus.preShutdownChannel.Close(); err != nil {
				log.Errorf("failed to close prepare-shutdown channel: %v", err)
			}
		}
	}()

	for !bus.preShutdownChannel.IsDialSuccessful() {
		if err = bus.dialToCoreAgentChannel(message.PrepareShutdownRequest, message.PrepareShutdownChannel); err != nil {
			log.Errorf("failed to listen to prepare-shutdown channel: %s", err.Error())
			bus.sleepFunc(time.Duration(bus.context.AppConfig().Ssm.HealthFrequencyMinutes) * time.Minute)
		}
	}

	log.Infof("Start to listen to Core Agent prepare-shutdown channel")
	errRecvCount := 0

	for {
		var request *message.Message
		if msg, err = bus.preShutdownChannel.Recv(); err != nil {
			errRecvCount++
			log.Errorf("failed to receive from prepare-shutdown channel: %s", err.Error())
			if errRecvCount >= maxRecvErrCount {
				log.Errorf("failed to receive from prepare-shutdown channel %v times. Stopping listener", errRecvCount)
				return
			}
			bus.sleepFunc(recvErrSleepTime)
			continue
		}

		errRecvCount = 0
		if err = json.Unmarshal(msg, &request); err != nil {
			log.Errorf("failed to unmarshal message: %s", err.Error())
			continue
		}

		if request.Topic != message.PrepareShutdownRequest {
			log.Warnf("Received invalid message on prepare-shutdown channel, %s", request.Topic)
			continue
		}

		log.Infof("Received prepare-shutdown signal from core agent, making in-flight work reboot-safe")
		shutdownPreparer(log)

		var result *message.Message
		if result, err = message.CreatePrepareShutdownResult(
			appconfig.SSMAgentWorkerName,
			message.LongRunning,
			os.Getpid()); err != nil {
			log.Errorf("failed to create prepare-shutdown response: %s", err.Error())
			continue
		}
		if err = bus.preShutdownChannel.Send(result); err != nil {
			// Preparation already happened, so a lost ack costs only telemetry.
			log.Errorf("failed to send prepare-shutdown response: %s", err.Error())
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
		if bus.terminationChannel.IsChannelInitialized() {
			if err = bus.terminationChannel.Close(); err != nil {
				bus.context.Log().Errorf("failed to close termination channel: %v", err)
			}
		}
	}()

	for !bus.terminationChannel.IsDialSuccessful() {
		if err = bus.dialToCoreAgentChannel(message.TerminateWorkerRequest, message.TerminationWorkerChannel); err != nil {
			// This happens when worker started before core agent is
			//   and when the amazon-ssm-agent is terminated milliseconds after the ssm-agent-worker has been started
			log.Errorf("failed to listen to termination channel: %s", err.Error())
			bus.sleepFunc(time.Duration(bus.context.AppConfig().Ssm.HealthFrequencyMinutes) * time.Minute)
		}
	}

	log.Infof("Start to listen to Core Agent termination channel")
	bus.terminationChannelConnected <- true
	errRecvCount := 0

	for {
		var request *message.Message
		if msg, err = bus.terminationChannel.Recv(); err != nil {
			log.Errorf("cannot receive message from core agent termination channel: %s", err.Error())
			errRecvCount++
			if errRecvCount >= maxRecvErrCount {
				// Consider communication channel to agent core to be broken
				// When ssm-agent-worker exits agent-ssm-agent (agent core) creates new worker with updated communication channel
				log.Errorf("failed to receive message from core agent termination channel %v times. Initiating worker shutdown", errRecvCount)
				// Unblock main() function to allow agent worker to exit
				bus.terminationRequestChannel <- true
				close(bus.terminationRequestChannel)
				// exit termination channel goroutine
				break
			}

			log.Debugf("Retrying receive from core agent termination channel in %v seconds", recvErrSleepTime.Seconds())
			bus.sleepFunc(recvErrSleepTime)
			continue
		}

		log.Infof("Received termination message from core agent %s", string(msg))
		errRecvCount = 0
		if err = json.Unmarshal(msg, &request); err != nil {
			// Incoming message is dropped and not retried
			log.Errorf("failed to unmarshal message: %s", err.Error())
			continue
		}

		if request.Topic == message.TerminateWorkerRequest {
			log.Debugf("Received termination signal from core agent, terminating %s", appconfig.SSMAgentWorkerName)

			var result *message.Message
			if result, err = message.CreateTerminateWorkerResult(
				appconfig.SSMAgentWorkerName,
				message.LongRunning,
				os.Getpid(),
				true); err != nil {
				// Response message is dropped and not retried
				log.Errorf("failed to create termination response: %s", err.Error())
			}

			if err = bus.terminationChannel.Send(result); err != nil {
				// Response message is dropped and not retried
				log.Errorf("failed to send termination response: %s", err.Error())
				continue
			}

			// terminating ssm-agent-worker
			bus.terminationRequestChannel <- true
			break
		} else {
			log.Warnf("Received invalid message on termination channel, %s", request.Topic)
		}
	}
}

func (bus *MessageBus) dialToCoreAgentChannel(topic message.TopicType, address string) error {
	var err error

	bus.context.Log().Infof("Dial to Core Agent broadcast channel")

	switch topic {
	case message.GetWorkerHealthRequest:
		if err = bus.healthChannel.Initialize("respondent"); err != nil {
			_ = bus.healthChannel.Close()
			return fmt.Errorf("can't get new respondent socket: %s", err.Error())
		}
		if err = bus.healthChannel.Dial(address); err != nil {
			_ = bus.healthChannel.Close()
			return fmt.Errorf("can't dial on respondent socket: %s", err.Error())
		}

		return nil
	case message.TerminateWorkerRequest:
		if err = bus.terminationChannel.Initialize("respondent"); err != nil {
			_ = bus.terminationChannel.Close()
			return fmt.Errorf("can't get new respondent socket: %s", err.Error())
		}
		if err = bus.terminationChannel.Dial(address); err != nil {
			_ = bus.terminationChannel.Close()
			return fmt.Errorf("can't dial on respondent socket: %s", err.Error())
		}

		return nil
	case message.PrepareShutdownRequest:
		if err = bus.preShutdownChannel.Initialize("respondent"); err != nil {
			_ = bus.preShutdownChannel.Close()
			return fmt.Errorf("can't get new respondent socket: %s", err.Error())
		}
		if err = bus.preShutdownChannel.Dial(address); err != nil {
			_ = bus.preShutdownChannel.Close()
			return fmt.Errorf("can't dial on respondent socket: %s", err.Error())
		}

		return nil
	default:
		return fmt.Errorf("unknown topic type: %s", topic)
	}
}

// GetTerminationRequestChan returns the terminate request channel
func (bus *MessageBus) GetTerminationRequestChan() chan bool {
	return bus.terminationRequestChannel
}

// GetTerminationChannelConnectedChan returns the channel notifying when termination channel is connected
func (bus *MessageBus) GetTerminationChannelConnectedChan() chan bool {
	return bus.terminationChannelConnected
}
