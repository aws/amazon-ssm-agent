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

// Package document implements the document related functionality for executecommand
package document

import (
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/docparser"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/remoteresource"
	"github.com/go-yaml/yaml"

	"encoding/json"
	"errors"
)

const (
	DocumentTypeCommand = "Command"
)

type ExecCommand interface {
	ParseDocument(log log.T, resource remoteresource.ResourceInfo, documentRaw []byte, orchestrationDir string, s3Bucket string, s3KeyPrefix string, messageID string, documentID string, defaultWorkingDirectory string, params map[string]interface{}) (pluginsInfo []model.PluginState, err error)
	//ExecuteDocument() (pluginOutputs map[string]*contracts.PluginResult)
}

type ExecCommandImpl struct{}

// ParseDocument parses the remote document obtained to a format that the executer can use.
// This function is also responsible for all the validation of document and parameters
func (doc ExecCommandImpl) ParseDocument(log log.T, resource remoteresource.ResourceInfo, documentRaw []byte, orchestrationDir string, s3Bucket string, s3KeyPrefix string, messageID string, documentID string, defaultWorkingDirectory string, params map[string]interface{}) (pluginsInfo []model.PluginState, err error) {
	var docContent contracts.DocumentContent
	if resource.ResourceExtension == remoteresource.YAMLExtension {
		if err := yaml.Unmarshal(documentRaw, &docContent); err != nil {
			log.Error("Unmarshalling YAML remote resource document failed. Please make sure the document is in the right format")
			return pluginsInfo, err
		}

	} else if resource.ResourceExtension == remoteresource.JSONExtension {
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
