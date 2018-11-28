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

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/channel"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/messaging"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/proc"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/plugin"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/log/ssmlog"
	"github.com/aws/amazon-ssm-agent/agent/task"
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

// initialize populates session worker information.
//rule of thumb is, do not trigger extra file operation or other intricate dependencies during this setup, make it light weight
func initialize(args []string) (context.T, string, error) {
	// intialize a light weight logger, use the default seelog config logger
	logger := ssmlog.SSMLogger(false)

	// initialize appconfig, use default config
	config := appconfig.DefaultConfig()

	logger.Debugf("Session worker parse args: %v", args)
	channelName, _, err := proc.ParseArgv(args)
	if err != nil {
		logger.Errorf("Failed to parse argv: %v", err)
	}

	//use argsVal1 as context name which is either channelName or dataChannelId
	return context.Default(logger, config).With(defaultSessionWorkerContextName).With("[" + channelName + "]"),
		channelName,
		err
}

// SessionWorker runs as independent worker process when invoked by master agent process and is responsible for running session plugins
func main() {
	args := os.Args

	context, channelName, err := initialize(args)
	log := context.Log()
	//ensure logs are flushed
	defer log.Close()
	if err != nil {
		log.Errorf("Session worker failed to initialize: %s", err)
		return
	}

	createFileChannelAndExecutePlugin(context, channelName)
	log.Info("Session worker closed")
}

// TODO Add interface for worker
// createFileChannelAndExecutePlugin creates file channel using channel name
// and initiates communication between master agent process and session worker process
func createFileChannelAndExecutePlugin(context context.T, channelName string) {
	log := context.Log()
	log.Infof("document: %v worker started", channelName)
	//create channel from the given handle identifier by master
	ipc, err, _ := channel.CreateFileChannel(log, channel.ModeWorker, channelName)
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
