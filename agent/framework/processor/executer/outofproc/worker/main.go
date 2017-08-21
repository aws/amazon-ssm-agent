package main

import (
	"os"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/channel"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/messaging"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/proc"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/plugin"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

const (
	defaultCommandTimeoutMax = 172800 * time.Second
)

var pluginRunner = func(
	context context.T,
	plugins []model.PluginState,
	resChan chan contracts.PluginResult,
	cancelFlag task.CancelFlag,
) {
	runpluginutil.RunPlugins(context, plugins, plugin.RegisteredWorkerPlugins(context), resChan, cancelFlag)
	//make sure to signal the client that job complete
	close(resChan)
}

//TODO revisit this, is plugin entitled to use appconfig?
//TODO add log level to args
//rule of thumb is, do not trigger extra file operation or other intricate dependencies during this setup, make it light weight
func initialize(args []string) (context.T, string, error) {
	// intialize a light weight logger, use the default seelog config logger
	logger := log.Logger()
	// initialize appconfig, use default config
	config := appconfig.DefaultConfig()
	logger.Debugf("parsing args: %v", args)
	name, channelName, err := proc.ParseArgv(args)
	if err != nil {
		logger.Errorf("failed to parse argv: %v", err)
	}
	//use process as context name
	return context.Default(logger, config).With("[" + name + "]"), channelName, err
}

func main() {
	var err error
	var logger log.T
	args := os.Args
	ctx, channelName, err := initialize(args)
	logger = ctx.Log()
	defer func() {
		//ensure logs are flushed
		logger.Flush()
		logger.Close()
		time.Sleep(1 * time.Second)
		if err != nil {
			os.Exit(1)
		}
	}()
	if err != nil {
		logger.Errorf("document worker failed to initialize, exit")
		return
	}
	logger.Infof("document: %v worker started", channelName)
	//create channel from the given handle identifier by master
	ipc, err, _ := channel.CreateFileChannel(logger, channel.ModeWorker, channelName)
	if err != nil {
		logger.Errorf("failed to create channel: %v", err)
		return
	}
	//TODO add command timeout
	stopTimer := make(chan bool)
	pipeline := messaging.NewWorkerBackend(ctx, pluginRunner)
	//TODO wait for sigterm or send fail message to the channel?
	if err = messaging.Messaging(ctx.Log(), ipc, pipeline, stopTimer); err != nil {
		logger.Errorf("messaging worker encountered error: %v", err)
		//If ipc messaging broke, there's nothing worker process can do, exit immediately
		return
	}
	logger.Info("document worker closed")
}
