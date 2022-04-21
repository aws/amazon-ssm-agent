// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package main implements a separate worker which is used to execute requests from session manager.
package main

import (
	"os"
	"runtime/debug"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/messaging"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/proc"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/plugin"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/log/ssmlog"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/amazon-ssm-agent/common/filewatcherbasedipc"
)

const (
	defaultSessionWorkerContextName = "[ssm-session-worker]"
)

var sessionPluginRunner = func(
	context context.T,
	docState contracts.DocumentState,
	resChan chan contracts.PluginResult,
	cancelFlag task.CancelFlag,
) {
	runpluginutil.RunPlugins(context,
		docState.InstancePluginsInformation,
		docState.IOConfig,
		runpluginutil.SSMPluginRegistry,
		resChan,
		cancelFlag)

	//make sure to signal the client that job complete
	close(resChan)
}

// SessionWorker runs as independent worker process when invoked by master agent process and is responsible for running session plugins
func main() {
	logger := ssmlog.SSMLogger(false)
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("session worker panic: %v", err)
			logger.Errorf("Stacktrace:\n%s", debug.Stack())
		}

		logger.Flush()
		logger.Close()
	}()

	logger.Infof("ssm-session-worker - %v", version.String())
	cfg, agentIdentity, channelName, err := proc.InitializeWorkerDependencies(logger, os.Args)
	if err != nil {
		logger.Errorf("session worker failed to initialize with error %v", err)
		return
	}

	ctx := context.Default(logger, *cfg, agentIdentity).With(defaultSessionWorkerContextName).With("[" + channelName + "]")
	logger = ctx.Log()

	createFileChannelAndExecutePlugin(ctx, channelName)
	logger.Info("Session worker closed")
}

// TODO Add interface for worker
// createFileChannelAndExecutePlugin creates file channel using channel name
// and initiates communication between master agent process and session worker process
func createFileChannelAndExecutePlugin(context context.T, channelName string) {
	log := context.Log()
	log.Infof("document: %v worker started", channelName)
	//create channel from the given handle identifier by master
	ipc, err, _ := filewatcherbasedipc.CreateFileWatcherChannel(log, context.Identity(), filewatcherbasedipc.ModeWorker, channelName, false)
	if err != nil {
		log.Errorf("failed to create channel: %v", err)
		return
	}

	//initialize SessionPluginRegistry
	runpluginutil.SSMPluginRegistry = plugin.RegisteredSessionWorkerPlugins()

	//TODO add command timeout
	stopTimer := make(chan bool)
	pipeline := messaging.NewWorkerBackend(context, sessionPluginRunner)
	//TODO wait for sigterm or send fail message to the channel?
	if err = messaging.Messaging(log, ipc, pipeline, stopTimer); err != nil {
		log.Errorf("messaging worker encountered error: %v", err)
		//If ipc messaging broke, there's nothing session worker process can do, exit immediately
		return
	}
}
