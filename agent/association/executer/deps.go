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

// Package executer allows execute Pending association and InProgress association
package executer

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager"
	docModel "github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/framework/engine"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

var bookkeepingSvc = bookkeepingImp{}
var pluginExecution = pluginExecutionImp{}

// bookkeepingService represents the dependency for docmanager
type bookkeepingService interface {
	GetDocumentInfo(log log.T, documentID, instanceID, locationFolder string) docModel.DocumentInfo
	PersistDocumentInfo(log log.T, docInfo docModel.DocumentInfo, documentID, instanceID, locationFolder string)
	MoveCommandState(log log.T, documentID, instanceID, srcLocationFolder, dstLocationFolder string)
}

type bookkeepingImp struct{}

// GetDocumentInfo wraps docmanager GetDocumentInfo
func (bookkeepingImp) GetDocumentInfo(log log.T, documentID, instanceID, locationFolder string) docModel.DocumentInfo {
	return docmanager.GetDocumentInfo(log, documentID, instanceID, locationFolder)
}

// PersistDocumentInfo wraps docmanager PersistDocumentInfo
func (bookkeepingImp) PersistDocumentInfo(log log.T, docInfo docModel.DocumentInfo, documentID, instanceID, locationFolder string) {
	docmanager.PersistDocumentInfo(log, docInfo, documentID, instanceID, locationFolder)
}

// MoveDocumentState wraps docmanager MoveDocumentState
func (bookkeepingImp) MoveDocumentState(log log.T, documentID, instanceID, srcLocationFolder, dstLocationFolder string) {
	docmanager.MoveDocumentState(log, documentID, instanceID, srcLocationFolder, dstLocationFolder)
}

// pluginExecutionService represents the dependency for engine
type pluginExecutionService interface {
	RunPlugins(
		context context.T,
		associationID string,
		documentCreatedDate string,
		plugins []docModel.PluginState,
		pluginRegistry runpluginutil.PluginRegistry,
		sendReply runpluginutil.SendResponse,
		cancelFlag task.CancelFlag,
	) (pluginOutputs map[string]*contracts.PluginResult)
}

type pluginExecutionImp struct{}

// RunPlugins wraps engine RunPlugins
func (pluginExecutionImp) RunPlugins(
	context context.T,
	associationID string,
	documentCreatedDate string,
	plugins []docModel.PluginState,
	pluginRegistry runpluginutil.PluginRegistry,
	assocUpdate runpluginutil.UpdateAssociation,
	cancelFlag task.CancelFlag,
) (pluginOutputs map[string]*contracts.PluginResult) {
	return engine.RunPlugins(context, associationID, documentCreatedDate, plugins, pluginRegistry, nil, assocUpdate, cancelFlag)
}
