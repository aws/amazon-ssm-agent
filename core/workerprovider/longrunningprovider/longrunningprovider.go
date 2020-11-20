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

// Package longrunningprovider provides an interface to start/stop a worker process for long-running tasks.
package longrunningprovider

import (
	"os"
	"runtime/debug"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/common/message"
	"github.com/aws/amazon-ssm-agent/core/app/context"
	reboot "github.com/aws/amazon-ssm-agent/core/app/reboot/model"
	"github.com/aws/amazon-ssm-agent/core/executor"
	"github.com/aws/amazon-ssm-agent/core/ipc/messagebus"
	"github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider/discover"
	"github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider/provider"
)

// WorkerContainers contains list of running workers, it starts/terminates/watches workers
type WorkerContainer struct {
	sync.Mutex
	context           context.ICoreAgentContext
	stopWorkerMonitor chan bool
	workerProvider    provider.IProvider
	workerDiscover    discover.IDiscover
	messageBus        messagebus.IMessageBus
}

// IContainer is the interface for starting/terminating/watching worker
type IContainer interface {
	Start()
	Monitor()
	Stop(reboot.StopType)
}

var getPpid = os.Getppid
var sleep = time.Sleep

// NewWorkerContainer returns worker container
func NewWorkerContainer(
	context context.ICoreAgentContext,
	messageBus messagebus.IMessageBus) *WorkerContainer {
	ctx := context.With("[LongRunningWorkerContainer]")

	processExecutor := executor.NewProcessExecutor(ctx.Log())
	workerDiscover := discover.NewWorkerDiscover(ctx.Log())
	workerProvider := provider.NewWorkerProvider(ctx, processExecutor)

	container := WorkerContainer{
		context:           ctx,
		stopWorkerMonitor: make(chan bool, 1),
		workerProvider:    workerProvider,
		workerDiscover:    workerDiscover,
		messageBus:        messageBus,
	}

	return &container
}

// Start identifies and loads the worker configs and starts long running workers
func (container *WorkerContainer) Start() {
	logger := container.context.Log()
	defer func() {
		if msg := recover(); msg != nil {
			logger.Errorf("worker container start run panic: %v", msg)
			logger.Errorf("%s: %s", msg, debug.Stack())
		}
	}()

	var pingResults []*message.Message
	// find worker configs
	configs := container.workerDiscover.FindWorkerConfigs()

	// request workers to ping back health
	healthRequest := message.CreateHealthRequest()
	pingResults, err := container.messageBus.SendSurveyMessage(healthRequest)

	if err != nil {
		logger.Errorf("failed to request worker health ping %s", err)
	}

	container.workerProvider.Start(configs, pingResults)
}

// Monitor watches worker process, restarts the worker when receive worker exist signal
func (container *WorkerContainer) Monitor() {
	logger := container.context.Log()
	defer func() {
		if msg := recover(); msg != nil {
			logger.Errorf("worker monitor run panic: %v", msg)
			logger.Errorf("%s: %s", msg, debug.Stack())
		}
	}()

	sleepInterval := container.context.AppConfig().Agent.LongRunningWorkerMonitorIntervalSeconds

	logger.Infof(
		"Monitor long running worker health every %v seconds",
		container.context.AppConfig().Agent.LongRunningWorkerMonitorIntervalSeconds)

	for {
		timer := time.NewTimer(time.Duration(sleepInterval) * time.Second)
		select {
		case <-container.stopWorkerMonitor:
			logger.Infof("Receiving stop signal, stop worker monitor")
			timer.Stop()
			return

		case <-timer.C:
			logger.Debugf("Verifying worker health")
			// find worker configs
			configs := container.workerDiscover.FindWorkerConfigs()

			// request workers to ping back health
			healthRequest := message.CreateHealthRequest()
			pingResults, err := container.messageBus.SendSurveyMessage(healthRequest)
			if err != nil {
				logger.Errorf("failed to request worker health ping %s", err)
			}

			container.workerProvider.Monitor(configs, pingResults)
		}
	}
}

// Stop stops monitor and stop all the running workers
func (container *WorkerContainer) Stop(stopType reboot.StopType) {
	logger := container.context.Log()

	container.stopWorkerMonitor <- true

	request := message.CreateTerminateWorkerRequest()
	if results, err := container.messageBus.SendSurveyMessage(request); err != nil {
		logger.Errorf("failed to broadcast core termination signal %s", err)
	} else {
		for _, result := range results {
			logger.Infof("Received worker termination result, %+v", result)
		}
	}

	container.messageBus.Stop()
	sleep(reboot.HardStopTimeout)

	// If agent parent is 0, force terminate and clean up all worker processes
	if getPpid() == 0 {
		container.workerProvider.KillAllWorkerProcesses()
	}
}
