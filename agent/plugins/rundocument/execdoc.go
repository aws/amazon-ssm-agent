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
	"errors"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/docparser"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/go-yaml/yaml"
)

type ExecDocument interface {
	ParseDocument(log log.T, extension string, documentRaw []byte, orchestrationDir string,
		s3Bucket string, s3KeyPrefix string, messageID string, documentID string, defaultWorkingDirectory string,
		params map[string]interface{}) (pluginsInfo []model.PluginState, err error)
	ExecuteDocument(context context.T, pluginInput []model.PluginState, documentID string,
		documentCreatedDate string, output *contracts.PluginOutput)
}

type ExecDocumentImpl struct {
	DocExecutor executer.Executer
}

// ParseDocument parses the remote document obtained to a format that the executor can use.
// This function is also responsible for all the validation of document and replacement of parameters
func (exec ExecDocumentImpl) ParseDocument(log log.T, extension string, documentRaw []byte, orchestrationDir string,
	s3Bucket string, s3KeyPrefix string, messageID string, documentID string, defaultWorkingDirectory string,
	params map[string]interface{}) (pluginsInfo []model.PluginState, err error) {
	var docContent contracts.DocumentContent
	if extension == yamlExtension {
		if err := yaml.Unmarshal(documentRaw, &docContent); err != nil {
			log.Error("Unmarshalling YAML remote resource document failed. Please make sure the document is in the right format")
			return pluginsInfo, err
		}

	} else if extension == jsonExtension {
		if err := json.Unmarshal(documentRaw, &docContent); err != nil {
			log.Error("Unmarshalling JSON remote resource document failed. Please make sure the document is in the right format")
			return pluginsInfo, err
		}
	} else {
		return pluginsInfo, errors.New("Extension type for documents is not supported")
	}
	parserInfo := docparser.DocumentParserInfo{
		OrchestrationDir:  orchestrationDir,
		S3Bucket:          s3Bucket,
		S3Prefix:          s3KeyPrefix,
		MessageId:         messageID,
		DocumentId:        documentID,
		DefaultWorkingDir: defaultWorkingDirectory,
	}
	pluginsInfo, err = docparser.ParseDocument(log, &docContent, parserInfo, params)

	log.Debug("Parsed document - ", docContent)
	log.Debug("Plugins Info - ", pluginsInfo)
	return
}

// ExecuteDocument is responsible to execute the sub-documents that are created or downloaded by the executeCommand plugin
func (exec ExecDocumentImpl) ExecuteDocument(context context.T, pluginInput []model.PluginState, documentID string,
	documentCreatedDate string, output *contracts.PluginOutput) {
	log := context.Log()
	var pluginOutput map[string]*contracts.PluginResult
	log.Info("Running sub-document")

	docState := model.DocumentState{
		DocumentInformation: model.DocumentInfo{
			DocumentID: documentID,
		},
		InstancePluginsInformation: pluginInput,
	}
	//specify the sub-document's bookkeeping location
	instanceID, err := instance.InstanceID()
	if err != nil {
		log.Error("failed to load instance id")
		return
	}
	docStore := executer.NewDocumentFileStore(context, documentID, instanceID, appconfig.DefaultLocationOfCurrent, &docState)
	cancelFlag := task.NewChanneledCancelFlag()
	resultChannels := exec.DocExecutor.Run(cancelFlag, &docStore)

	for res := range resultChannels {
		if res.LastPlugin == "" {
			pluginOutput = res.PluginResults
			break
		}
	}
	if pluginOutput == nil {
		output.MarkAsFailed(log, errors.New("No output obtained from executing document"))
	}
	for _, pluginOut := range pluginOutput {
		if pluginOut.StandardOutput != "" {
			// separating the append so that the output is on a new line
			output.AppendInfof(log, "%v", pluginOut.StandardOutput)
		}
		if pluginOut.StandardError != "" {
			// separating the append so that the output is on a new line
			output.AppendErrorf(log, "%v", pluginOut.StandardError)
		}
		if pluginOut.Error != nil {
			output.MarkAsFailed(log, pluginOut.Error)
		} else {
			output.MarkAsSucceeded()
		}
		output.Status = contracts.MergeResultStatus(output.Status, pluginOut.Status)
	}
	return
}
