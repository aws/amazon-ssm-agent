package runutil

import (
	"encoding/json"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/statemanager/model"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// SendResponse is used to send response on plugin completion.
// If pluginID is empty it will send responses of all plugins.
// If pluginID is specified, response will be sent of that particular plugin.
type SendResponse func(messageID string, pluginID string, results map[string]*contracts.PluginResult)

func NoReply(messageID string, pluginID string, results map[string]*contracts.PluginResult) {}

// SendDocumentLevelResponse is used to send status response before plugin begins
type SendDocumentLevelResponse func(messageID string, resultStatus contracts.ResultStatus, documentTraceOutput string)

// UpdateAssociation updates association status
type UpdateAssociation func(log log.T, documentID string, pluginOutputs map[string]*contracts.PluginResult, totalNumberOfPlugins int)

func NoUpdate(log log.T, documentID string, pluginOutputs map[string]*contracts.PluginResult, totalNumberOfPlugins int) {
}

// T is the interface type for plugins.
type T interface {
	Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, subDocumentRunner Runner) contracts.PluginResult
}

// PluginRegistry stores a set of plugins (both worker and long running plugins), indexed by ID.
type PluginRegistry map[string]T

type Runner struct {
	PluginRunner func(
		context context.T,
		documentID string,
		plugins map[string]model.PluginState,
		pluginRegistry PluginRegistry,
		sendReply SendResponse,
		updateAssoc UpdateAssociation,
		cancelFlag task.CancelFlag,
	) (pluginOutputs map[string]*contracts.PluginResult)
	Plugins     PluginRegistry
	SendReply   SendResponse
	UpdateAssoc UpdateAssociation
	CancelFlag  task.CancelFlag
}

func (r *Runner) ParseDocument(context context.T, documentRaw []byte, orchestrationDir string, s3Bucket string, s3KeyPrefix string, messageID string, documentID string, defaultWorkingDirectory string) (pluginsInfo map[string]model.PluginState, err error) {
	var docContent contracts.DocumentContent
	err = json.Unmarshal(documentRaw, &docContent)
	pluginConfigurations := make(map[string]*contracts.Configuration)
	for pluginName, pluginConfig := range docContent.RuntimeConfig {
		config := contracts.Configuration{
			Settings:                pluginConfig.Settings,
			Properties:              pluginConfig.Properties,
			OutputS3BucketName:      s3Bucket,
			OutputS3KeyPrefix:       fileutil.BuildS3Path(s3KeyPrefix, pluginName),
			OrchestrationDirectory:  fileutil.BuildPath(orchestrationDir, pluginName),
			MessageId:               messageID,
			BookKeepingFileName:     documentID,
			DefaultWorkingDirectory: defaultWorkingDirectory,
		}
		pluginConfigurations[pluginName] = &config
	}

	//initialize plugin states
	pluginsInfo = make(map[string]model.PluginState)

	for key, value := range pluginConfigurations {
		var plugin model.PluginState
		plugin.Configuration = *value
		plugin.HasExecuted = false
		pluginsInfo[key] = plugin
	}

	return
}

func (r *Runner) ExecuteDocument(context context.T, pluginInput map[string]model.PluginState, documentID string) (pluginOutputs map[string]*contracts.PluginResult) {
	log := context.Log()
	for name, _ := range pluginInput {
		log.Debugf("SubDocument type %v", name)
	}

	return r.PluginRunner(context, documentID, pluginInput, r.Plugins, r.SendReply, r.UpdateAssoc, r.CancelFlag)
}
