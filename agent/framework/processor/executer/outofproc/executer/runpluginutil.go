package main

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

//runplugin stubbing
func runPlugins(
	context context.T,
	plugins []model.PluginState,
	pluginRegistry runpluginutil.PluginRegistry,
	resChan chan contracts.PluginResult,
	cancelFlag task.CancelFlag,
) (pluginOutputs map[string]*contracts.PluginResult) {

	return map[string]*contracts.PluginResult{}
}
