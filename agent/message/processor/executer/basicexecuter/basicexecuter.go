// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package executer provides interfaces as document execution logic
package basicexecuter

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/message/processor/executer"
	"github.com/aws/amazon-ssm-agent/agent/message/processor/executer/plugin"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

//TODO currently BasicExecuter.Run() is not idempotent, we should make it so in future
// BasicExecuter is a thin wrapper over runPlugins().
type BasicExecuter struct {
	//TODO add cancelFlag attribute
	statusChan chan contracts.PluginResult
	ctx        context.T
}

var pluginRunner = func(context context.T,
	docStore executer.DocumentStore,
	resChan chan contracts.PluginResult,
	cancelFlag task.CancelFlag) {
	defer func() {
		if msg := recover(); msg != nil {
			context.Log().Errorf("Executer run panic: %v", msg)
		}
	}()
	docState := docStore.Load()
	outputs := runPlugins(context, docState.DocumentInformation.MessageID, "", docState.InstancePluginsInformation, plugin.RegisteredWorkerPlugins(context), resChan, cancelFlag)
	pluginOutputContent, _ := jsonutil.Marshal(outputs)
	context.Log().Debugf("Plugin outputs %v", jsonutil.Indent(pluginOutputContent))
	//TODO DocInfo is a service oriented object, and may not be persisted by Executer
	// aggregate the document information from plugin outputs
	docState.DocumentInformation = docmanager.DocumentResultAggregator(context.Log(), "", outputs)
	// persist the docState object
	docStore.Save()
	//sender close the channel
	close(resChan)
}

// NewBasicExecuter returns a pointer that impl the Executer interface
// using a pointer so that it can be shared among multiple threads(go-routines)
func NewBasicExecuter(context context.T) executer.Executer {
	return &BasicExecuter{
		ctx: context,
	}
}

func (e *BasicExecuter) Run(
	cancelFlag task.CancelFlag,
	docStore executer.DocumentStore) chan contracts.PluginResult {

	log := e.ctx.Log()
	docState := docStore.Load()
	nPlugins := len(docState.InstancePluginsInformation)
	// we're creating a buffered channel according to the number of plugins the document has
	e.statusChan = make(chan contracts.PluginResult, nPlugins)

	log.Debug("Running plugins...")
	go pluginRunner(e.ctx, docStore, e.statusChan, cancelFlag)
	return e.statusChan
}
