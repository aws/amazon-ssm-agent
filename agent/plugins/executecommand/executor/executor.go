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

// Package executor implements the document and script related functionality for executecommand
package executor

import (
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/docparser"
	"github.com/aws/amazon-ssm-agent/agent/executers"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/filemanager"
	"github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/remoteresource"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/task"

	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"

	"github.com/go-yaml/yaml"
)

const (
	DocumentTypeCommand = "Command"
)

type ExecCommand interface {
	ParseDocument(log log.T, extension string, documentRaw []byte, orchestrationDir string, s3Bucket string, s3KeyPrefix string, messageID string,
		documentID string, defaultWorkingDirectory string, params map[string]interface{}) (pluginsInfo []model.PluginState, err error)
	ExecuteDocument(context context.T, pluginInput []model.PluginState, documentID string, documentCreatedDate string, output *contracts.PluginOutput)
	ExecuteScript(log log.T, fileName string, arg []string, executionTimeout int, out *contracts.PluginOutput)
}

type ExecCommandImpl struct {
	ScriptExecutor executers.T
	DocExecutor    func(context context.T) executer.Executer
}

// ParseDocument parses the remote document obtained to a format that the executor can use.
// This function is also responsible for all the validation of document and replacement of parameters
func (doc ExecCommandImpl) ParseDocument(log log.T, extension string, documentRaw []byte, orchestrationDir string,
	s3Bucket string, s3KeyPrefix string, messageID string, documentID string, defaultWorkingDirectory string,
	params map[string]interface{}) (pluginsInfo []model.PluginState, err error) {
	var docContent contracts.DocumentContent
	if extension == remoteresource.YAMLExtension {
		if err := yaml.Unmarshal(documentRaw, &docContent); err != nil {
			log.Error("Unmarshalling YAML remote resource document failed. Please make sure the document is in the right format")
			return pluginsInfo, err
		}

	} else if extension == remoteresource.JSONExtension {
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
func (doc ExecCommandImpl) ExecuteDocument(context context.T, pluginInput []model.PluginState, documentID string,
	documentCreatedDate string, output *contracts.PluginOutput) {
	log := context.Log()
	var pluginOutput map[string]*contracts.PluginResult
	log.Info("Running sub-document")

	// Using  basicexecuter.NewBasicExecuter() to create an object to Run the documents.
	exe := doc.DocExecutor(context)
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
	resultChannels := exe.Run(cancelFlag, &docStore)

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
			output.AppendInfof(log, "%v output:", pluginOut.PluginName)
			output.AppendInfof(log, "%v", pluginOut.StandardOutput)
		}
		if pluginOut.StandardError != "" {
			// separating the append so that the output is on a new line
			//TODO How does this appear in the output?
			output.AppendErrorf(log, "%v errors:", pluginOut.PluginName)
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

// ExecuteScript executes the scripts pulled down from remote resources
func (exec ExecCommandImpl) ExecuteScript(log log.T, fileName string, args []string, executionTimeout int, out *contracts.PluginOutput) {
	// populate the command Name and command args based on the platform type
	commandName, shellArgs := populateCommand(fileName)
	cancelFlag := task.NewChanneledCancelFlag()
	log.Debug("Local destination path - ", fileName)
	if err := filemanager.SetPermission(fileName, appconfig.ReadWriteExecuteAccess); err != nil {
		out.MarkAsFailed(log, fmt.Errorf("Failed to execute script - %v. Error - %v", path.Base(fileName), err))
	}
	commandArgs := appendArgs(shellArgs, args, fileName)

	log.Infof("Running command - %v %v", commandName, commandArgs)
	stdout, stderr, exitCode, errs := exec.ScriptExecutor.Execute(log, filepath.Dir(fileName), "", "", cancelFlag, executionTimeout, commandName, commandArgs)

	// Set output status
	out.ExitCode = exitCode
	status := pluginutil.GetStatus(out.ExitCode, cancelFlag)

	if len(errs) > 0 {
		for _, err := range errs {
			if status != contracts.ResultStatusCancelled &&
				status != contracts.ResultStatusTimedOut &&
				status != contracts.ResultStatusSuccessAndReboot {
				out.MarkAsFailed(log, fmt.Errorf("failed to run commands: %v", err))
			}
		}
	}
	if bytesOut, err := ioutil.ReadAll(stdout); err != nil {
		log.Error(err)
	} else {
		out.AppendInfo(log, string(bytesOut))
	}
	if bytesErr, err := ioutil.ReadAll(stderr); err != nil {
		log.Error(err)
	} else {
		out.AppendError(log, string(bytesErr))
		if string(bytesErr) != "" {
			out.MarkAsFailed(log, errors.New("Error encountered while running script"))
		}
	}

	log.Debug("Status - ", status)
	out.Status = contracts.MergeResultStatus(out.Status, status)
	return
}

var instance instanceInfo = &instanceInfoImp{}

// system represents the dependency for platform
type instanceInfo interface {
	InstanceID() (string, error)
	Region() (string, error)
}

type instanceInfoImp struct{}

// InstanceID wraps platform InstanceID
func (instanceInfoImp) InstanceID() (string, error) { return platform.InstanceID() }

// Region wraps platform Region
func (instanceInfoImp) Region() (string, error) { return platform.Region() }
