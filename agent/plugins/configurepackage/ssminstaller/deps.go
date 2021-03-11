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
// permissions and limitations under the License.

package ssminstaller

import (
	"encoding/json"
	"io/ioutil"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/docmanager"
	"github.com/aws/amazon-ssm-agent/agent/framework/docparser"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/basicexecuter"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// dependency on action execution
type execDep interface {
	ParseDocument(context context.T, documentRaw []byte, orchestrationDir string, s3Bucket string, s3KeyPrefix string, messageID string, documentID string, defaultWorkingDirectory string) (pluginsInfo []contracts.PluginState, err error)
	ExecuteDocument(context context.T, pluginInput []contracts.PluginState, documentID string, documentCreatedDate string, orchestrationDirectory string) (pluginOutputs map[string]*contracts.PluginResult)
}

type execDepImp struct {
}

func (m *execDepImp) ParseDocument(context context.T, documentRaw []byte, orchestrationDir string, s3Bucket string, s3KeyPrefix string, messageID string, documentID string, defaultWorkingDirectory string) (pluginsInfo []contracts.PluginState, err error) {
	parserInfo := docparser.DocumentParserInfo{
		OrchestrationDir:  orchestrationDir,
		S3Bucket:          s3Bucket,
		S3Prefix:          s3KeyPrefix,
		MessageId:         messageID,
		DocumentId:        documentID,
		DefaultWorkingDir: defaultWorkingDirectory,
	}

	var docContent docparser.DocContent
	err = json.Unmarshal(documentRaw, &docContent)
	if err != nil {
		return
	}
	// TODO Add parameters
	return docContent.ParseDocument(context, contracts.DocumentInfo{}, parserInfo, nil)
}

func (m *execDepImp) ExecuteDocument(context context.T, pluginInput []contracts.PluginState, documentID string, documentCreatedDate string, orchestrationDirectory string) (pluginOutputs map[string]*contracts.PluginResult) {
	log := context.Log()
	log.Debugf("Running subcommand")
	exe := basicexecuter.NewBasicExecuter(context)
	docState := contracts.DocumentState{
		DocumentInformation: contracts.DocumentInfo{
			DocumentID: documentID,
		},
		IOConfig: contracts.IOConfiguration{
			OrchestrationDirectory: orchestrationDirectory,
		},
		InstancePluginsInformation: pluginInput,
	}
	//specify the subdocument's bookkeeping location
	docStore := executer.NewDocumentFileStore(documentID, appconfig.DefaultLocationOfCurrent, &docState, docmanager.NewDocumentFileMgr(context, appconfig.DefaultDataStorePath, appconfig.DefaultDocumentRootDirName, appconfig.DefaultLocationOfState))
	cancelFlag := task.NewChanneledCancelFlag()
	resChan := exe.Run(cancelFlag, &docStore)

	for res := range resChan {
		//basicExecuter can guarantee result order, however outofproc Executer cannot
		if res.LastPlugin == "" {
			pluginOutputs = res.PluginResults
			break
		}
	}
	return
}

// dependency on filesystem and os utility functions
type fileSysDep interface {
	Exists(filePath string) bool
	ReadFile(filename string) ([]byte, error)
}

type fileSysDepImp struct{}

func (fileSysDepImp) Exists(filePath string) bool {
	return fileutil.Exists(filePath)
}

func (fileSysDepImp) ReadFile(filename string) ([]byte, error) {
	return ioutil.ReadFile(filename)
}
