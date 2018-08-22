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
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docparser"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/filemanager"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/basicexecuter"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	ssmsvc "github.com/aws/amazon-ssm-agent/agent/ssm"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/aws/aws-sdk-go/service/ssm"

	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"strings"

	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/go-yaml/yaml"
)

const (
	executeCommandMaxDepth = 3 //Maximum depth of document execution
	jsonExtension          = ".json"
	yamlExtension          = ".yaml"

	SSMDocumentType = "SSMDocument"
	LocalPathType   = "LocalPath"

	downloadsDir = "downloads" //Directory under the orchestration directory where the downloaded resource resides

	FailExitCode = 1
	PassExitCode = 0
)

// NewPlugin returns a new instance of the plugin.
func NewPlugin() (*Plugin, error) {
	var plugin Plugin

	return &plugin, nil
}

// Plugin is the type for the aws:copyContent plugin.
type Plugin struct {
	filesys filemanager.FileSystem
	ssmSvc  ssmsvc.Service
	execDoc ExecDocument
}

// RunDocumentPluginInput is a struct that holds the parameters sent through send command
type RunDocumentPluginInput struct {
	contracts.PluginInput
	DocumentType       string      `json:"documentType"`
	DocumentPath       string      `json:"documentPath"`
	DocumentParameters interface{} `json:"documentParameters"`
}

// ExecutePluginDepth is the struct that is sent through to the sub-documents to maintain the depth of execution
type ExecutePluginDepth struct {
	executeCommandDepth int
}

// Execute runs multiple sets of commands and returns their outputs.
// res.Output will contain a slice of RunCommandPluginOutput.
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, output iohandler.IOHandler) {
	p.filesys = filemanager.FileSystemImpl{}
	p.ssmSvc = ssmsvc.NewService()
	exec := basicexecuter.NewBasicExecuter(context)
	p.execDoc = ExecDocumentImpl{
		DocExecutor: exec,
	}
	p.execute(context, config, cancelFlag, output)
}

func (p *Plugin) execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, output iohandler.IOHandler) {
	log := context.Log()
	log.Info("Plugin aws:runDocument started with configuration", config)

	if cancelFlag.ShutDown() {
		output.MarkAsShutdown()
	} else if cancelFlag.Canceled() {
		output.MarkAsCancelled()
	} else if input, err := parseAndValidateInput(config.Properties); err != nil {
		output.MarkAsFailed(err)
	} else {
		p.runDocument(context, input, config, output)
	}
}

// runCopyContent figures out the type of location, downloads the resource, saves it on disk and returns information required for it
func (p *Plugin) runDocument(context context.T, input *RunDocumentPluginInput, config contracts.Configuration, output iohandler.IOHandler) {

	log := context.Log()
	//Run aws:runDocument plugin
	log.Debug("Inside aws:runDocument function")
	var documentPath string
	var pluginsInfo []contracts.PluginState
	var err error
	//Set the depth of execution to be 1 for the first level execution
	execDepth := 1
	// Getting the current depth of execution and checking against maximum depth
	if config.Settings != nil {
		if settings, ok := config.Settings.(*ExecutePluginDepth); !ok {
			log.Error("Plugin setting is not of the right type")
			output.MarkAsFailed(errors.New("There was an error obtaining the depth of execution"))
			return
		} else {
			execDepth = settings.executeCommandDepth + 1
			if execDepth > executeCommandMaxDepth {
				output.MarkAsFailed(fmt.Errorf("Maximum depth for document execution exceeded. "+
					"Maximum depth permitted - %v and current depth - %v", executeCommandMaxDepth, execDepth))
				return
			}
		}
	}
	log.Info("Depth of execution - ", execDepth)

	if input.DocumentType == SSMDocumentType {
		if documentPath, err = p.downloadDocumentFromSSM(log, config, input); err != nil {
			output.MarkAsFailed(err)
		}
	} else {
		if filepath.IsAbs(input.DocumentPath) {
			documentPath = input.DocumentPath
		} else {
			orchestrationDir := strings.TrimSuffix(config.OrchestrationDirectory, config.PluginID)
			// The Document path is expected to have the name of the document
			documentPath = filepath.Join(orchestrationDir, downloadsDir, input.DocumentPath)
		}
	}
	if pluginsInfo, err = p.prepareDocumentForExecution(log, documentPath, config, input.DocumentParameters); err != nil {
		output.MarkAsFailed(fmt.Errorf("There was an error while preparing documents - %v", err.Error()))
		return
	}
	// Sending execution depth in Configuration.Settings to the sub-documents
	for i, plugins := range pluginsInfo {
		plugins.Configuration.Settings = &ExecutePluginDepth{executeCommandDepth: execDepth}
		pluginsInfo[i] = plugins
	}

	var resultsChannel chan contracts.DocumentResult
	var pluginOutput map[string]*contracts.PluginResult
	if resultsChannel, err = p.execDoc.ExecuteDocument(config, context, pluginsInfo, config.BookKeepingFileName, times.ToIso8601UTC(time.Now())); err != nil {
		output.MarkAsFailed(fmt.Errorf("There was an error while running documents - %v", err.Error()))
	}
	for res := range resultsChannel {
		if res.LastPlugin == "" {
			pluginOutput = res.PluginResults
			break
		}
	}
	if pluginOutput == nil {
		output.MarkAsFailed(errors.New("No output obtained from executing document"))
	}
	for _, pluginOut := range pluginOutput {
		if pluginOut.StandardOutput != "" {
			// separating the append so that the output is on a new line
			output.AppendInfof("%v", pluginOut.StandardOutput)
		}
		if pluginOut.StandardError != "" {
			// separating the append so that the output is on a new line
			output.AppendErrorf("%v", pluginOut.StandardError)
		}
		if pluginOut.Error != "" {
			output.MarkAsFailed(errors.New(pluginOut.Error))
		}
		output.SetStatus(contracts.MergeResultStatus(output.GetStatus(), pluginOut.Status))
	}
}

func (p *Plugin) downloadDocumentFromSSM(log log.T, config contracts.Configuration, input *RunDocumentPluginInput) (string, error) {
	var err error
	// Downloads folder for download path
	destination := filepath.Join(config.OrchestrationDirectory, downloadsDir)

	docName, docVersion := docparser.ParseDocumentNameAndVersion(input.DocumentPath)
	var docResponse *ssm.GetDocumentOutput
	if docResponse, err = p.ssmSvc.GetDocument(log, docName, docVersion); err != nil {
		log.Errorf("Unable to get ssm document. %v", err)
		return "", err
	}

	log.Debugf("Destination is %v ", destination)
	// create directory to download github resources
	if err = p.filesys.MakeDirs(destination); err != nil {
		log.Error("failed to create directory for github - ", err)
		return "", err
	}

	pathToFile := filepath.Join(destination, filepath.Base(docName)+jsonExtension)

	if err = p.filesys.WriteFile(pathToFile, *docResponse.Content); err != nil {
		log.Errorf("Error writing to file %v - %v", pathToFile, err)
		return "", err
	}
	return pathToFile, nil

}

// PrepareDocumentForExecution parses the raw content of the document, validates it and returns a PluginState that can be executed.
func (p *Plugin) prepareDocumentForExecution(log log.T, pathToFile string, config contracts.Configuration, params interface{}) (pluginsInfo []contracts.PluginState, err error) {
	parameters := make(map[string]interface{})
	if params != nil {
		switch params := params.(type) {
		case string:
			log.Debug("Document parameter type is String. Params to be unmarshaled - ", params)
			if err = json.Unmarshal([]byte(params), &parameters); err != nil {
				if erryaml := yaml.Unmarshal([]byte(params), &parameters); erryaml != nil {
					errs := fmt.Errorf("Unmarshalling document parameters failed. Please make sure the parameters are specified in the right format"+
						"JSON format error - %v, YAML format error - %v.", err, erryaml)
					return pluginsInfo, errs
				}
			}
		case map[string]interface{}:
			log.Debug("Document parameter type is map[string]interface{}")
			for k, v := range params {
				parameters[k] = v
			}
		default:
			return pluginsInfo, errors.New("parameter type specified to run document is unknown")

		}
		log.Info("Parameters passed in are ", parameters)
	}
	var rawDocument []byte
	if rawDocument, err = readFileContents(log, p.filesys, pathToFile); err != nil {
		log.Error("Could not read document from remote resource - ", err)
		return nil, err
	}
	log.Infof("Sending the document received for parsing - %v", string(rawDocument))

	return p.execDoc.ParseDocument(log, rawDocument, config.OrchestrationDirectory, config.OutputS3BucketName, config.OutputS3KeyPrefix, config.MessageId, config.PluginID, config.DefaultWorkingDirectory, parameters)
}

// Name returns the plugin name
func Name() string {
	return appconfig.PluginRunDocument
}

// parseAndValidateInput parses the input json file and also validates its inputs
func parseAndValidateInput(rawPluginInput interface{}) (*RunDocumentPluginInput, error) {
	var input RunDocumentPluginInput
	var err error
	if err = jsonutil.Remarshal(rawPluginInput, &input); err != nil {
		return nil, fmt.Errorf("invalid format in plugin properties %v; \nerror %v", rawPluginInput, err)
	}

	if valid, err := validateInput(&input); !valid {
		return nil, fmt.Errorf("invalid input: %v", err)
	}

	return &input, nil
}

// validateInput ensures the plugin input matches the defined schema
func validateInput(input *RunDocumentPluginInput) (valid bool, err error) {
	// ensure non-empty location type
	if input.DocumentType == "" {
		return false, errors.New("Document Type must be specified to either by SSMDocument or LocalPath.")
	}
	if input.DocumentType != SSMDocumentType && input.DocumentType != LocalPathType {
		return false, errors.New("Document type specified in invalid")
	}
	if input.DocumentPath == "" {
		return false, errors.New("Document Path must be provided")
	}
	return true, nil
}

// readFileContents is a method to read the contents of a give file path
func readFileContents(log log.T, filesysdep filemanager.FileSystem, destinationPath string) (fileContent []byte, err error) {

	log.Debug("Reading file contents from file - ", destinationPath)

	var rawFile string
	if rawFile, err = filesysdep.ReadFile(destinationPath); err != nil {
		log.Error("Error occurred while reading file - ", err)
		return nil, err
	}
	if rawFile == "" {
		return []byte(rawFile), errors.New("File is empty!")
	}

	return []byte(rawFile), nil
}

var instance instanceInfo = &instanceInfoImp{}

// system represents the dependency for platform
type instanceInfo interface {
	InstanceID() (string, error)
}

type instanceInfoImp struct{}

// InstanceID wraps platform InstanceID
func (instanceInfoImp) InstanceID() (string, error) { return platform.InstanceID() }
