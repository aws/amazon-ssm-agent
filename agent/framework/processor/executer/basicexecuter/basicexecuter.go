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

// Package basicexecuter provides interfaces as document execution logic
package basicexecuter

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"

	"sync"

	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

//TODO currently BasicExecuter.Run() is not idempotent, we should make it so in future
// BasicExecuter is a thin wrapper over runPlugins().
type BasicExecuter struct {
	resChan chan contracts.DocumentResult
	ctx     context.T
}

var pluginRunner = func(context context.T,
	docState contracts.DocumentState,
	resChan chan contracts.PluginResult,
	cancelFlag task.CancelFlag) (pluginOutputs map[string]*contracts.PluginResult) {
	return runpluginutil.RunPlugins(context, docState.InstancePluginsInformation, docState.IOConfig, runpluginutil.SSMPluginRegistry, resChan, cancelFlag)

}

func run(context context.T,
	docStore executer.DocumentStore,
	resChan chan contracts.DocumentResult,
	cancelFlag task.CancelFlag) {
	defer func() {
		if msg := recover(); msg != nil {
			context.Log().Errorf("Executer run panic: %v", msg)
		}
	}()
	docState := docStore.Load()
	//document information summary
	messageID := docState.DocumentInformation.MessageID
	associationID := docState.DocumentInformation.AssociationID
	nPlugins := len(docState.InstancePluginsInformation)
	documentName := docState.DocumentInformation.DocumentName
	documentVersion := docState.DocumentInformation.DocumentVersion
	//status channel for plugins update
	statusChan := make(chan contracts.PluginResult)
	var wg sync.WaitGroup
	wg.Add(1)
	//The go-routine to listen to individual plugin update
	go func(state *contracts.DocumentState) {
		defer func() {
			if msg := recover(); msg != nil {
				context.Log().Errorf("Executer listener panic: %v", msg)
			}
			wg.Done()
		}()
		results := make(map[string]*contracts.PluginResult)
		for res := range statusChan {
			results[res.PluginID] = &res
			//TODO decompose this function to return only Status
			status, _, _ := contracts.DocumentResultAggregator(context.Log(), res.PluginID, results)
			docResult := contracts.DocumentResult{
				Status:          status,
				PluginResults:   results,
				LastPlugin:      res.PluginID,
				AssociationID:   associationID,
				MessageID:       messageID,
				NPlugins:        nPlugins,
				DocumentName:    documentName,
				DocumentVersion: documentVersion,
			}
			resChan <- docResult
			contracts.UpdateDocState(&docResult, state)
		}
	}(&docState)

	outputs := pluginRunner(context, docState, statusChan, cancelFlag)
	close(statusChan)
	//make sure the launched go routine has finshed before sending the final response
	wg.Wait()
	pluginOutputContent, _ := jsonutil.Marshal(outputs)
	context.Log().Debugf("Plugin outputs %v", jsonutil.Indent(pluginOutputContent))
	//send DocLevel response
	status, _, _ := contracts.DocumentResultAggregator(context.Log(), "", outputs)
	result := contracts.DocumentResult{
		Status:          status,
		PluginResults:   outputs,
		LastPlugin:      "",
		MessageID:       messageID,
		AssociationID:   associationID,
		NPlugins:        nPlugins,
		DocumentName:    documentName,
		DocumentVersion: documentVersion,
	}
	resChan <- result
	docState.DocumentInformation.DocumentStatus = status
	// persist the docState object
	docStore.Save(docState)
	//sender close the channel
	close(resChan)
}

// NewBasicExecuter returns a pointer that impl the Executer interface
// using a pointer so that it can be shared among multiple threads(go-routines)
func NewBasicExecuter(context context.T) *BasicExecuter {
	return &BasicExecuter{
		ctx: context.With("[BasicExecuter]"),
	}
}

func (e *BasicExecuter) Run(
	cancelFlag task.CancelFlag,
	docStore executer.DocumentStore) chan contracts.DocumentResult {

	log := e.ctx.Log()
	docState := docStore.Load()
	nPlugins := len(docState.InstancePluginsInformation)
	// we're creating a buffered channel according to the number of plugins the document has
	e.resChan = make(chan contracts.DocumentResult, nPlugins)

	log.Debug("Running plugins...")
	go run(e.ctx, docStore, e.resChan, cancelFlag)
	return e.resChan
}
