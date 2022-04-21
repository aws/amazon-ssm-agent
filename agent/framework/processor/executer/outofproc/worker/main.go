package main

import (
	"os"
	"runtime/debug"
	"time"

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
	defaultCommandTimeoutMax = 172800 * time.Second
	defaultWorkerContextName = "[ssm-document-worker]"
)

var pluginRunner = func(
	context context.T,
	docState contracts.DocumentState,
	resChan chan contracts.PluginResult,
	cancelFlag task.CancelFlag,
) {
	runpluginutil.RunPlugins(context, docState.InstancePluginsInformation, docState.IOConfig, runpluginutil.SSMPluginRegistry, resChan, cancelFlag)
	//make sure to signal the client that job complete
	close(resChan)
}

func main() {
	logger := ssmlog.SSMLogger(false)
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("document worker panic: %v", err)
			logger.Errorf("Stacktrace:\n%s", debug.Stack())
		}

		logger.Flush()
		logger.Close()
	}()

	logger.Infof("ssm-document-worker - %v", version.String())
	cfg, agentIdentity, channelName, err := proc.InitializeWorkerDependencies(logger, os.Args)
	if err != nil {
		logger.Errorf("document worker failed to initialize with error %v", err)
		return
	}

	ctx := context.Default(logger, *cfg, agentIdentity).With(defaultWorkerContextName).With("[" + channelName + "]")
	logger = ctx.Log()

	logger.Infof("document: %v worker started", channelName)
	//create channel from the given handle identifier by master
	ipc, err, _ := filewatcherbasedipc.CreateFileWatcherChannel(logger, agentIdentity, filewatcherbasedipc.ModeWorker, channelName, false)
	if err != nil {
		logger.Errorf("failed to create channel: %v", err)
		logger.Close()
		return
	}
	//initialize PluginRegistry
	runpluginutil.SSMPluginRegistry = plugin.RegisteredWorkerPlugins(ctx)

	//TODO add command timeout
	stopTimer := make(chan bool)
	pipeline := messaging.NewWorkerBackend(ctx, pluginRunner)
	//TODO wait for sigterm or send fail message to the channel?
	if err = messaging.Messaging(logger, ipc, pipeline, stopTimer); err != nil {
		logger.Errorf("messaging worker encountered error: %v", err)
		//If ipc messaging broke, there's nothing worker process can do, exit immediately
		logger.Close()
		return
	}
	logger.Info("document worker closed")
	//ensure logs are flushed
	logger.Close()
	//TODO figure out s3 aync problem
	//TODO figure out why defer main doesnt work on windows
	if err != nil {
		os.Exit(1)
	}
}
