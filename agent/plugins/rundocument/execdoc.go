// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License..

// Package rundocument implements the aws:runDocument plugin
package rundocument

import (
	"encoding/json"
	"path/filepath"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/docparser"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"gopkg.in/yaml.v2"
)

type ExecDocument interface {
	ParseDocument(context context.T, documentRaw []byte, orchestrationDir string,
		s3Bucket string, s3KeyPrefix string, messageID string, documentID string, defaultWorkingDirectory string,
		params map[string]interface{}) (pluginsInfo []contracts.PluginState, err error)
	ExecuteDocument(config contracts.Configuration, context context.T, pluginInput []contracts.PluginState, documentID string,
		documentCreatedDate string) (chan contracts.DocumentResult, error)
}

type ExecDocumentImpl struct {
	DocExecutor executer.Executer
}

// ParseDocument parses the remote document obtained to a format that the executor can use.
// This function is also responsible for all the validation of document and replacement of parameters
func (exec ExecDocumentImpl) ParseDocument(context context.T, documentRaw []byte, orchestrationDir string,
	s3Bucket string, s3KeyPrefix string, messageID string, documentID string, defaultWorkingDirectory string,
	params map[string]interface{}) (pluginsInfo []contracts.PluginState, err error) {
	log := context.Log()
	docContent := docparser.DocContent{
		InvokedPlugin: appconfig.PluginRunDocument,
	}
	if err := json.Unmarshal(documentRaw, &docContent); err != nil {
		if err := yaml.Unmarshal(documentRaw, &docContent); err != nil {
			log.Error("Unmarshaling remote resource document failed. Please make sure the document is in the correct JSON or YAML formal")
			return pluginsInfo, err
		}
	}
	parserInfo := docparser.DocumentParserInfo{
		OrchestrationDir:  orchestrationDir,
		S3Bucket:          s3Bucket,
		S3Prefix:          s3KeyPrefix,
		MessageId:         messageID,
		DocumentId:        documentID,
		DefaultWorkingDir: defaultWorkingDirectory,
	}

	pluginsInfo, err = docContent.ParseDocument(context, contracts.DocumentInfo{}, parserInfo, params)
	log.Debug("Parsed document - ", docContent)
	log.Debug("Plugins Info - ", pluginsInfo)
	return
}

// ExecuteDocument is responsible to execute the sub-documents that are created or downloaded by the executeCommand plugin
func (exec ExecDocumentImpl) ExecuteDocument(config contracts.Configuration, context context.T, pluginInput []contracts.PluginState, documentID string,
	documentCreatedDate string) (resultChannels chan contracts.DocumentResult, err error) {
	log := context.Log()
	log.Info("Running sub-document")

	// The full path of orchestrationDir should look like:
	// Linux: /var/lib/amazon/ssm/instance-id/document/orchestration/command-id/plugin-id
	// Windows: %PROGRAMDATA%\Amazon\SSM\InstanceData\instance-id\document\orchestration\command-id\plugin-id
	orchestrationDir := filepath.Join(config.OrchestrationDirectory, config.PluginID)

	docState := contracts.DocumentState{
		DocumentInformation: contracts.DocumentInfo{
			DocumentID: documentID,
		},
		IOConfig: contracts.IOConfiguration{
			OrchestrationDirectory: orchestrationDir,
			OutputS3BucketName:     "",
			OutputS3KeyPrefix:      "",
		},
		InstancePluginsInformation: pluginInput,
	}

	docStore := executer.NewDocumentFileStore(documentID, appconfig.DefaultLocationOfCurrent, &docState,
		NewNoOpDocumentMgr(context), true)
	cancelFlag := task.NewChanneledCancelFlag()
	resultChannels = exec.DocExecutor.Run(cancelFlag, &docStore)

	return resultChannels, nil
}

// Workaround to keep this plugin from overwriting the DocumentState of the top-level document.
//
// Up until version 3.0.732.0, this plugin always failed to write the DocumentState to file due to a
// path calculation bug.  In 3.0.732.0, the path calculation bug was fixed, and the plugin
// started overwriting the top-level document's DocumentState, which sometimes interfered with
// plugins in the top-level document that executed after this one.  NoOpDocumentMgr is meant
// to keep this plugin from overwriting the state of the top-level document.  However, it does
// not persist the DocumentState over a reboot, and it does not track the status (e.g. "pending",
// "current", "corrupt") of the document.  Longer-term, we may want to implement proper DocumentState
// bookkeeping for the documents executed by this plugin.
type NoOpDocumentMgr struct {
	context context.T
	state   contracts.DocumentState
}

func NewNoOpDocumentMgr(ctx context.T) *NoOpDocumentMgr {
	return &NoOpDocumentMgr{
		ctx,
		contracts.DocumentState{},
	}
}

func (m *NoOpDocumentMgr) MoveDocumentState(fileName, srcLocationFolder, dstLocationFolder string) {
	// No-op
	m.context.Log().Debugf("NoOpDocumentMgr.MoveDocumentState(%s, %s, %s)", fileName, srcLocationFolder, dstLocationFolder)
}

func (m *NoOpDocumentMgr) PersistDocumentState(fileName, locationFolder string, state contracts.DocumentState) {
	m.context.Log().Debugf("NoOpDocumentMgr.PersistDocumentState(%s, %s, %+v)", fileName, locationFolder, state)
	m.state = state
}

func (m *NoOpDocumentMgr) GetDocumentState(fileName, locationFolder string) contracts.DocumentState {
	m.context.Log().Debugf("NoOpDocumentMgr.GetDocumentState(%s, %s)", fileName, locationFolder)
	m.context.Log().Warn("NoOpDocumentMgr.GetDocumentState() called, consider using a persistent DocumentMgr")
	return m.state
}

func (m *NoOpDocumentMgr) RemoveDocumentState(fileName, locationFolder string) {
	// No-op
	m.context.Log().Debugf("NoOpDocumentMgr.RemoveDocumentState(%s, %s)", fileName, locationFolder)
}
