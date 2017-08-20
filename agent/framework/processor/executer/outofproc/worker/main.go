package main

import (
	"fmt"
	"os"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/channel"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/messaging"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/plugin"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

const (
	defaultCommandTimeoutMax = 172800 * time.Second
)

var channelCreator = func(log log.T, mode channel.Mode, documentID string) (channel.Channel, error, bool) {
	return channel.CreateFileChannel(log, mode, documentID)
}

var pluginRunner = func(
	context context.T,
	plugins []model.PluginState,
	resChan chan contracts.PluginResult,
	cancelFlag task.CancelFlag,
) {
	runpluginutil.RunPlugins(context, plugins, executer.PluginRegistry, resChan, cancelFlag)
	//make sure to signal the client that job complete
	close(resChan)
}

//TODO revisit this, is plugin entitled to use appconfig?
//rule of thumb is, do not trigger extra file operation or other intricate dependencies during this setup, make it light weight
func createContext(name string) context.T {
	// initialize appconfig, use default config
	config := appconfig.DefaultConfig()
	// intialiez logger, use the default seelog config logger
	logger := log.WithContext(name)
	//use process as context name
	return context.Default(logger, config)
}

func main() {
	//TODO revisit this, do not create CW diagnostic logger
	args := os.Args

	name, channelName, err := messaging.ParseArgv(args)
	if err != nil {
		fmt.Println("failed to parse argv: ", err)
		os.Exit(1)
	}
	ctx := createContext(name)
	log := ctx.Log()
	//initialize PluginRegistry
	executer.PluginRegistry = plugin.RegisteredWorkerPlugins(ctx)
	//create channel from the given handle identifier by master
	ipc, err, _ := channelCreator(ctx.Log(), channel.ModeWorker, channelName)
	if err != nil {
		log.Errorf("failed to create channel: %v", err)
		os.Exit(1)
	}
	//TODO add command timeout
	stopTimer := make(chan bool)
	pipeline := messaging.NewWorkerBackend(ctx, pluginRunner)
	//TODO wait for sigterm or send fail message to the channel?
	if err := messaging.Messaging(ctx.Log(), ipc, pipeline, stopTimer); err != nil {
		log.Errorf("messaging worker encountered error: %v", err)
		//If messaging broke, there's nothing worker process can do, exit immediately
		os.Exit(1)
	}
}
