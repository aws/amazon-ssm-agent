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
	"github.com/aws/amazon-ssm-agent/agent/docparser"
	"github.com/aws/amazon-ssm-agent/agent/framework/docmanager"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/go-yaml/yaml"
)

type ExecDocument interface {
	ParseDocument(log log.T, documentRaw []byte, orchestrationDir string,
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
func (exec ExecDocumentImpl) ParseDocument(log log.T, documentRaw []byte, orchestrationDir string,
	s3Bucket string, s3KeyPrefix string, messageID string, documentID string, defaultWorkingDirectory string,
	params map[string]interface{}) (pluginsInfo []contracts.PluginState, err error) {
	docContent := docparser.DocContent{}
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

	pluginsInfo, err = docContent.ParseDocument(log, contracts.DocumentInfo{}, parserInfo, params)
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
	//specify the sub-document's bookkeeping location
	instanceID, err := instance.InstanceID()
	if err != nil {
		log.Error("failed to load instance id")
		return resultChannels, err
	}
	docStore := executer.NewDocumentFileStore(context, documentID, instanceID, appconfig.DefaultLocationOfCurrent,
		&docState, docmanager.NewDocumentFileMgr(appconfig.DefaultDataStorePath, appconfig.DefaultDocumentRootDirName, appconfig.DefaultLocationOfState))
	cancelFlag := task.NewChanneledCancelFlag()
	resultChannels = exec.DocExecutor.Run(cancelFlag, &docStore)

	return resultChannels, nil
}
