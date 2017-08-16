package main

import (
	"os"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/channel"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/plugin"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

var channelCreator channel.ChannelCreator
var ctx context.T

var pluginRunner = func(
	context context.T,
	plugins []model.PluginState,
	resChan chan contracts.PluginResult,
	cancelFlag task.CancelFlag,
) {
	runpluginutil.RunPlugins(context, plugins, executer.PluginRegistry, resChan, cancelFlag)
	//signal the client that job complete
	close(resChan)
}

func main() {
	log := logger.Logger()
	args := os.Args
	if len(args) < 2 {
		log.Error("not enough argument input to the executable")
		os.Exit(1)
	}
	// initialize appconfig
	var config appconfig.SsmagentConfig
	config, err := appconfig.Config(false)
	if err != nil {
		log.Errorf("Could not load config file: %v", err)
		return
	}
	name := args[0]
	handle := string(args[1])
	//use process as context name
	ctx = context.Default(log, config).With(name)
	//initialize PluginRegistry
	executer.PluginRegistry = plugin.RegisteredWorkerPlugins(ctx)
	//client connect to the connection object which is already set up by server
	ipc := channelCreator(channel.ModeWorker)
	if err = ipc.Open(handle); err != nil {
		log.Errorf("failed to connect to ")
		os.Exit(1)
	}
	pipeline, stopChan := outofproc.NewWorkerBackend(ctx, pluginRunner)
	if err := outofproc.Messaging(ctx.Log(), ipc, pipeline, stopChan); err != nil {
		log.Errorf("messaging worker encountered error: %v", err)
		os.Exit(1)
	}
}
